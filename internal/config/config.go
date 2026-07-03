package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr          string
	DatabaseURL       string
	LogLevelName      string
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration
	MaxPageSize       int
}

func Load() Config {
	return Config{
		HTTPAddr:          env("HTTP_ADDR", ":8080"),
		DatabaseURL:       env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/reporting?sslmode=disable"),
		LogLevelName:      env("LOG_LEVEL", "info"),
		DBMaxOpenConns:    envInt("DB_MAX_OPEN_CONNS", 20),
		DBMaxIdleConns:    envInt("DB_MAX_IDLE_CONNS", 10),
		DBConnMaxLifetime: time.Duration(envInt("DB_CONN_MAX_LIFETIME_MINUTES", 30)) * time.Minute,
		MaxPageSize:       envInt("MAX_PAGE_SIZE", 500),
	}
}

func (c Config) LogLevel() slog.Level {
	switch strings.ToLower(c.LogLevelName) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}
