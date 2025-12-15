package wal

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Kopleman/gomemdb/internal/command"
	"go.uber.org/zap"
)

// Recover reads all WAL segments and returns commands in order.
func Recover(dataDirectory string, logger *zap.Logger) ([]command.Command, error) {
	// Check if directory exists
	if _, err := os.Stat(dataDirectory); os.IsNotExist(err) {
		logger.Info("WAL directory does not exist, skipping recovery")
		return nil, nil
	}

	// List all segment files
	entries, err := os.ReadDir(dataDirectory)
	if err != nil {
		return nil, fmt.Errorf("failed to read WAL directory: %w", err)
	}

	// Filter and sort segment files
	segments := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), walFilePrefix) && strings.HasSuffix(entry.Name(), walFileSuffix) {
			segments = append(segments, entry.Name())
		}
	}

	if len(segments) == 0 {
		logger.Info("no WAL segments found, skipping recovery")
		return nil, nil
	}

	// Sort segments by sequence number
	sort.Slice(segments, func(i, j int) bool {
		seqI := extractSequenceNumber(segments[i])
		seqJ := extractSequenceNumber(segments[j])
		return seqI < seqJ
	})

	logger.Info("recovering from WAL segments", zap.Int("count", len(segments)))

	// Read all segments
	var allCommands []command.Command
	for _, segName := range segments {
		segPath := filepath.Join(dataDirectory, segName)
		commands, err := readSegment(segPath, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to read segment %s: %w", segName, err)
		}
		allCommands = append(allCommands, commands...)
	}

	logger.Info("recovered commands from WAL", zap.Int("count", len(allCommands)))
	return allCommands, nil
}

// extractSequenceNumber extracts sequence number from segment filename.
// Format: wal_0000000000000001.log -> 1.
func extractSequenceNumber(filename string) uint64 {
	// Remove prefix "wal_" and suffix ".log".
	seqStr := strings.TrimPrefix(filename, walFilePrefix)
	seqStr = strings.TrimSuffix(seqStr, walFileSuffix)
	seq, err := strconv.ParseUint(seqStr, 10, 64)
	if err != nil {
		return 0
	}
	return seq
}

// readSegment reads all commands from a segment file.
func readSegment(path string, logger *zap.Logger) ([]command.Command, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open segment file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logger.Error("failed to close segment file", zap.Error(closeErr), zap.String("path", path))
		}
	}()

	var commands []command.Command

	for {
		cmd, err := readCommand(file)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read command: %w", err)
		}
		commands = append(commands, cmd)
	}

	logger.Debug("read segment", zap.String("path", path), zap.Int("commands", len(commands)))
	return commands, nil
}

// readCommand reads a single command from the file.
func readCommand(r io.Reader) (command.Command, error) {
	var cmdType byte
	if err := binary.Read(r, binary.LittleEndian, &cmdType); err != nil {
		return command.Command{}, fmt.Errorf("failed to read command type: %w", err)
	}

	// Read name length
	var nameLen uint32
	if err := binary.Read(r, binary.LittleEndian, &nameLen); err != nil {
		return command.Command{}, fmt.Errorf("failed to read name length: %w", err)
	}

	// Read name
	nameBytes := make([]byte, nameLen)
	if _, err := io.ReadFull(r, nameBytes); err != nil {
		return command.Command{}, fmt.Errorf("failed to read name: %w", err)
	}

	// Read value length
	var valueLen uint32
	if err := binary.Read(r, binary.LittleEndian, &valueLen); err != nil {
		return command.Command{}, fmt.Errorf("failed to read value length: %w", err)
	}

	// Read value
	var valueBytes []byte
	if valueLen > 0 {
		valueBytes = make([]byte, valueLen)
		if _, err := io.ReadFull(r, valueBytes); err != nil {
			return command.Command{}, fmt.Errorf("failed to read value: %w", err)
		}
	}

	// Construct command
	cmd := command.Command{
		Name: string(nameBytes),
	}

	switch cmdType {
	case cmdTypeSET:
		cmd.Type = command.CommandSET
		cmd.Set = command.SetArgs{
			Value: string(valueBytes),
		}
	case cmdTypeDEL:
		cmd.Type = command.CommandDEL
	default:
		return command.Command{}, fmt.Errorf("unknown command type: %d", cmdType)
	}

	return cmd, nil
}
