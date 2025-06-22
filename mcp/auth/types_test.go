package auth

import (
	"context"
	"testing"
	"time"
)

func TestUser_HasRole(t *testing.T) {
	user := &User{
		Roles: []string{"admin", "user", "editor"},
	}
	
	if !user.HasRole("admin") {
		t.Error("Expected user to have admin role")
	}
	
	if user.HasRole("nonexistent") {
		t.Error("Expected user not to have nonexistent role")
	}
}

func TestUser_HasScope(t *testing.T) {
	user := &User{
		Scopes: []string{"read", "write", "delete"},
	}
	
	if !user.HasScope("read") {
		t.Error("Expected user to have read scope")
	}
	
	if user.HasScope("admin") {
		t.Error("Expected user not to have admin scope")
	}
}

func TestUser_HasAnyScope(t *testing.T) {
	user := &User{
		Scopes: []string{"read", "write"},
	}
	
	if !user.HasAnyScope([]string{"read", "admin"}) {
		t.Error("Expected user to have at least one scope from the list")
	}
	
	if user.HasAnyScope([]string{"admin", "delete"}) {
		t.Error("Expected user not to have any scope from the list")
	}
}

func TestUser_HasAllScopes(t *testing.T) {
	user := &User{
		Scopes: []string{"read", "write", "delete"},
	}
	
	if !user.HasAllScopes([]string{"read", "write"}) {
		t.Error("Expected user to have all scopes from the list")
	}
	
	if user.HasAllScopes([]string{"read", "admin"}) {
		t.Error("Expected user not to have all scopes from the list")
	}
}

func TestUser_IsExpired(t *testing.T) {
	now := time.Now()
	
	// Expired user
	expiredUser := &User{
		ExpiresAt: now.Add(-1 * time.Hour),
	}
	
	if !expiredUser.IsExpired() {
		t.Error("Expected user to be expired")
	}
	
	// Valid user
	validUser := &User{
		ExpiresAt: now.Add(1 * time.Hour),
	}
	
	if validUser.IsExpired() {
		t.Error("Expected user not to be expired")
	}
	
	// User with no expiration
	noExpirationUser := &User{}
	
	if noExpirationUser.IsExpired() {
		t.Error("Expected user with no expiration not to be expired")
	}
}

func TestSession_IsExpired(t *testing.T) {
	now := time.Now()
	
	// Expired session
	expiredSession := &Session{
		ExpiresAt: now.Add(-1 * time.Hour),
	}
	
	if !expiredSession.IsExpired() {
		t.Error("Expected session to be expired")
	}
	
	// Valid session
	validSession := &Session{
		ExpiresAt: now.Add(1 * time.Hour),
	}
	
	if validSession.IsExpired() {
		t.Error("Expected session not to be expired")
	}
}

func TestGetUserFromContext(t *testing.T) {
	user := &User{ID: "test-user"}
	ctx := WithUser(context.Background(), user)
	
	retrievedUser, ok := GetUserFromContext(ctx)
	if !ok {
		t.Fatal("Expected to retrieve user from context")
	}
	
	if retrievedUser.ID != user.ID {
		t.Errorf("Expected user ID %s, got %s", user.ID, retrievedUser.ID)
	}
	
	// Test with empty context
	_, ok = GetUserFromContext(context.Background())
	if ok {
		t.Error("Expected not to retrieve user from empty context")
	}
}

func TestGetSessionFromContext(t *testing.T) {
	session := &Session{ID: "test-session"}
	ctx := WithSession(context.Background(), session)
	
	retrievedSession, ok := GetSessionFromContext(ctx)
	if !ok {
		t.Fatal("Expected to retrieve session from context")
	}
	
	if retrievedSession.ID != session.ID {
		t.Errorf("Expected session ID %s, got %s", session.ID, retrievedSession.ID)
	}
}

func TestGetTokenFromContext(t *testing.T) {
	token := "test-token"
	ctx := WithToken(context.Background(), token)
	
	retrievedToken, ok := GetTokenFromContext(ctx)
	if !ok {
		t.Fatal("Expected to retrieve token from context")
	}
	
	if retrievedToken != token {
		t.Errorf("Expected token %s, got %s", token, retrievedToken)
	}
}

func TestAuthError_Error(t *testing.T) {
	// Error without details
	err1 := NewAuthError("test_code", "Test message", "")
	expected1 := "Test message"
	if err1.Error() != expected1 {
		t.Errorf("Expected error message '%s', got '%s'", expected1, err1.Error())
	}
	
	// Error with details
	err2 := NewAuthError("test_code", "Test message", "additional details")
	expected2 := "Test message: additional details"
	if err2.Error() != expected2 {
		t.Errorf("Expected error message '%s', got '%s'", expected2, err2.Error())
	}
}

func TestNewAuthError(t *testing.T) {
	code := "test_code"
	message := "Test message"
	details := "Test details"
	
	err := NewAuthError(code, message, details)
	
	if err.Code != code {
		t.Errorf("Expected code %s, got %s", code, err.Code)
	}
	
	if err.Message != message {
		t.Errorf("Expected message %s, got %s", message, err.Message)
	}
	
	if err.Details != details {
		t.Errorf("Expected details %s, got %s", details, err.Details)
	}
}