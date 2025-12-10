package parser

import (
	"context"
	"errors"
	"strings"

	"github.com/Kopleman/gomemdb/internal/command"
	"go.uber.org/zap"
)

var (
	ErrUnknownCommand = errors.New("unknown command")
	ErrArgs           = errors.New("invalid number of args")
)

const (
	getArgsCnt = 1
	delArgsCnt = 1
	setArgsCnt = 2
)

type Parser struct {
	logger *zap.Logger
}

func New(logger *zap.Logger) *Parser {
	return &Parser{
		logger: logger,
	}
}

func (p Parser) Parse(ctx context.Context, line string) (command.Command, error) {
	const minWordsCnt = 2
	const maxWordsCnt = 3

	p.logger.Debug("parse", zap.String("line", line))

	words := strings.Fields(line)

	if len(words) > maxWordsCnt || len(words) < minWordsCnt {
		return command.Command{}, ErrUnknownCommand
	}

	cmd := words[0]
	args := words[1:]

	switch cmd {
	case string(command.CommandGET):
		return parseGET(args)
	case string(command.CommandDEL):
		return parseDEL(args)
	case string(command.CommandSET):
		return parseSET(args)
	}
	return command.Command{}, ErrUnknownCommand
}

func parseGET(args []string) (command.Command, error) {
	if len(args) != getArgsCnt {
		return command.Command{}, ErrArgs
	}
	return command.Command{
		Type: command.CommandGET,
		Name: args[0],
	}, nil
}

func parseSET(args []string) (command.Command, error) {
	if len(args) != setArgsCnt {
		return command.Command{}, ErrArgs
	}
	return command.Command{
		Type: command.CommandSET,
		Name: args[0],

		Set: command.SetArgs{
			Value: args[1],
		},
	}, nil
}

func parseDEL(args []string) (command.Command, error) {
	if len(args) != delArgsCnt {
		return command.Command{}, ErrArgs
	}
	return command.Command{
		Type: command.CommandDEL,
		Name: args[0],
	}, nil
}
