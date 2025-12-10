package config

import (
	"flag"
	"fmt"
	"time"

	"github.com/Kopleman/gomemdb/internal/utils"
	"github.com/Kopleman/gomemdb/pkg/config/types/flags"
)

type CliConfig struct {
	EndPoint       *flags.NetAddress
	IdleTimeout    time.Duration
	MaxMessageSize int
}

func GetCliConfig() (*CliConfig, error) {
	config := new(CliConfig)
	endpoint := new(flags.NetAddress)
	endpoint.Host = ""
	endpoint.Port = ""
	config.EndPoint = endpoint

	endpointValue := flag.Value(endpoint)
	flag.Var(endpointValue, "a", "address and port of server")
	flag.DurationVar(&config.IdleTimeout, "t", time.Minute, "Idle timeout for connection")

	maxMessageSizeStr := flag.String("s", "4KB", "Max message size for connection")
	flag.Parse()

	maxMessageSize, err := utils.ParseSize(*maxMessageSizeStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse max message size: %w", err)
	}

	config.MaxMessageSize = maxMessageSize

	return config, nil
}
