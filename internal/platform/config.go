// Package platform provides platform-specific configurations and utilities.
package platform

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                 string
	DatabaseURL          string
	Environment          string
	SecretsEncryptionKey string
	ComputeBackend       string
}

func NewConfig() (*Config, error) {
	_ = godotenv.Load() // Ignore error if .env doesn't exist

	return &Config{
		Port:                 getEnv("PORT", "8080"),
		DatabaseURL:          getEnv("DATABASE_URL", "postgres://cloud:cloud@localhost:5433/thecloud"),
		Environment:          getEnv("APP_ENV", "development"),
		SecretsEncryptionKey: os.Getenv("SECRETS_ENCRYPTION_KEY"),
		ComputeBackend:       getEnv("COMPUTE_BACKEND", "docker"),
	}, nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
