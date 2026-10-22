package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Defaults(t *testing.T) {
	// Clear any environment overrides
	os.Clearenv()

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "8080", cfg.ServerPort)
	assert.Equal(t, "0.0.0.0", cfg.ServerHost)
	assert.Equal(t, EnvDevelopment, cfg.Environment)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, 5*time.Second, cfg.ShutdownTimeout)
	assert.Equal(t, int32(25), cfg.DBMaxConns)
	assert.Equal(t, int32(5), cfg.DBMinConns)
	assert.Equal(t, "0.0.0.0:8080", cfg.Address())
}

func TestConfig_Validation(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(c *Config)
		expectedErr string
	}{
		{
			name: "invalid port non-numeric",
			modify: func(c *Config) {
				c.ServerPort = "invalid"
			},
			expectedErr: "invalid SERVER_PORT",
		},
		{
			name: "invalid port out of range",
			modify: func(c *Config) {
				c.ServerPort = "70000"
			},
			expectedErr: "must be between 1 and 65535",
		},
		{
			name: "invalid environment",
			modify: func(c *Config) {
				c.Environment = "invalid_env"
			},
			expectedErr: "invalid APP_ENV",
		},
		{
			name: "invalid log level",
			modify: func(c *Config) {
				c.LogLevel = "verbose"
			},
			expectedErr: "invalid LOG_LEVEL",
		},
		{
			name: "empty database URL",
			modify: func(c *Config) {
				c.DatabaseURL = ""
			},
			expectedErr: "DATABASE_URL cannot be empty",
		},
		{
			name: "DBMaxConns zero",
			modify: func(c *Config) {
				c.DBMaxConns = 0
			},
			expectedErr: "DB_MAX_CONNS must be greater than 0",
		},
		{
			name: "DBMinConns exceeds DBMaxConns",
			modify: func(c *Config) {
				c.DBMinConns = 30
				c.DBMaxConns = 10
			},
			expectedErr: "DB_MIN_CONNS (30) cannot exceed DB_MAX_CONNS (10)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load()
			require.NoError(t, err)

			tt.modify(cfg)
			err = cfg.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestConfig_EnvironmentOverrides(t *testing.T) {
	t.Setenv("SERVER_PORT", "9090")
	t.Setenv("SERVER_HOST", "127.0.0.1")
	t.Setenv("APP_ENV", "production")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("DB_MAX_CONNS", "50")
	t.Setenv("DB_MIN_CONNS", "10")
	t.Setenv("SHUTDOWN_TIMEOUT", "10s")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "9090", cfg.ServerPort)
	assert.Equal(t, "127.0.0.1", cfg.ServerHost)
	assert.Equal(t, EnvProduction, cfg.Environment)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, int32(50), cfg.DBMaxConns)
	assert.Equal(t, int32(10), cfg.DBMinConns)
	assert.Equal(t, 10*time.Second, cfg.ShutdownTimeout)
	assert.Equal(t, "127.0.0.1:9090", cfg.Address())
}
