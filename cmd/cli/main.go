package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"syscall"

	"github.com/Kopleman/gomemdb/internal/network"
	"github.com/Kopleman/gomemdb/pkg/config"
	"github.com/davecgh/go-spew/spew"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()

	cfg, cfgErr := config.GetCliConfig()
	if cfgErr != nil {
		logger.Fatal("failed to parse cli cfg", zap.Error(cfgErr))
	}
	spew.Dump(cfg)

	var options []network.GRPCClientOption
	// gRPC client options can be added here if needed in the future

	reader := bufio.NewReader(os.Stdin)
	client, err := network.NewGRPCClient(cfg.EndPoint.String(), options...)
	if err != nil {
		logger.Fatal("failed to connect with server", zap.Error(err))
	}
	defer client.Close()

	for {
		fmt.Print("[gomemdb] > ")
		request, err := reader.ReadString('\n')
		if errors.Is(err, syscall.EPIPE) {
			logger.Fatal("connection was closed", zap.Error(err))
		} else if err != nil {
			logger.Error("failed to read query", zap.Error(err))
		}

		response, err := client.Send([]byte(request))
		if errors.Is(err, syscall.EPIPE) {
			logger.Fatal("connection was closed", zap.Error(err))
		} else if err != nil {
			logger.Error("failed to send query", zap.Error(err))
		}

		fmt.Println(string(response))
	}
}
