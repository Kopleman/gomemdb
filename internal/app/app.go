package app

import (
	"context"
	"fmt"
	"os"

	"github.com/Kopleman/gomemdb/internal/compute/parser"
	"github.com/Kopleman/gomemdb/internal/database"
	"github.com/Kopleman/gomemdb/internal/network"
	"github.com/Kopleman/gomemdb/internal/storage/engine"
	"github.com/Kopleman/gomemdb/internal/storage/wal"
	"github.com/Kopleman/gomemdb/pkg/config"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type App struct {
	db     *database.DataBase
	wal    *wal.WAL
	server *network.GRPCServer
	logger *zap.Logger
	config *config.Config
}

func New(cfg *config.Config, logger *zap.Logger) (*App, error) {
	p := parser.New(logger)
	s := engine.New(logger)

	// Create WAL if enabled
	var walInstance *wal.WAL
	if cfg.WAL.Enabled {
		walConfig := wal.Config{
			Enabled:              cfg.WAL.Enabled,
			FlushingBatchSize:    cfg.WAL.FlushingBatchSize,
			FlushingBatchTimeout: cfg.WAL.FlushingBatchTimeout,
			MaxSegmentSize:       cfg.WAL.MaxSegmentSize,
			DataDirectory:        cfg.WAL.DataDirectory,
		}
		var err error
		walInstance, err = wal.New(walConfig, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create WAL: %w", err)
		}

		// Recover data from WAL
		if err := recoverFromWAL(cfg.WAL.DataDirectory, s, logger); err != nil {
			return nil, fmt.Errorf("failed to recover from WAL: %w", err)
		}
	}

	c := database.New(database.RunArgs{
		Reader:  os.Stdin,
		Parser:  p,
		Storage: s,
		WAL:     walInstance,
		Logger:  logger,
	})
	server, serverErr := CreateNetwork(cfg, logger, c)
	if serverErr != nil {
		// Close WAL if network creation fails
		if walInstance != nil {
			_ = walInstance.Close()
		}
		return nil, fmt.Errorf("failed to create network: %w", serverErr)
	}
	return &App{
		db:     c,
		wal:    walInstance,
		logger: logger,
		server: server,
		config: cfg,
	}, nil
}

// recoverFromWAL recovers database state from WAL segments.
func recoverFromWAL(dataDirectory string, storage *engine.Engine, logger *zap.Logger) error {
	commands, err := wal.Recover(dataDirectory, logger)
	if err != nil {
		return fmt.Errorf("failed to recover WAL: %w", err)
	}

	if len(commands) == 0 {
		logger.Info("no commands to recover from WAL")
		return nil
	}

	logger.Info("recovering database state from WAL", zap.Int("commands", len(commands)))

	ctx := context.Background()
	// Apply all recovered commands to storage
	for _, cmd := range commands {
		if _, err := storage.Do(ctx, cmd); err != nil {
			logger.Error("failed to apply recovered command", zap.Error(err), zap.String("cmd", string(cmd.Type)))
			return fmt.Errorf("failed to apply recovered command: %w", err)
		}
	}

	logger.Info("successfully recovered database state from WAL", zap.Int("commands", len(commands)))
	return nil
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

	// Close WAL on shutdown
	if a.wal != nil {
		if err := a.wal.Close(); err != nil {
			a.logger.Error("failed to close WAL", zap.Error(err))
		}
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
