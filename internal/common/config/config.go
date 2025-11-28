package config

import (
	"os"
	"strconv"
)

// ============================================================
// Configuration
// ============================================================

type Config struct {
	Port         string
	Environment  string
	ReadTimeout  int
	WriteTimeout int
}

// Load загружает конфигурацию из переменных окружения
func Load() *Config {
	return &Config{
		Port:         getEnv("PORT", "3000"),
		Environment:  getEnv("ENV", "development"),
		ReadTimeout:  getEnvAsInt("READ_TIMEOUT", 10),
		WriteTimeout: getEnvAsInt("WRITE_TIMEOUT", 10),
	}
}

func getEnv(key, defaultVal string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultVal
}
