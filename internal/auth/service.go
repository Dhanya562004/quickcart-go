package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Service defines authentication and token management operations.
type Service interface {
	Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error)
	Login(ctx context.Context, req LoginRequest) (*AuthResponse, error)
	Refresh(ctx context.Context, req RefreshRequest) (*AuthResponse, error)
	Logout(ctx context.Context, req RefreshRequest) error
	GetUserByID(ctx context.Context, id uuid.UUID) (*User, error)
}

type service struct {
	repo     Repository
	tokenMgr TokenManager
	logger   *slog.Logger
}

// NewService constructs an authentication Service.
func NewService(repo Repository, tokenMgr TokenManager, logger *slog.Logger) Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &service{
		repo:     repo,
		tokenMgr: tokenMgr,
		logger:   logger,
	}
}

// Register creates a new customer user and issues initial access and refresh tokens.
// Invariant: Public registration strictly creates customers; admin roles cannot be self-assigned.
func (s *service) Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error) {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" {
		return nil, errors.New("email cannot be empty")
	}

	// 1. Hash password with bcrypt cost 12
	hashedPassword, err := HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	// 2. Persist user with RoleCustomer enforced
	user, err := s.repo.CreateUser(ctx, User{
		Email:        req.Email,
		PasswordHash: hashedPassword,
		FullName:     strings.TrimSpace(req.FullName),
		Phone:        strings.TrimSpace(req.Phone),
		Role:         RoleCustomer, // Hardcoded: never trust client for role
	})
	if err != nil {
		return nil, err
	}

	// 3. Issue Token Pair
	tokenPair, err := s.tokenMgr.GenerateTokenPair(*user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token pair: %w", err)
	}

	// 4. Store hashed refresh token in database (7 days TTL)
	refreshHash := s.tokenMgr.HashRefreshToken(tokenPair.RefreshToken)
	refreshExpiresAt := s.tokenMgr.Clock().Now().Add(7 * 24 * time.Hour)
	if err := s.repo.SaveRefreshToken(ctx, user.ID, refreshHash, refreshExpiresAt); err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	// Safe logging: Never log passwords or tokens
	s.logger.Info("user successfully registered", "user_id", user.ID, "email", user.Email, "role", user.Role)

	return &AuthResponse{
		Tokens: *tokenPair,
		User:   *user,
	}, nil
}

// Login validates user credentials and issues a fresh token pair.
func (s *service) Login(ctx context.Context, req LoginRequest) (*AuthResponse, error) {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	// 1. Find user by email
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	// 2. Verify password
	if err := CheckPassword(user.PasswordHash, req.Password); err != nil {
		return nil, ErrInvalidCredentials
	}

	// 3. Generate token pair
	tokenPair, err := s.tokenMgr.GenerateTokenPair(*user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token pair: %w", err)
	}

	// 4. Save refresh token
	refreshHash := s.tokenMgr.HashRefreshToken(tokenPair.RefreshToken)
	refreshExpiresAt := s.tokenMgr.Clock().Now().Add(7 * 24 * time.Hour)
	if err := s.repo.SaveRefreshToken(ctx, user.ID, refreshHash, refreshExpiresAt); err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	s.logger.Info("user successfully logged in", "user_id", user.ID, "email", user.Email, "role", user.Role)

	return &AuthResponse{
		Tokens: *tokenPair,
		User:   *user,
	}, nil
}

// Refresh performs atomic refresh-token rotation to prevent concurrent reuse.
// If an active refresh token is rotated, it is immediately revoked and a new token pair is issued.
// If a revoked or invalid token is presented, the request is rejected with ErrRefreshTokenReuseBlocked.
func (s *service) Refresh(ctx context.Context, req RefreshRequest) (*AuthResponse, error) {
	if strings.TrimSpace(req.RefreshToken) == "" {
		return nil, errors.New("refresh token cannot be empty")
	}

	tokenHash := s.tokenMgr.HashRefreshToken(req.RefreshToken)

	// 1. Atomically pick and revoke the active token
	activeToken, err := s.repo.PickActiveRefreshTokenForRotation(ctx, tokenHash)
	if err != nil {
		s.logger.Warn("refresh token rotation rejected; invalid or reused token", "hash_prefix", tokenHash[:8])
		return nil, ErrRefreshTokenReuseBlocked
	}

	// 2. Fetch associated user
	user, err := s.repo.GetUserByID(ctx, activeToken.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user for token: %w", err)
	}

	// 3. Generate new token pair
	newTokenPair, err := s.tokenMgr.GenerateTokenPair(*user)
	if err != nil {
		return nil, fmt.Errorf("failed to issue new token pair: %w", err)
	}

	// 4. Persist newly rotated refresh token
	newRefreshHash := s.tokenMgr.HashRefreshToken(newTokenPair.RefreshToken)
	newExpiresAt := s.tokenMgr.Clock().Now().Add(7 * 24 * time.Hour)
	if err := s.repo.SaveRefreshToken(ctx, user.ID, newRefreshHash, newExpiresAt); err != nil {
		return nil, fmt.Errorf("failed to save new refresh token: %w", err)
	}

	s.logger.Info("refresh token rotated successfully", "user_id", user.ID)

	return &AuthResponse{
		Tokens: *newTokenPair,
		User:   *user,
	}, nil
}

// Logout revokes the given refresh token.
func (s *service) Logout(ctx context.Context, req RefreshRequest) error {
	if strings.TrimSpace(req.RefreshToken) == "" {
		return nil
	}
	tokenHash := s.tokenMgr.HashRefreshToken(req.RefreshToken)
	return s.repo.RevokeRefreshToken(ctx, tokenHash)
}

func (s *service) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	return s.repo.GetUserByID(ctx, id)
}
