package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewOAuthIntrospector(t *testing.T) {
	config := OAuthConfig{
		IntrospectionURL: "https://example.com/introspect",
		ClientID:         "test-client",
		ClientSecret:     "test-secret",
	}
	
	introspector := NewOAuthIntrospector(config)
	
	if introspector == nil {
		t.Fatal("Expected NewOAuthIntrospector to return non-nil introspector")
	}
	
	if introspector.config.HTTPClient == nil {
		t.Error("Expected default HTTP client to be set")
	}
	
	if introspector.config.CacheTTL != 5*time.Minute {
		t.Errorf("Expected default cache TTL 5m, got %v", introspector.config.CacheTTL)
	}
}

func TestOAuthIntrospector_ValidateToken_Success(t *testing.T) {
	// Create mock introspection server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and content type
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("Expected correct content type")
		}
		
		// Verify basic auth
		username, password, ok := r.BasicAuth()
		if !ok || username != "test-client" || password != "test-secret" {
			t.Error("Expected correct basic auth credentials")
		}
		
		// Return successful introspection response
		response := introspectionResult{
			Active:    true,
			ClientID:  "test-client",
			Username:  "testuser",
			Scope:     "read write",
			TokenType: "Bearer",
			Exp:       time.Now().Add(1 * time.Hour).Unix(),
			Iat:       time.Now().Unix(),
			Sub:       "user123",
			Aud:       "test-audience",
			Iss:       "test-issuer",
			Email:     "test@example.com",
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	
	config := OAuthConfig{
		IntrospectionURL: server.URL,
		ClientID:         "test-client",
		ClientSecret:     "test-secret",
		RequiredScopes:   []string{"read"},
		Issuer:           "test-issuer",
		Audience:         "test-audience",
	}
	
	introspector := NewOAuthIntrospector(config)
	
	user, err := introspector.ValidateToken(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("Token validation failed: %v", err)
	}
	
	// Verify user properties
	if user.ID != "user123" {
		t.Errorf("Expected user ID 'user123', got '%s'", user.ID)
	}
	
	if user.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", user.Username)
	}
	
	if user.Email != "test@example.com" {
		t.Errorf("Expected email 'test@example.com', got '%s'", user.Email)
	}
	
	if !user.HasScope("read") || !user.HasScope("write") {
		t.Error("Expected user to have 'read' and 'write' scopes")
	}
	
	if user.Issuer != "test-issuer" {
		t.Errorf("Expected issuer 'test-issuer', got '%s'", user.Issuer)
	}
}

