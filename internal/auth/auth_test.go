package auth_test

import (
	"testing"
	"time"

	"ticket-system/internal/auth"
)

func TestPasswordHashing(t *testing.T) {
	password := "my-secure-password-123"

	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("unexpected error hashing password: %v", err)
	}

	if hash == password {
		t.Errorf("hash should not equal plain password")
	}

	if !auth.CheckPassword(password, hash) {
		t.Errorf("expected password to match hash")
	}

	if auth.CheckPassword("wrong-password", hash) {
		t.Errorf("expected wrong password to fail check")
	}
}

func TestJWTTokenGenerationAndValidation(t *testing.T) {
	secret := "secret-for-testing"
	userID := int64(42)
	email := "test@example.com"

	token, err := auth.GenerateToken(userID, email, secret, 1*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error generating token: %v", err)
	}

	claims, err := auth.ValidateToken(token, secret)
	if err != nil {
		t.Fatalf("unexpected error validating token: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("expected userID %d, got %d", userID, claims.UserID)
	}

	if claims.Email != email {
		t.Errorf("expected email %s, got %s", email, claims.Email)
	}

	// Test validation with wrong secret
	_, err = auth.ValidateToken(token, "wrong-secret")
	if err == nil {
		t.Errorf("expected error when validating with incorrect secret")
	}
}
