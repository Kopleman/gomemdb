package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Kopleman/gomemdb/internal/utils"
	"github.com/joho/godotenv"
)

const (
	defaultMaxConnections    = 100
	defaultMaxMessageSize    = "4KB"
	defaultIdleTimeout       = time.Minute
	defaultWALBatchSize      = 100
	defaultWALBatchTimeout   = 10
	defaultWALMaxSegmentSize = 10 * 1024 * 1024 // 10MB
)

// WALConfig represents Write-Ahead Log configuration.
type WALConfig struct {
	DataDirectory        string
	FlushingBatchSize    int
	FlushingBatchTimeout time.Duration
	MaxSegmentSize       int64
	Enabled              bool
}

// Config представляет конфигурацию приложения.
type Config struct {
	RunEnv         string
	Address        string
	MaxMessageSize string
	WAL            WALConfig
	MaxConnections int
	IdleTimeout    time.Duration
}

// LoadConfig загружает конфигурацию из переменных окружения и .env файла.
func LoadConfig() (*Config, error) {
	// Загружаем .env файл, если он существует
	_ = godotenv.Load()

	walEnabled := getEnv("WAL_ENABLED", "") != ""
	walConfig := WALConfig{
		DataDirectory:        getEnv("WAL_DATA_DIRECTORY", "./data/gomemdb/wal"),
		FlushingBatchSize:    getEnvInt("WAL_FLUSHING_BATCH_SIZE", defaultWALBatchSize),
		FlushingBatchTimeout: getEnvDuration("WAL_FLUSHING_BATCH_TIMEOUT", defaultWALBatchTimeout*time.Millisecond),
		MaxSegmentSize:       defaultWALMaxSegmentSize,
		Enabled:              walEnabled,
	}

	if maxSegmentSizeStr := getEnv("WAL_MAX_SEGMENT_SIZE", ""); maxSegmentSizeStr != "" {
		if parsedSize, err := utils.ParseSize(maxSegmentSizeStr); err == nil {
			walConfig.MaxSegmentSize = int64(parsedSize)
		}
	}

	config := &Config{
		RunEnv:         getEnv("ENV", "development"),
		Address:        getEnv("ADDRESS", ":8080"),
		MaxConnections: getEnvInt("MAX_CONNECTIONS", defaultMaxConnections),
		MaxMessageSize: getEnv("MAX_MESSAGE_SIZE", defaultMaxMessageSize),
		IdleTimeout:    getEnvDuration("IDLE_TIMEOUT", defaultIdleTimeout),
		WAL:            walConfig,
	}

	if err := config.validate(); err != nil {
		return nil, err
	}

	return config, nil
}

func (c *Config) validate() error {
	return nil
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	var result int
	_, err := fmt.Sscanf(value, "%d", &result)
	if err != nil {
		return defaultValue
	}
	return result
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}

	duration, err := time.ParseDuration(value)
	if err == nil {
		return duration
	}

	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return defaultValue
	}

	return time.Duration(seconds) * time.Second
}
