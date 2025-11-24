package database

import (
	"bufio"
	"context"
	"fmt"
	"io"

	"github.com/Kopleman/gomemdb/internal/command"
	"go.uber.org/zap"
)

type Parser interface {
	Parse(ctx context.Context, line string) (command.Command, error)
}
type Storage interface {
	Do(ctx context.Context, cmd command.Command) (string, error)
}

type DataBase struct {
	s *bufio.Scanner

	p       Parser
	storage Storage
	logger  *zap.Logger
}

type RunArgs struct {
	Reader  io.Reader
	Parser  Parser
	Storage Storage
	Logger  *zap.Logger
}

func New(args RunArgs) *DataBase {
	s := bufio.NewScanner(args.Reader)
	s.Split(bufio.ScanLines)

	return &DataBase{
		s:       s,
		p:       args.Parser,
		storage: args.Storage,
		logger:  args.Logger,
	}
}

func (c *DataBase) HandleQuery(ctx context.Context, queryStr string) (string, error) {
	cmd, err := c.p.Parse(ctx, queryStr)
	if err != nil {
		c.logger.Error("parse error", zap.Error(err))
		return "", fmt.Errorf("failed to parse query: %s", queryStr)
	}

	out, doErr := c.storage.Do(ctx, cmd)
	if doErr != nil {
		c.logger.Error("storage exec error", zap.Error(doErr))
		return "", fmt.Errorf("failed to execute command: %s", queryStr)
	}

	return out, nil
}
