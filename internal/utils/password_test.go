package utils

import "testing"

func TestHashAndCheckPassword(t *testing.T) {
	const cost = 4 // cheap cost for fast tests; production uses config.BcryptCost (>=12)

	hash, err := HashPassword("correct-horse-battery", cost)
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if hash == "correct-horse-battery" {
		t.Fatal("hash must not equal the plaintext password")
	}

	if !CheckPassword(hash, "correct-horse-battery") {
		t.Error("expected correct password to match")
	}
	if CheckPassword(hash, "wrong-password") {
		t.Error("expected incorrect password to not match")
	}
}

func TestHashPasswordRejectsTooShort(t *testing.T) {
	if _, err := HashPassword("short", 4); err == nil {
		t.Error("expected error for password under 8 characters")
	}
}

func TestHashPasswordRejectsTooLong(t *testing.T) {
	long := make([]byte, 73)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := HashPassword(string(long), 4); err == nil {
		t.Error("expected error for password over 72 characters")
	}
}

func TestCheckPasswordAgainstGarbageHash(t *testing.T) {
	if CheckPassword("not-a-real-bcrypt-hash", "anything") {
		t.Error("expected malformed hash to never match")
	}
}
