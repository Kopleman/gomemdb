package parser

import (
	"context"
	"testing"

	"github.com/Kopleman/gomemdb/internal/command"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestParser(t *testing.T) {
	t.Parallel()

	type test struct {
		err   error
		cmd   command.Command
		input string
	}

	tests := map[string]test{
		"invalid string": {
			input: "just invalid string without meaning",
			cmd:   command.Command{},
			err:   ErrUnknownCommand,
		},
		"no command just words": {
			input: "string without meaning",
			cmd:   command.Command{},
			err:   ErrUnknownCommand,
		},

		"GET with many args": {
			input: "GET some value",
			cmd:   command.Command{},
			err:   ErrArgs,
		},

		"GET with simple arg": {
			input: "GET name",
			cmd: command.Command{
				Type: command.CommandGET,
				Name: "name",
			},
			err: nil,
		},
		"GET with specials in arg": {
			input: "GET some**magic_value-/",
			cmd: command.Command{
				Type: command.CommandGET,
				Name: "some**magic_value-/",
			},
			err: nil,
		},

		"SET without value": {
			input: "SET name",
			cmd:   command.Command{},
			err:   ErrArgs,
		},
		"SET command with specials in args": {
			input: "SET magic*name some**magic_value-/",
			cmd: command.Command{
				Type: command.CommandSET,
				Name: "magic*name",
				Set: command.SetArgs{
					Value: "some**magic_value-/",
				},
			},
			err: nil,
		},

		"DEL with many args": {
			input: "DEL name value",
			cmd:   command.Command{},
			err:   ErrArgs,
		},
		"DEL simple": {
			input: "DEL name",
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
			logger := zap.NewNop()
			p := New(logger)

			ctx := context.Background()

			cmd, err := p.Parse(ctx, tc.input)
			if tc.err != nil {
				assert.ErrorIs(t, err, tc.err)
				return
			}
			assert.Equal(t, tc.cmd, cmd)
		})
	}
}
