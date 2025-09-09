package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Kopleman/gomemdb/internal/app"
	"github.com/Kopleman/gomemdb/pkg/config"
	"go.uber.org/zap"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Failed to load config ", err)
	}

	var logger *zap.Logger
	if cfg.RunEnv == "production" {
		logger, err = zap.NewProduction()
		if err != nil {
			log.Fatal("Failed to initialize logger: ", err)
		}
	} else {
		logger, err = zap.NewDevelopment()
		if err != nil {
			log.Fatal("Failed to initialize logger:", err)
		}
	}
	defer logger.Sync() //nolint:all // its safe
	a := app.New(logger)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	startErr := a.Start(ctx)
	if startErr != nil {
		logger.Fatal("start error", zap.Error(startErr))
	}
}
