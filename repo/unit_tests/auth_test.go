package unit_tests

import (
	"testing"
	"time"

	"campusrec/internal/middleware"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-jwt-secret-key-for-unit-tests"

func TestGenerateToken(t *testing.T) {
	token, err := middleware.GenerateToken("user-123", "admin", "testuser", "Test User", testSecret)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	if token == "" {
		t.Fatal("GenerateToken returned empty token")
	}
}

func TestGenerateTokenParseRoundTrip(t *testing.T) {
	userID := "user-abc-123"
	role := "member"
	username := "john"
	displayName := "John Doe"

	tokenStr, err := middleware.GenerateToken(userID, role, username, displayName, testSecret)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	// Parse the token back
	claims := &middleware.JWTClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(testSecret), nil
	})
	if err != nil {
		t.Fatalf("ParseWithClaims failed: %v", err)
	}
	if !token.Valid {
		t.Fatal("Token is not valid")
	}

	if claims.Sub != userID {
		t.Errorf("Sub = %q, want %q", claims.Sub, userID)
	}
	if claims.Role != role {
		t.Errorf("Role = %q, want %q", claims.Role, role)
	}
	if claims.Username != username {
		t.Errorf("Username = %q, want %q", claims.Username, username)
	}
	if claims.DisplayName != displayName {
		t.Errorf("DisplayName = %q, want %q", claims.DisplayName, displayName)
	}
}

func TestTokenExpiration(t *testing.T) {
	tokenStr, err := middleware.GenerateToken("user-1", "member", "u", "U", testSecret)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	claims := &middleware.JWTClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(testSecret), nil
	})
	if err != nil {
		t.Fatalf("ParseWithClaims failed: %v", err)
	}
	if !token.Valid {
		t.Fatal("Token is not valid")
	}

	expiry := claims.ExpiresAt.Time
	issuedAt := claims.IssuedAt.Time
	duration := expiry.Sub(issuedAt)

	if duration != 24*time.Hour {
		t.Errorf("Token duration = %v, want 24h", duration)
	}
}

func TestTokenSigningMethod(t *testing.T) {
	tokenStr, _ := middleware.GenerateToken("user-1", "member", "u", "U", testSecret)

	token, _ := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return []byte(testSecret), nil
	})

	if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
		t.Errorf("Unexpected signing method: %v", token.Header["alg"])
	}
}

func TestTokenInvalidSecret(t *testing.T) {
	tokenStr, _ := middleware.GenerateToken("user-1", "member", "u", "U", testSecret)

	_, err := jwt.ParseWithClaims(tokenStr, &middleware.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte("wrong-secret"), nil
	})
	if err == nil {
		t.Fatal("Expected error when parsing with wrong secret")
	}
}
