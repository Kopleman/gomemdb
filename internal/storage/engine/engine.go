package engine

import (
	"context"
	"errors"

	"github.com/Kopleman/gomemdb/internal/command"
	"go.uber.org/zap"
)

var (
	ErrUnknownCmd = errors.New("unknown command")
	ErrInvalidCmd = errors.New("invalid command")
)

type Output struct {
	Error error
	Msg   string
}

type Engine struct {
	s      *storage
	logger *zap.Logger
}

func New(logger *zap.Logger) *Engine {
	return &Engine{
		s:      newStorage(),
		logger: logger,
	}
}

func (e *Engine) Do(ctx context.Context, cmd command.Command) (string, error) {
	e.logger.Debug("do command", zap.String("cmd", string(cmd.Type)))

	name := cmd.Name
	if len(name) == 0 {
		return "", ErrInvalidCmd
	}
	switch cmd.Type {
	case command.CommandGET:
		return e.s.Get(ctx, name)

	case command.CommandSET:
		value := cmd.Set.Value
		if len(value) == 0 {
			return "", ErrInvalidCmd
		}

		return "", e.s.Set(ctx, name, value)

	case command.CommandDEL:
		return "", e.s.Del(ctx, name)

	case command.CommandUnknown:
		return "", ErrUnknownCmd
	default:
		return "", ErrUnknownCmd
	}
}
