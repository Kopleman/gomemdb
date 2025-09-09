package config

import (
	"os"

	"github.com/joho/godotenv"
)

// Config представляет конфигурацию приложения.
type Config struct {
	RunEnv string
}

// LoadConfig загружает конфигурацию из переменных окружения и .env файла.
func LoadConfig() (*Config, error) {
	// Загружаем .env файл, если он существует
	_ = godotenv.Load()

	config := &Config{
		RunEnv: getEnv("ENV", "development"),
	}

	// Проверяем обязательные переменные
	if err := config.validate(); err != nil {
		return nil, err
	}

	return config, nil
}

// validate проверяет наличие обязательных переменных.
func (c *Config) validate() error {
	return nil
}

// getEnv получает значение переменной окружения или возвращает значение по умолчанию.
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
