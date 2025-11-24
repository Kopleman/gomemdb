package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	defaultMaxConnections = 100
	defaultMaxMessageSize = "4KB"
	defaultIdleTimeout    = time.Minute
)

// Config представляет конфигурацию приложения.
type Config struct {
	RunEnv         string
	Address        string
	MaxMessageSize string
	MaxConnections int
	IdleTimeout    time.Duration
}

// LoadConfig загружает конфигурацию из переменных окружения и .env файла.
func LoadConfig() (*Config, error) {
	// Загружаем .env файл, если он существует
	_ = godotenv.Load()

	config := &Config{
		RunEnv:         getEnv("ENV", "development"),
		Address:        getEnv("ADDRESS", ":8080"),
		MaxConnections: getEnvInt("MAX_CONNECTIONS", defaultMaxConnections),
		MaxMessageSize: getEnv("MAX_MESSAGE_SIZE", defaultMaxMessageSize),
		IdleTimeout:    getEnvDuration("IDLE_TIMEOUT", defaultIdleTimeout),
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
