package wal

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Kopleman/gomemdb/internal/command"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestWAL_Write_Basic(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logger := zap.NewNop()

	config := Config{
		Enabled:              true,
		FlushingBatchSize:    5,
		FlushingBatchTimeout: 100 * time.Millisecond,
		MaxSegmentSize:       1024 * 1024, // 1MB
		DataDirectory:        tmpDir,
	}

	wal, err := New(config, logger)
	require.NoError(t, err)
	require.NotNil(t, wal)
	defer func() {
		if closeErr := wal.Close(); closeErr != nil {
			t.Errorf("failed to close WAL: %v", closeErr)
		}
	}()

	ctx := context.Background()

	// Write SET command
	cmd := command.Command{
		Type: command.CommandSET,
		Name: "key1",
		Set: command.SetArgs{
			Value: "value1",
		},
	}

	err = wal.Write(ctx, cmd)
	assert.NoError(t, err)

	// Write DEL command
	cmd2 := command.Command{
		Type: command.CommandDEL,
		Name: "key1",
	}

	err = wal.Write(ctx, cmd2)
	assert.NoError(t, err)
}

func TestWAL_Write_BatchBySize(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logger := zap.NewNop()

	batchSize := 3
	config := Config{
		Enabled:              true,
		FlushingBatchSize:    batchSize,
		FlushingBatchTimeout: 1 * time.Second, // Long timeout to test batch size
		MaxSegmentSize:       1024 * 1024,
		DataDirectory:        tmpDir,
	}

	wal, err := New(config, logger)
	require.NoError(t, err)
	require.NotNil(t, wal)
	defer func() {
		if closeErr := wal.Close(); closeErr != nil {
			t.Errorf("failed to close WAL: %v", closeErr)
		}
	}()

	ctx := context.Background()

	// Write exactly batchSize commands
	for i := range batchSize {
		cmd := command.Command{
			Type: command.CommandSET,
			Name: "key" + string(rune('0'+i)),
			Set: command.SetArgs{
				Value: "value" + string(rune('0'+i)),
			},
		}
		err = wal.Write(ctx, cmd)
		assert.NoError(t, err)
	}

	// Give some time for flush
	time.Sleep(50 * time.Millisecond)

	// Verify segment file exists and has data
	entries, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	assert.Greater(t, len(entries), 0)
}

func TestWAL_Write_BatchByTimeout(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logger := zap.NewNop()

	timeout := 50 * time.Millisecond
	config := Config{
		Enabled:              true,
		FlushingBatchSize:    100, // Large batch size to test timeout
		FlushingBatchTimeout: timeout,
		MaxSegmentSize:       1024 * 1024,
		DataDirectory:        tmpDir,
	}

	wal, err := New(config, logger)
	require.NoError(t, err)
	require.NotNil(t, wal)
	defer func() {
		if closeErr := wal.Close(); closeErr != nil {
			t.Errorf("failed to close WAL: %v", closeErr)
		}
	}()

	ctx := context.Background()

	// Write one command
	cmd := command.Command{
		Type: command.CommandSET,
		Name: "key1",
		Set: command.SetArgs{
			Value: "value1",
		},
	}

	start := time.Now()
	err = wal.Write(ctx, cmd)
	duration := time.Since(start)
	assert.NoError(t, err)

	// Should have waited for timeout (with some tolerance)
	assert.GreaterOrEqual(t, duration, timeout-time.Millisecond*10)
	assert.Less(t, duration, timeout*2)

	// Verify segment file exists
	entries, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	assert.Greater(t, len(entries), 0)
}

func TestWAL_SegmentRotation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logger := zap.NewNop()

	// Small segment size to trigger rotation quickly
	maxSegmentSize := int64(100) // 100 bytes
	config := Config{
		Enabled:              true,
		FlushingBatchSize:    1,
		FlushingBatchTimeout: 10 * time.Millisecond,
		MaxSegmentSize:       maxSegmentSize,
		DataDirectory:        tmpDir,
	}

	wal, err := New(config, logger)
	require.NoError(t, err)
	require.NotNil(t, wal)
	defer func() {
		if closeErr := wal.Close(); closeErr != nil {
			t.Errorf("failed to close WAL: %v", closeErr)
		}
	}()

	ctx := context.Background()

	// Write commands until we trigger segment rotation
	// Each command is roughly: 1 (type) + 4 (name_len) + len(name) + 4 (value_len) + len(value)
	// "keyX" = 4 bytes, "valueX" = 6 bytes, so roughly 1+4+4+4+6 = 19 bytes per command
	// We need at least 100/19 ≈ 6 commands to trigger rotation
	const commandsToWrite = 10
	for i := range commandsToWrite {
		cmd := command.Command{
			Type: command.CommandSET,
			Name: "key" + string(rune('0'+i)),
			Set: command.SetArgs{
				Value: "value" + string(rune('0'+i)),
			},
		}
		err = wal.Write(ctx, cmd)
		assert.NoError(t, err)
	}

	// Give time for flushes
	time.Sleep(100 * time.Millisecond)

	// Verify multiple segment files exist
	entries, err := os.ReadDir(tmpDir)
	require.NoError(t, err)

	segmentCount := 0
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".log" {
			segmentCount++
		}
	}
	assert.Greater(t, segmentCount, 1, "should have multiple segments")
}

