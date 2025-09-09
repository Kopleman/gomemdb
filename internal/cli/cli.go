package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"

	"github.com/Kopleman/gomemdb/internal/command"
	"go.uber.org/zap"
)

const prompt = "-> "

type Parser interface {
	Parse(ctx context.Context, line string) (command.Command, error)
}
type Storage interface {
	Do(ctx context.Context, cmd command.Command) (string, error)
}

type Cli struct {
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

func New(args RunArgs) *Cli {
	s := bufio.NewScanner(args.Reader)
	s.Split(bufio.ScanLines)

	return &Cli{
		s:       s,
		p:       args.Parser,
		storage: args.Storage,
		logger:  args.Logger,
	}
}

func (c *Cli) run(ctx context.Context, errChan chan<- error) {
	for {
		c.logger.Info(prompt)

		if !c.s.Scan() {
			break
		}

		line := c.s.Text()
		c.logger.Debug("read line", zap.String("line", line))

		cmd, err := c.p.Parse(ctx, line)
		if err != nil {
			c.logger.Error("parse error", zap.Error(err))
			continue
		}

		out, doErr := c.storage.Do(ctx, cmd)
		if doErr != nil {
			c.logger.Error("storage exec error", zap.Error(doErr))
			continue
		}

		c.logger.Info(out)
	}

	errChan <- c.s.Err()
}

func (c *Cli) Start(ctx context.Context) error {
	errCh := make(chan error, 1)
	go c.run(ctx, errCh)

	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled: %w", ctx.Err())
	case err := <-errCh:
		return fmt.Errorf("cli error: %w", err)
	}
}
