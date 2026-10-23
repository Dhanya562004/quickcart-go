package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Role constants defining access privileges in QuickCart.
const (
	RoleCustomer = "customer"
	RoleAdmin    = "admin"
	RolePicker   = "picker"
	RoleDelivery = "delivery_partner"
)

var (
	ErrInvalidCredentials       = errors.New("invalid email or password")
	ErrUserAlreadyExists        = errors.New("a user with this email already exists")
	ErrUserNotFound             = errors.New("user not found")
	ErrInvalidToken             = errors.New("invalid or malformed token")
	ErrExpiredToken             = errors.New("token has expired")
	ErrRefreshTokenRevoked      = errors.New("refresh token has been revoked or expired")
	ErrUnauthorized             = errors.New("unauthorized; authentication required")
	ErrForbidden                = errors.New("forbidden; insufficient permissions")
	ErrRefreshTokenReuseBlocked = errors.New("refresh token reuse detected or token already invalidated")
)

// User represents an authenticated user in QuickCart.
type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // Never expose in JSON serialization
	FullName     string    `json:"full_name"`
	Phone        string    `json:"phone"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Claims represents the JWT claims payload signed by QuickCart.
type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	Role   string    `json:"role"`
	jwt.RegisteredClaims
}

// TokenPair represents an access and refresh token returned upon login or rotation.
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int64     `json:"expires_in"` // Access token TTL in seconds
	ExpiresAt    time.Time `json:"expires_at"`
}

// RefreshToken represents a stored refresh token record in PostgreSQL.
type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	Revoked   bool
	ExpiresAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// RegisterRequest represents the user registration payload.
// Invariant: Administrative roles cannot be assigned via public registration.
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	FullName string `json:"full_name" binding:"required,min=2"`
	Phone    string `json:"phone" binding:"required,min=10"`
}

// LoginRequest represents credentials supplied for authentication.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// RefreshRequest supplies a refresh token to obtain a new token pair.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// AuthResponse encapsulates authentication results returned to clients.
type AuthResponse struct {
	Tokens TokenPair `json:"tokens"`
	User   User      `json:"user"`
}
