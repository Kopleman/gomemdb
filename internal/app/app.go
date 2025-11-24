package app

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/Kopleman/gomemdb/internal/compute/parser"
	"github.com/Kopleman/gomemdb/internal/database"
	"github.com/Kopleman/gomemdb/internal/network"
	"github.com/Kopleman/gomemdb/internal/storage/engine"
	"github.com/Kopleman/gomemdb/internal/utils"
	"github.com/Kopleman/gomemdb/pkg/config"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type App struct {
	db     *database.DataBase
	server *network.TCPServer
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
	server, serverErr := CreateNetwork(cfg, logger)
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
		a.server.HandleQueries(groupCtx, func(ctx context.Context, query []byte) []byte {
			response, handleErr := a.db.HandleQuery(ctx, string(query))
			if handleErr != nil {
				response = fmt.Sprintf("[error] %s", handleErr.Error())
			}
			return []byte(fmt.Sprintf("[ok] %s", response))
		})

		return nil
	})

	a.logger.Info("starting server")

	if err := group.Wait(); err != nil {
		return fmt.Errorf("server group wait: %w", err)
	}

	return nil
}

func CreateNetwork(cfg *config.Config, logger *zap.Logger) (*network.TCPServer, error) {
	address := cfg.Address
	var options []network.TCPServerOption

	if cfg.MaxConnections != 0 {
		options = append(options, network.WithServerMaxConnectionsNumber(uint(cfg.MaxConnections)))
	}

	if cfg.MaxMessageSize != "" {
		size, err := utils.ParseSize(cfg.MaxMessageSize)
		if err != nil {
			return nil, errors.New("incorrect max message size")
		}

		options = append(options, network.WithServerBufferSize(uint(size)))
	}

	if cfg.IdleTimeout != 0 {
		options = append(options, network.WithServerIdleTimeout(cfg.IdleTimeout))
	}

	server, err := network.NewTCPServer(address, logger, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create tcp server: %w", err)
	}

	return server, nil
}