func TestWAL_Recover(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logger := zap.NewNop()

	config := Config{
		Enabled:              true,
		FlushingBatchSize:    1,
		FlushingBatchTimeout: 10 * time.Millisecond,
		MaxSegmentSize:       1024 * 1024,
		DataDirectory:        tmpDir,
	}

	// Create WAL and write some commands
	wal, err := New(config, logger)
	require.NoError(t, err)
	require.NotNil(t, wal)

	ctx := context.Background()

	commands := []command.Command{
		{
			Type: command.CommandSET,
			Name: "key1",
			Set: command.SetArgs{
				Value: "value1",
			},
		},
		{
			Type: command.CommandSET,
			Name: "key2",
			Set: command.SetArgs{
				Value: "value2",
			},
		},
		{
			Type: command.CommandDEL,
			Name: "key1",
		},
	}

	for _, cmd := range commands {
		err = wal.Write(ctx, cmd)
		assert.NoError(t, err)
	}

	// Close WAL
	err = wal.Close()
	require.NoError(t, err)

	// Recover commands
	recovered, err := Recover(tmpDir, logger)
	require.NoError(t, err)
	require.Equal(t, len(commands), len(recovered))

	// Verify commands match
	for i, cmd := range commands {
		assert.Equal(t, cmd.Type, recovered[i].Type)
		assert.Equal(t, cmd.Name, recovered[i].Name)
		if cmd.Type == command.CommandSET {
			assert.Equal(t, cmd.Set.Value, recovered[i].Set.Value)
		}
	}
}

func TestWAL_Recover_MultipleSegments(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logger := zap.NewNop()

	// Small segment size to create multiple segments
	maxSegmentSize := int64(100)
	config := Config{
		Enabled:              true,
		FlushingBatchSize:    1,
		FlushingBatchTimeout: 10 * time.Millisecond,
		MaxSegmentSize:       maxSegmentSize,
		DataDirectory:        tmpDir,
	}

	// Create WAL and write commands
	wal, err := New(config, logger)
	require.NoError(t, err)
	require.NotNil(t, wal)

	ctx := context.Background()

	// Write enough commands to create multiple segments
	const commandsToWrite = 10
	expectedCommands := make([]command.Command, 0)
	for i := range commandsToWrite {
		cmd := command.Command{
			Type: command.CommandSET,
			Name: "key" + string(rune('0'+i)),
			Set: command.SetArgs{
				Value: "value" + string(rune('0'+i)),
			},
		}
		expectedCommands = append(expectedCommands, cmd)
		err = wal.Write(ctx, cmd)
		assert.NoError(t, err)
	}

	// Close WAL
	err = wal.Close()
	require.NoError(t, err)

	// Recover commands
	recovered, err := Recover(tmpDir, logger)
	require.NoError(t, err)
	assert.Equal(t, len(expectedCommands), len(recovered))

	// Verify all commands are recovered
	for i, cmd := range expectedCommands {
		assert.Equal(t, cmd.Type, recovered[i].Type)
		assert.Equal(t, cmd.Name, recovered[i].Name)
		if cmd.Type == command.CommandSET {
			assert.Equal(t, cmd.Set.Value, recovered[i].Set.Value)
		}
	}
}

func TestWAL_Disabled(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()

	config := Config{
		Enabled: false,
	}

	wal, err := New(config, logger)
	assert.ErrorIs(t, err, ErrWALDisabled)
	assert.Nil(t, wal) // Should return nil when disabled
}

func TestWAL_Close(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	logger := zap.NewNop()

	config := Config{
		Enabled:              true,
		FlushingBatchSize:    10,
		FlushingBatchTimeout: 100 * time.Millisecond,
		MaxSegmentSize:       1024 * 1024,
		DataDirectory:        tmpDir,
	}

	wal, err := New(config, logger)
	require.NoError(t, err)
	require.NotNil(t, wal)

	ctx := context.Background()

	// Write a command
	cmd := command.Command{
		Type: command.CommandSET,
		Name: "key1",
		Set: command.SetArgs{
			Value: "value1",
		},
	}

	err = wal.Write(ctx, cmd)
	assert.NoError(t, err)

	// Close WAL
	err = wal.Close()
	assert.NoError(t, err)

	// Writing after close should fail
	err = wal.Write(ctx, cmd)
	assert.Error(t, err)
	assert.Equal(t, ErrWALClosed, err)
}
