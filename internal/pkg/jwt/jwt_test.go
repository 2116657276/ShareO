package jwt

import (
	"strings"
	"testing"
	"time"
)

func init() {
	Init("test-secret-key-for-unit-tests", 72)
}

func TestGenerateAndParse_RoundTrip(t *testing.T) {
	token, err := GenerateToken(1, "testuser", "user")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	if token == "" {
		t.Fatal("token is empty")
	}

	claims, err := ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}
	if claims.UserID != 1 {
		t.Errorf("UserID = %d, want 1", claims.UserID)
	}
	if claims.Username != "testuser" {
		t.Errorf("Username = %s, want testuser", claims.Username)
	}
	if claims.Role != "user" {
		t.Errorf("Role = %s, want user", claims.Role)
	}
	if claims.Issuer != "shareo" {
		t.Errorf("Issuer = %s, want shareo", claims.Issuer)
	}
}

func TestGenerateToken_AdminRole(t *testing.T) {
	token, err := GenerateToken(100, "admin", "admin")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	claims, err := ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}
	if claims.UserID != 100 {
		t.Errorf("UserID = %d, want 100", claims.UserID)
	}
	if claims.Role != "admin" {
		t.Errorf("Role = %s, want admin", claims.Role)
	}
}

func TestParseToken_InvalidFormat(t *testing.T) {
	_, err := ParseToken("not-a-valid-jwt-token")
	if err == nil {
		t.Fatal("expected error for invalid token, got nil")
	}
}

func TestParseToken_EmptyString(t *testing.T) {
	_, err := ParseToken("")
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}
}

func TestParseToken_Tampered(t *testing.T) {
	token, err := GenerateToken(1, "testuser", "user")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	// Tamper with the payload by changing a character in the signature
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatal("unexpected token format")
	}
	tamperedToken := parts[0] + "." + parts[1] + ".tampered_signature"

	_, err = ParseToken(tamperedToken)
	if err == nil {
		t.Fatal("expected error for tampered token, got nil")
	}
}

func TestParseToken_Expired(t *testing.T) {
	// Init with 0 hours to get immediately expired tokens
	Init("test-secret-expired", 0)
	token, err := GenerateToken(1, "testuser", "user")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	// Small sleep to ensure expiry
	time.Sleep(10 * time.Millisecond)

	_, err = ParseToken(token)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}

	// Restore normal init
	Init("test-secret-key-for-unit-tests", 72)
}

func TestParseToken_WrongKey(t *testing.T) {
	Init("key-a", 72)
	token, err := GenerateToken(1, "testuser", "user")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	// Switch to different key
	Init("key-b", 72)

	_, err = ParseToken(token)
	if err == nil {
		t.Fatal("expected error for token signed with different key, got nil")
	}

	// Restore
	Init("test-secret-key-for-unit-tests", 72)
}

func TestGenerateToken_DifferentUsers(t *testing.T) {
	users := []struct {
		id       int64
		username string
		role     string
	}{
		{1, "alice", "user"},
		{2, "bob", "user"},
		{99, "root", "admin"},
		{1000, "long_username_test", "user"},
	}

	for _, u := range users {
		token, err := GenerateToken(u.id, u.username, u.role)
		if err != nil {
			t.Errorf("GenerateToken(%d, %s, %s) failed: %v", u.id, u.username, u.role, err)
			continue
		}
		claims, err := ParseToken(token)
		if err != nil {
			t.Errorf("ParseToken for %s failed: %v", u.username, err)
			continue
		}
		if claims.UserID != u.id || claims.Username != u.username || claims.Role != u.role {
			t.Errorf("mismatch for %s: got id=%d,name=%s,role=%s", u.username, claims.UserID, claims.Username, claims.Role)
		}
	}
}

func TestToken_ContainsExpiry(t *testing.T) {
	token, err := GenerateToken(1, "testuser", "user")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	claims, err := ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}

	if claims.ExpiresAt == nil {
		t.Fatal("token has no expiry")
	}
	if claims.IssuedAt == nil {
		t.Fatal("token has no issued-at")
	}
	if claims.ExpiresAt.Before(claims.IssuedAt.Time) {
		t.Error("expiry is before issue time")
	}
}

func TestParseToken_NilClaims(t *testing.T) {
	// Generate but then try parsing an empty token structure
	_, err := ParseToken("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30=.bad")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
