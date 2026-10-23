package auth_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Dhanya562004/quickcart-go/internal/auth"
)

func TestAuthService_NoCredentialsOrTokensInLogs(t *testing.T) {
	var logBuf bytes.Buffer
	testLogger := slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	repo := auth.NewFakeRepository()
	tm := auth.NewTokenManager(auth.DefaultTokenConfig("test-secret-key-32-bytes-long-1234"), nil)
	svc := auth.NewService(repo, tm, testLogger)

	ctx := context.Background()
	secretPassword := "SuperSensitivePassword999!"

	// 1. Register
	regRes, err := svc.Register(ctx, auth.RegisterRequest{
		Email:    "leaktest@quickcart.in",
		Password: secretPassword,
		FullName: "Security Tester",
		Phone:    "9876543299",
	})
	require.NoError(t, err)

	// 2. Login
	loginRes, err := svc.Login(ctx, auth.LoginRequest{
		Email:    "leaktest@quickcart.in",
		Password: secretPassword,
	})
	require.NoError(t, err)

	// 3. Refresh
	_, err = svc.Refresh(ctx, auth.RefreshRequest{
		RefreshToken: loginRes.Tokens.RefreshToken,
	})
	require.NoError(t, err)

	logOutput := logBuf.String()

	// Assert: Password must NEVER appear in logs
	assert.False(t, strings.Contains(logOutput, secretPassword), "plaintext password must never appear in log records")

	// Assert: Raw access tokens must NEVER appear in logs
	assert.False(t, strings.Contains(logOutput, regRes.Tokens.AccessToken), "raw access token must never appear in log records")
	assert.False(t, strings.Contains(logOutput, loginRes.Tokens.AccessToken), "raw access token must never appear in log records")

	// Assert: Raw refresh tokens must NEVER appear in logs
	assert.False(t, strings.Contains(logOutput, regRes.Tokens.RefreshToken), "raw refresh token must never appear in log records")
	assert.False(t, strings.Contains(logOutput, loginRes.Tokens.RefreshToken), "raw refresh token must never appear in log records")
}
