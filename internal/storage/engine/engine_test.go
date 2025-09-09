package engine

import (
	"context"
	"testing"
	"time"

	"github.com/Kopleman/gomemdb/internal/command"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestDo_checkErr(t *testing.T) {
	t.Parallel()

	type test struct {
		err error
		cmd command.Command
	}

	tests := map[string]test{
		"name is not set": {
			cmd: command.Command{
				Type: command.CommandGET,
			},
			err: ErrInvalidCmd,
		},

		"unknown command with arg": {
			cmd: command.Command{
				Type: command.CommandUnknown,
				Name: "name",
			},
			err: ErrUnknownCmd,
		},
		"get command from empty storage": {
			cmd: command.Command{
				Type: command.CommandGET,
				Name: "name",
			},
			err: ErrNotFound,
		},
		"set command": {
			cmd: command.Command{
				Type: command.CommandSET,

				Name: "name",
				Set: command.SetArgs{
					Value: "value",
				},
			},
			err: nil,
		},
		"set command with one arg": {
			cmd: command.Command{
				Type: command.CommandSET,
				Name: "name",
			},
			err: ErrInvalidCmd,
		},
		"del command for empty storage": {
			cmd: command.Command{
				Type: command.CommandDEL,
				Name: "name",
			},
			err: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			logger := zap.NewNop()
			s := New(logger)
			_, err := s.Do(ctx, tc.cmd)
			assert.ErrorIs(t, tc.err, err)
		})
	}
}

func TestDo_getAfterSet(t *testing.T) {
	t.Parallel()

	type test struct {
		err      error
		cmd      command.Command
		setName  string
		setValue string
	}

	tests := map[string]test{
		"get after set": {
			cmd: command.Command{
				Type: command.CommandGET,
				Name: "name",
			},

			setName:  "name",
			setValue: "value",

			err: nil,
		},

		"get what not set": {
			cmd: command.Command{
				Type: command.CommandGET,
				Name: "different_name",
			},

			setName:  "name",
			setValue: "value",

			err: ErrNotFound,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			logger := zap.NewNop()
			s := New(logger)

			// set перед выполнением get
			setCmd := command.Command{
				Type: command.CommandSET,
				Name: tc.setName,
				Set: command.SetArgs{
					Value: tc.setValue,
				},
			}
			_, err := s.Do(ctx, setCmd)
			require.NoError(t, err)

			val, err := s.Do(ctx, tc.cmd)
			if tc.err != nil {
				assert.ErrorIs(t, err, tc.err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.setValue, val)
			}
		})
	}
}

func TestDo_getAfterDel(t *testing.T) {
	t.Parallel()

	name := "some_name"
	value := "secret_value"

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	logger := zap.NewNop()
	s := New(logger)

	// set перед выполнением get
	cmd := command.Command{
		Type: command.CommandSET,
		Name: name,
		Set: command.SetArgs{
			Value: value,
		},
	}
	val, err := s.Do(ctx, cmd)
	require.NoError(t, err)
	assert.Empty(t, val)

	cmd = command.Command{
		Type: command.CommandGET,
		Name: name,
	}

	val, err = s.Do(ctx, cmd)
	assert.NoError(t, err)
	assert.Equal(t, value, val)

	cmd = command.Command{
		Type: command.CommandDEL,
		Name: name,
	}

	val, err = s.Do(ctx, cmd)
	assert.NoError(t, err)
	assert.Empty(t, val)

	cmd = command.Command{
		Type: command.CommandGET,
		Name: name,
	}

	_, err = s.Do(ctx, cmd)
	assert.ErrorIs(t, ErrNotFound, err)
}
