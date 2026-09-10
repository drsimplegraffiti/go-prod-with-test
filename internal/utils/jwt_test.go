package utils

import (
	"testing"
	"time"
)

func newTestManager() *JWTManager {
	return NewJWTManager("test-secret-at-least-32-characters-long", time.Minute, time.Hour)
}

func TestGenerateAndVerifyAccessToken(t *testing.T) {
	m := newTestManager()

	token, expiresAt, err := m.GenerateAccessToken("user-123", "user")
	if err != nil {
		t.Fatalf("GenerateAccessToken error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if !expiresAt.After(time.Now()) {
		t.Fatal("expected expiry to be in the future")
	}

	claims, err := m.Verify(token, AccessToken)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("expected UserID user-123, got %s", claims.UserID)
	}
	if claims.Role != "user" {
		t.Errorf("expected Role user, got %s", claims.Role)
	}
}

func TestVerifyRejectsWrongType(t *testing.T) {
	m := newTestManager()

	refreshToken, _, err := m.GenerateRefreshToken("user-123", "user")
	if err != nil {
		t.Fatalf("GenerateRefreshToken error: %v", err)
	}

	if _, err := m.Verify(refreshToken, AccessToken); err == nil {
		t.Error("expected error when verifying a refresh token as an access token")
	}
}

func TestVerifyRejectsTamperedToken(t *testing.T) {
	m := newTestManager()

	token, _, err := m.GenerateAccessToken("user-123", "user")
	if err != nil {
		t.Fatalf("GenerateAccessToken error: %v", err)
	}

	tampered := token[:len(token)-2] + "xx"
	if _, err := m.Verify(tampered, AccessToken); err == nil {
		t.Error("expected error for tampered token signature")
	}
}

func TestVerifyRejectsDifferentSecret(t *testing.T) {
	m1 := NewJWTManager("secret-one-at-least-32-characters!!", time.Minute, time.Hour)
	m2 := NewJWTManager("secret-two-at-least-32-characters!!", time.Minute, time.Hour)

	token, _, err := m1.GenerateAccessToken("user-123", "user")
	if err != nil {
		t.Fatalf("GenerateAccessToken error: %v", err)
	}

	if _, err := m2.Verify(token, AccessToken); err == nil {
		t.Error("expected error when verifying with a different secret")
	}
}

func TestVerifyRejectsExpiredToken(t *testing.T) {
	m := NewJWTManager("test-secret-at-least-32-characters-long", -time.Second, time.Hour)

	token, _, err := m.GenerateAccessToken("user-123", "user")
	if err != nil {
		t.Fatalf("GenerateAccessToken error: %v", err)
	}

	if _, err := m.Verify(token, AccessToken); err == nil {
		t.Error("expected error for already-expired token")
	}
}