func TestOAuthIntrospector_ValidateToken_InactiveToken(t *testing.T) {
	// Create mock server that returns inactive token
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := introspectionResult{
			Active: false,
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	
	config := OAuthConfig{
		IntrospectionURL: server.URL,
		ClientID:         "test-client",
		ClientSecret:     "test-secret",
	}
	
	introspector := NewOAuthIntrospector(config)
	
	_, err := introspector.ValidateToken(context.Background(), "inactive-token")
	if err == nil {
		t.Error("Expected validation to fail for inactive token")
	}
	
	authErr, ok := err.(*AuthError)
	if !ok {
		t.Errorf("Expected AuthError, got %T", err)
	} else if authErr.Code != ErrCodeInvalidToken {
		t.Errorf("Expected error code %s, got %s", ErrCodeInvalidToken, authErr.Code)
	}
}

func TestOAuthIntrospector_ValidateToken_ExpiredToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := introspectionResult{
			Active: true,
			Exp:    time.Now().Add(-1 * time.Hour).Unix(), // Expired 1 hour ago
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	
	config := OAuthConfig{
		IntrospectionURL: server.URL,
		ClientID:         "test-client",
		ClientSecret:     "test-secret",
	}
	
	introspector := NewOAuthIntrospector(config)
	
	_, err := introspector.ValidateToken(context.Background(), "expired-token")
	if err == nil {
		t.Error("Expected validation to fail for expired token")
	}
	
	authErr, ok := err.(*AuthError)
	if !ok {
		t.Errorf("Expected AuthError, got %T", err)
	} else if authErr.Code != ErrCodeExpiredToken {
		t.Errorf("Expected error code %s, got %s", ErrCodeExpiredToken, authErr.Code)
	}
}

func TestOAuthIntrospector_ValidateToken_InsufficientScope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := introspectionResult{
			Active: true,
			Scope:  "read", // Missing "write" scope
			Exp:    time.Now().Add(1 * time.Hour).Unix(),
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	
	config := OAuthConfig{
		IntrospectionURL: server.URL,
		ClientID:         "test-client",
		ClientSecret:     "test-secret",
		RequiredScopes:   []string{"read", "write"}, // Require both read and write
	}
	
	introspector := NewOAuthIntrospector(config)
	
	_, err := introspector.ValidateToken(context.Background(), "insufficient-scope-token")
	if err == nil {
		t.Error("Expected validation to fail for insufficient scope")
	}
	
	authErr, ok := err.(*AuthError)
	if !ok {
		t.Errorf("Expected AuthError, got %T", err)
	} else if authErr.Code != ErrCodeInsufficientScope {
		t.Errorf("Expected error code %s, got %s", ErrCodeInsufficientScope, authErr.Code)
	}
}

func TestOAuthIntrospector_ValidateToken_InvalidIssuer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := introspectionResult{
			Active: true,
			Iss:    "wrong-issuer",
			Exp:    time.Now().Add(1 * time.Hour).Unix(),
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	
	config := OAuthConfig{
		IntrospectionURL: server.URL,
		ClientID:         "test-client",
		ClientSecret:     "test-secret",
		Issuer:           "expected-issuer",
	}
	
	introspector := NewOAuthIntrospector(config)
	
	_, err := introspector.ValidateToken(context.Background(), "wrong-issuer-token")
	if err == nil {
		t.Error("Expected validation to fail for invalid issuer")
	}
	
	authErr, ok := err.(*AuthError)
	if !ok {
		t.Errorf("Expected AuthError, got %T", err)
	} else if authErr.Code != ErrCodeInvalidIssuer {
		t.Errorf("Expected error code %s, got %s", ErrCodeInvalidIssuer, authErr.Code)
	}
}

func TestOAuthIntrospector_ValidateToken_InvalidAudience(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := introspectionResult{
			Active: true,
			Aud:    "wrong-audience",
			Exp:    time.Now().Add(1 * time.Hour).Unix(),
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	
	config := OAuthConfig{
		IntrospectionURL: server.URL,
		ClientID:         "test-client",
		ClientSecret:     "test-secret",
		Audience:         "expected-audience",
	}
	
	introspector := NewOAuthIntrospector(config)
	
	_, err := introspector.ValidateToken(context.Background(), "wrong-audience-token")
	if err == nil {
		t.Error("Expected validation to fail for invalid audience")
	}
	
	authErr, ok := err.(*AuthError)
	if !ok {
		t.Errorf("Expected AuthError, got %T", err)
	} else if authErr.Code != ErrCodeInvalidAudience {
		t.Errorf("Expected error code %s, got %s", ErrCodeInvalidAudience, authErr.Code)
	}
}

func TestOAuthIntrospector_Cache(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		
		response := introspectionResult{
			Active: true,
			Sub:    "user123",
			Exp:    time.Now().Add(1 * time.Hour).Unix(),
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	
	config := OAuthConfig{
		IntrospectionURL: server.URL,
		ClientID:         "test-client",
		ClientSecret:     "test-secret",
		CacheTTL:         1 * time.Minute,
	}
	
	introspector := NewOAuthIntrospector(config)
	
	token := "cacheable-token"
	
	// First call should hit the server
	_, err := introspector.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("First validation failed: %v", err)
	}
	
	if callCount != 1 {
		t.Errorf("Expected 1 server call, got %d", callCount)
	}
	
	// Second call should use cache
	_, err = introspector.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("Second validation failed: %v", err)
	}
	
	if callCount != 1 {
		t.Errorf("Expected 1 server call (cached), got %d", callCount)
	}
}

func TestOAuthIntrospector_ValidateTokenWithClaims(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := introspectionResult{
			Active:    true,
			ClientID:  "test-client",
			Username:  "testuser",
			Scope:     "read write",
			TokenType: "Bearer",
			Sub:       "user123",
			Email:     "test@example.com",
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	
	config := OAuthConfig{
		IntrospectionURL: server.URL,
		ClientID:         "test-client",
		ClientSecret:     "test-secret",
	}
	
	introspector := NewOAuthIntrospector(config)
	
	user, claims, err := introspector.ValidateTokenWithClaims(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("Token validation failed: %v", err)
	}
	
	if user.ID != "user123" {
		t.Errorf("Expected user ID 'user123', got '%s'", user.ID)
	}
	
	// Check that claims contain the OAuth introspection data
	if claims["active"] != true {
		t.Error("Expected active claim to be true")
	}
	
	if claims["client_id"] != "test-client" {
		t.Error("Expected client_id claim to match")
	}
	
	if claims["username"] != "testuser" {
		t.Error("Expected username claim to match")
	}
}

func TestOAuthIntrospector_ClearCache(t *testing.T) {
	config := OAuthConfig{
		IntrospectionURL: "https://example.com/introspect",
		ClientID:         "test-client",
		ClientSecret:     "test-secret",
	}
	
	introspector := NewOAuthIntrospector(config)
	
	// Add something to cache
	introspector.cache["test-token"] = &introspectionResult{
		Active:   true,
		CachedAt: time.Now(),
	}
	
	if len(introspector.cache) != 1 {
		t.Error("Expected cache to have 1 entry")
	}
	
	introspector.ClearCache()
	
	if len(introspector.cache) != 0 {
		t.Error("Expected cache to be empty after clear")
	}
}

func TestOAuthIntrospector_CleanupExpiredCache(t *testing.T) {
	config := OAuthConfig{
		IntrospectionURL: "https://example.com/introspect",
		ClientID:         "test-client",
		ClientSecret:     "test-secret",
		CacheTTL:         1 * time.Minute,
	}
	
	introspector := NewOAuthIntrospector(config)
	
	// Add expired and valid entries
	now := time.Now()
	introspector.cache["expired-token"] = &introspectionResult{
		Active:   true,
		CachedAt: now.Add(-2 * time.Minute), // Expired
	}
	introspector.cache["valid-token"] = &introspectionResult{
		Active:   true,
		CachedAt: now, // Valid
	}
	
	if len(introspector.cache) != 2 {
		t.Error("Expected cache to have 2 entries")
	}
	
	introspector.CleanupExpiredCache()
	
	if len(introspector.cache) != 1 {
		t.Error("Expected cache to have 1 entry after cleanup")
	}
	
	if _, exists := introspector.cache["valid-token"]; !exists {
		t.Error("Expected valid token to remain in cache")
	}
}