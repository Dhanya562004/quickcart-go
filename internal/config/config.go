package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Environment represents the deployment stage.
type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvStaging     Environment = "staging"
	EnvProduction  Environment = "production"
)

// Config holds all runtime application configuration.
// In QuickCart, all configuration is driven by environment variables with sensible defaults.
type Config struct {
	// Server configuration
	ServerPort      string        `json:"server_port"`
	ServerHost      string        `json:"server_host"`
	Environment     Environment   `json:"environment"`
	LogLevel        string        `json:"log_level"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`

	// Database (PostgreSQL via pgxpool)
	DatabaseURL        string        `json:"database_url"`
	DBMaxConns         int32         `json:"db_max_conns"`
	DBMinConns         int32         `json:"db_min_conns"`
	DBMaxConnLifetime  time.Duration `json:"db_max_conn_lifetime"`
	DBMaxConnIdleTime  time.Duration `json:"db_max_conn_idle_time"`
	DBHealthCheckDelay time.Duration `json:"db_health_check_delay"`

	// Redis configuration
	RedisURL string `json:"redis_url"`

	// Concurrency Worker Pool
	WorkerPoolSize     int           `json:"worker_pool_size"`
	WorkerDrainTimeout time.Duration `json:"worker_drain_timeout"`
}

// Load loads application configuration from environment variables with safe defaults.
func Load() (*Config, error) {
	cfg := &Config{
		ServerPort:         getEnv("SERVER_PORT", "8080"),
		ServerHost:         getEnv("SERVER_HOST", "0.0.0.0"),
		Environment:        Environment(getEnv("APP_ENV", string(EnvDevelopment))),
		LogLevel:           strings.ToLower(getEnv("LOG_LEVEL", "info")),
		ShutdownTimeout:    getEnvDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://quickcart:quickcart@localhost:5432/quickcart?sslmode=disable"),
		DBMaxConns:         int32(getEnvInt("DB_MAX_CONNS", 25)),
		DBMinConns:         int32(getEnvInt("DB_MIN_CONNS", 5)),
		DBMaxConnLifetime:  getEnvDuration("DB_MAX_CONN_LIFETIME", 1*time.Hour),
		DBMaxConnIdleTime:  getEnvDuration("DB_MAX_CONN_IDLE_TIME", 30*time.Minute),
		DBHealthCheckDelay: getEnvDuration("DB_HEALTH_CHECK_DELAY", 1*time.Minute),
		RedisURL:           getEnv("REDIS_URL", "redis://localhost:6379/0"),
		WorkerPoolSize:     getEnvInt("WORKER_POOL_SIZE", 4),
		WorkerDrainTimeout: getEnvDuration("WORKER_DRAIN_TIMEOUT", 5*time.Second),
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// Validate verifies that critical configuration parameters are within operational bounds.
func (c *Config) Validate() error {
	if c.ServerPort == "" {
		return errors.New("SERVER_PORT cannot be empty")
	}

	portNum, err := strconv.Atoi(c.ServerPort)
	if err != nil || portNum < 1 || portNum > 65535 {
		return fmt.Errorf("invalid SERVER_PORT: %s (must be between 1 and 65535)", c.ServerPort)
	}

	switch c.Environment {
	case EnvDevelopment, EnvStaging, EnvProduction:
		// valid
	default:
		return fmt.Errorf("invalid APP_ENV: %s (allowed: development, staging, production)", c.Environment)
	}

	switch c.LogLevel {
	case "debug", "info", "warn", "error":
		// valid
	default:
		return fmt.Errorf("invalid LOG_LEVEL: %s (allowed: debug, info, warn, error)", c.LogLevel)
	}

	if c.DatabaseURL == "" {
		return errors.New("DATABASE_URL cannot be empty")
	}

	if c.DBMaxConns <= 0 {
		return errors.New("DB_MAX_CONNS must be greater than 0")
	}

	if c.DBMinConns < 0 {
		return errors.New("DB_MIN_CONNS cannot be negative")
	}

	if c.DBMinConns > c.DBMaxConns {
		return fmt.Errorf("DB_MIN_CONNS (%d) cannot exceed DB_MAX_CONNS (%d)", c.DBMinConns, c.DBMaxConns)
	}

	if c.WorkerPoolSize <= 0 {
		return errors.New("WORKER_POOL_SIZE must be greater than 0")
	}

	if c.WorkerDrainTimeout <= 0 {
		return errors.New("WORKER_DRAIN_TIMEOUT must be greater than 0")
	}

	return nil
}

// Address returns the combined host:port listen address.
func (c *Config) Address() string {
	return fmt.Sprintf("%s:%s", c.ServerHost, c.ServerPort)
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(valStr)
	if err != nil {
		return defaultVal
	}
	return d
}
