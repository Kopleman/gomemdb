package app

import (
	"context"
	"fmt"
	"os"

	"github.com/Kopleman/gomemdb/internal/compute/parser"
	"github.com/Kopleman/gomemdb/internal/database"
	"github.com/Kopleman/gomemdb/internal/network"
	"github.com/Kopleman/gomemdb/internal/storage/engine"
	"github.com/Kopleman/gomemdb/pkg/config"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type App struct {
	db     *database.DataBase
	server *network.GRPCServer
	logger *zap.Logger
	config *config.Config
}

func New(cfg *config.Config, logger *zap.Logger) (*App, error) {
	p := parser.New(logger)
	s := engine.New(logger)

	c := database.New(database.RunArgs{
		Reader:  os.Stdin,
		Parser:  p,
		Storage: s,
		Logger:  logger,
	})
	server, serverErr := CreateNetwork(cfg, logger, c)
	if serverErr != nil {
		return nil, fmt.Errorf("failed to create network: %w", serverErr)
	}
	return &App{
		db:     c,
		logger: logger,
		server: server,
		config: cfg,
	}, nil
}

func (a *App) Start(ctx context.Context) error {
	group, groupCtx := errgroup.WithContext(ctx)

	group.Go(func() error {
		// gRPC server handles queries internally via ExecuteCommand method
		a.server.HandleQueries(groupCtx)
		return nil
	})

	a.logger.Info("starting server")

	if err := group.Wait(); err != nil {
		return fmt.Errorf("server group wait: %w", err)
	}

	return nil
}

func CreateNetwork(cfg *config.Config, logger *zap.Logger, db *database.DataBase) (*network.GRPCServer, error) {
	address := cfg.Address
	var options []network.GRPCServerOption

	server, err := network.NewGRPCServer(address, db, logger, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create grpc server: %w", err)
	}

	return server, nil
}
