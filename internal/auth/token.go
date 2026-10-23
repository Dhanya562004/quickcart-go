package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Clock abstracts system time to allow deterministic testing of token expiry.
type Clock interface {
	Now() time.Time
}

// RealClock implements Clock using the standard system time.
type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now().UTC()
}

// MockClock implements Clock for testing token expiry.
type MockClock struct {
	CurrentTime time.Time
}

func (m *MockClock) Now() time.Time {
	return m.CurrentTime
}

func (m *MockClock) Advance(d time.Duration) {
	m.CurrentTime = m.CurrentTime.Add(d)
}

// TokenConfig encapsulates token lifetimes and HMAC signing keys.
type TokenConfig struct {
	SecretKey       []byte
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	Issuer          string
}

// DefaultTokenConfig provides standard production defaults.
func DefaultTokenConfig(secretKey string) TokenConfig {
	return TokenConfig{
		SecretKey:       []byte(secretKey),
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
		Issuer:          "quickcart-engine",
	}
}

// TokenManager defines the contract for JWT creation, validation, and token hashing.
type TokenManager interface {
	GenerateTokenPair(user User) (*TokenPair, error)
	ValidateAccessToken(tokenStr string) (*Claims, error)
	HashRefreshToken(rawToken string) string
	Clock() Clock
}

type jwtTokenManager struct {
	cfg   TokenConfig
	clock Clock
}

// NewTokenManager initializes a production TokenManager with clock injection.
func NewTokenManager(cfg TokenConfig, clock Clock) TokenManager {
	if clock == nil {
		clock = RealClock{}
	}
	if cfg.AccessTokenTTL <= 0 {
		cfg.AccessTokenTTL = 15 * time.Minute
	}
	if cfg.RefreshTokenTTL <= 0 {
		cfg.RefreshTokenTTL = 7 * 24 * time.Hour
	}
	if cfg.Issuer == "" {
		cfg.Issuer = "quickcart-engine"
	}

	return &jwtTokenManager{
		cfg:   cfg,
		clock: clock,
	}
}

func (m *jwtTokenManager) Clock() Clock {
	return m.clock
}

// GenerateTokenPair issues a short-lived access token and a secure cryptographically random refresh token.
func (m *jwtTokenManager) GenerateTokenPair(user User) (*TokenPair, error) {
	now := m.clock.Now()
	accessExpiresAt := now.Add(m.cfg.AccessTokenTTL)

	// 1. Construct JWT Claims
	claims := Claims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.cfg.Issuer,
			Subject:   user.ID.String(),
			Audience:  jwt.ClaimStrings{"quickcart-api"},
			ExpiresAt: jwt.NewNumericDate(accessExpiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.New().String(),
		},
	}

	// 2. Sign JWT using HMAC-SHA256
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedAccess, err := token.SignedString(m.cfg.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// 3. Generate high-entropy 256-bit random refresh token (64 hex characters)
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random refresh token: %w", err)
	}
	rawRefreshToken := hex.EncodeToString(rawBytes)

	return &TokenPair{
		AccessToken:  signedAccess,
		RefreshToken: rawRefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(m.cfg.AccessTokenTTL.Seconds()),
		ExpiresAt:    accessExpiresAt,
	}, nil
}

// ValidateAccessToken verifies token signature, algorithm, and expiration.
func (m *jwtTokenManager) ValidateAccessToken(tokenStr string) (*Claims, error) {
	if tokenStr == "" {
		return nil, ErrUnauthorized
	}

	// Parse and validate with custom KeyFunc and clock injection
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		// Strict check: enforce HMAC-SHA256 and reject "none" algorithm attacks
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.cfg.SecretKey, nil
	}, jwt.WithTimeFunc(m.clock.Now))

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// HashRefreshToken generates a deterministic SHA-256 fingerprint for database storage.
func (m *jwtTokenManager) HashRefreshToken(rawToken string) string {
	h := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(h[:])
}
