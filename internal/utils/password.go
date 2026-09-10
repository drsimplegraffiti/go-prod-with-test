package utils

import (
	"fmt"

	"github.com/example/goapi/internal/pkg/bcrypt"
)

// HashPassword hashes a plaintext password with bcrypt at the given cost.
// Cost is configurable so tests can use a cheap cost (e.g. bcrypt.MinCost)
// while production uses a strong one (>=12).
func HashPassword(password string, cost int) (string, error) {
	if len(password) < 8 {
		return "", fmt.Errorf("password must be at least 8 characters")
	}
	if len(password) > 72 {
		// bcrypt silently truncates beyond 72 bytes; reject explicitly instead.
		return "", fmt.Errorf("password must be at most 72 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword reports whether the plaintext password matches the bcrypt
// hash. It returns false (never an error to the caller) on mismatch so
// call sites can't accidentally leak timing/error information.
func CheckPassword(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
