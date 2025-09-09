package app

import (
	"context"
	"fmt"
	"os"

	"github.com/Kopleman/gomemdb/internal/cli"
	"github.com/Kopleman/gomemdb/internal/compute/parser"
	"github.com/Kopleman/gomemdb/internal/storage/engine"
	"go.uber.org/zap"
)

type App struct {
	cli    *cli.Cli
	logger *zap.Logger
}

func New(logger *zap.Logger) App {
	p := parser.New(logger)
	s := engine.New(logger)

	c := cli.New(cli.RunArgs{
		Reader:  os.Stdin,
		Parser:  p,
		Storage: s,
		Logger:  logger,
	})
	return App{
		cli:    c,
		logger: logger,
	}
}

func (a *App) Start(ctx context.Context) error {
	if err := a.cli.Start(ctx); err != nil {
		return fmt.Errorf("failed to start CLI: %w", err)
	}
	return nil
}
