package auth

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// DefaultBcryptCost defines the work factor for password hashing.
// Cost 12 provides strong protection against brute force and GPU cracking.
const DefaultBcryptCost = 12

// HashPassword hashes a raw plaintext password using bcrypt with work factor 12.
func HashPassword(password string) (string, error) {
	if len(password) == 0 {
		return "", errors.New("password cannot be empty")
	}

	bytes, err := bcrypt.GenerateFromPassword([]byte(password), DefaultBcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(bytes), nil
}

// CheckPassword verifies that a plaintext password matches a stored bcrypt hash.
func CheckPassword(hashedPassword, password string) error {
	if len(password) == 0 || len(hashedPassword) == 0 {
		return ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)); err != nil {
		return ErrInvalidCredentials
	}
	return nil
}
