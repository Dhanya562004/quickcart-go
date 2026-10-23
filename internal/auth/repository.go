package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Dhanya562004/quickcart-go/internal/db"
)

// Repository defines data access methods for users and refresh tokens.
type Repository interface {
	CreateUser(ctx context.Context, u User) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*User, error)
	SaveRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error
	PickActiveRefreshTokenForRotation(ctx context.Context, tokenHash string) (*RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
	RevokeAllUserRefreshTokens(ctx context.Context, userID uuid.UUID) error
}

type pgxRepository struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

// NewRepository creates a PostgreSQL-backed auth repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgxRepository{
		pool:    pool,
		queries: db.New(pool),
	}
}

func (r *pgxRepository) CreateUser(ctx context.Context, u User) (*User, error) {
	row, err := r.queries.CreateUser(ctx, db.CreateUserParams{
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		FullName:     u.FullName,
		Phone:        u.Phone,
		Role:         u.Role,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrUserAlreadyExists
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return toDomainUser(row), nil
}

func (r *pgxRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	row, err := r.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return toDomainUser(row), nil
}

func (r *pgxRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	var pgID pgtype.UUID
	_ = pgID.Scan(id.String())

	row, err := r.queries.GetUserByID(ctx, pgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}
	return toDomainUser(row), nil
}

func (r *pgxRepository) SaveRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	var pgUserID pgtype.UUID
	_ = pgUserID.Scan(userID.String())

	_, err := r.queries.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		UserID:    pgUserID,
		TokenHash: tokenHash,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to save refresh token: %w", err)
	}
	return nil
}

// PickActiveRefreshTokenForRotation atomically revokes an active refresh token in a single query.
// Guarantees:
// 1. If the token is already revoked, returns ErrNoRows (concurrent reuse attempt prevented).
// 2. If the token is expired, returns ErrNoRows.
func (r *pgxRepository) PickActiveRefreshTokenForRotation(ctx context.Context, tokenHash string) (*RefreshToken, error) {
	row, err := r.queries.PickActiveRefreshTokenForRotation(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRefreshTokenRevoked
		}
		return nil, fmt.Errorf("failed to rotate refresh token: %w", err)
	}

	var uID uuid.UUID
	if row.UserID.Valid {
		uID = row.UserID.Bytes
	}

	var id uuid.UUID
	if row.ID.Valid {
		id = row.ID.Bytes
	}

	return &RefreshToken{
		ID:        id,
		UserID:    uID,
		TokenHash: row.TokenHash,
		Revoked:   row.Revoked,
		ExpiresAt: row.ExpiresAt.Time,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}, nil
}

func (r *pgxRepository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := r.queries.RevokeRefreshToken(ctx, tokenHash)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("failed to revoke refresh token: %w", err)
	}
	return nil
}

func (r *pgxRepository) RevokeAllUserRefreshTokens(ctx context.Context, userID uuid.UUID) error {
	var pgUserID pgtype.UUID
	_ = pgUserID.Scan(userID.String())
	return r.queries.RevokeAllUserRefreshTokens(ctx, pgUserID)
}

func toDomainUser(u db.User) *User {
	var id uuid.UUID
	if u.ID.Valid {
		id = u.ID.Bytes
	}

	return &User{
		ID:           id,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		FullName:     u.FullName,
		Phone:        u.Phone,
		Role:         u.Role,
		CreatedAt:    u.CreatedAt.Time,
		UpdatedAt:    u.UpdatedAt.Time,
	}
}

// --- In-Memory Fake Repository for Testing ---

type FakeRepository struct {
	mu           sync.RWMutex
	users        map[uuid.UUID]*User
	usersByEmail map[string]*User
	tokens       map[string]*RefreshToken
}

// NewFakeRepository constructs an in-memory auth repository for tests.
func NewFakeRepository() *FakeRepository {
	return &FakeRepository{
		users:        make(map[uuid.UUID]*User),
		usersByEmail: make(map[string]*User),
		tokens:       make(map[string]*RefreshToken),
	}
}

func (f *FakeRepository) CreateUser(_ context.Context, u User) (*User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.usersByEmail[u.Email]; exists {
		return nil, ErrUserAlreadyExists
	}

	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	u.CreatedAt = time.Now().UTC()
	u.UpdatedAt = u.CreatedAt

	userCopy := u
	f.users[u.ID] = &userCopy
	f.usersByEmail[u.Email] = &userCopy
	return &userCopy, nil
}

func (f *FakeRepository) GetUserByEmail(_ context.Context, email string) (*User, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	u, exists := f.usersByEmail[email]
	if !exists {
		return nil, ErrUserNotFound
	}
	userCopy := *u
	return &userCopy, nil
}

func (f *FakeRepository) GetUserByID(_ context.Context, id uuid.UUID) (*User, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	u, exists := f.users[id]
	if !exists {
		return nil, ErrUserNotFound
	}
	userCopy := *u
	return &userCopy, nil
}

func (f *FakeRepository) SaveRefreshToken(_ context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.tokens[tokenHash] = &RefreshToken{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: tokenHash,
		Revoked:   false,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	return nil
}

func (f *FakeRepository) PickActiveRefreshTokenForRotation(_ context.Context, tokenHash string) (*RefreshToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	t, exists := f.tokens[tokenHash]
	if !exists || t.Revoked || time.Now().After(t.ExpiresAt) {
		return nil, ErrRefreshTokenRevoked
	}

	t.Revoked = true
	t.UpdatedAt = time.Now().UTC()
	tokenCopy := *t
	return &tokenCopy, nil
}

func (f *FakeRepository) RevokeRefreshToken(_ context.Context, tokenHash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if t, exists := f.tokens[tokenHash]; exists {
		t.Revoked = true
		t.UpdatedAt = time.Now().UTC()
	}
	return nil
}

func (f *FakeRepository) RevokeAllUserRefreshTokens(_ context.Context, userID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, t := range f.tokens {
		if t.UserID == userID {
			t.Revoked = true
			t.UpdatedAt = time.Now().UTC()
		}
	}
	return nil
}
