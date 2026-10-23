package auth_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Dhanya562004/quickcart-go/internal/auth"
)

func TestTokenManager_GenerateAndValidate(t *testing.T) {
	mockClock := &auth.MockClock{CurrentTime: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)}
	cfg := auth.DefaultTokenConfig("super-secret-hmac-key-for-quickcart-tests")
	cfg.AccessTokenTTL = 15 * time.Minute
	tm := auth.NewTokenManager(cfg, mockClock)

	user := auth.User{
		ID:    uuid.New(),
		Email: "customer@quickcart.in",
		Role:  auth.RoleCustomer,
	}

	// 1. Generate Token Pair
	pair, err := tm.GenerateTokenPair(user)
	require.NoError(t, err)
	require.NotEmpty(t, pair.AccessToken)
	require.NotEmpty(t, pair.RefreshToken)
	assert.Equal(t, int64(900), pair.ExpiresIn)

	// 2. Validate Access Token within TTL
	claims, err := tm.ValidateAccessToken(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, user.ID, claims.UserID)
	assert.Equal(t, user.Email, claims.Email)
	assert.Equal(t, auth.RoleCustomer, claims.Role)
	assert.Equal(t, "quickcart-engine", claims.Issuer)

	// 3. Test Injected Clock Expiry (advance 16 minutes without time.Sleep)
	mockClock.Advance(16 * time.Minute)

	expiredClaims, err := tm.ValidateAccessToken(pair.AccessToken)
	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrExpiredToken)
	assert.Nil(t, expiredClaims)
}

func TestTokenManager_TamperedToken_Rejected(t *testing.T) {
	mockClock := &auth.MockClock{CurrentTime: time.Now().UTC()}
	cfg := auth.DefaultTokenConfig("valid-key-123456789012345678901234")
	tm := auth.NewTokenManager(cfg, mockClock)

	user := auth.User{
		ID:    uuid.New(),
		Email: "user@quickcart.in",
		Role:  auth.RoleCustomer,
	}

	pair, err := tm.GenerateTokenPair(user)
	require.NoError(t, err)

	// Tamper with the token signature
	tamperedToken := pair.AccessToken + "tampered"
	claims, err := tm.ValidateAccessToken(tamperedToken)
	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
	assert.Nil(t, claims)
}

func TestTokenManager_DifferentKey_Rejected(t *testing.T) {
	mockClock := &auth.MockClock{CurrentTime: time.Now().UTC()}
	tm1 := auth.NewTokenManager(auth.DefaultTokenConfig("secret-key-one-1234567890123456"), mockClock)
	tm2 := auth.NewTokenManager(auth.DefaultTokenConfig("secret-key-two-1234567890123456"), mockClock)

	user := auth.User{ID: uuid.New(), Email: "user@test.com", Role: auth.RoleAdmin}
	pair, err := tm1.GenerateTokenPair(user)
	require.NoError(t, err)

	// tm2 with different key must reject token signed by tm1
	claims, err := tm2.ValidateAccessToken(pair.AccessToken)
	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrInvalidToken)
	assert.Nil(t, claims)
}
