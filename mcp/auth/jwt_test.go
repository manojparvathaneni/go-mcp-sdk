package auth

import (
	"context"
	"testing"
	"time"
)

func TestNewJWTValidator(t *testing.T) {
	config := JWTConfig{
		SigningKey: []byte("test-secret"),
		Issuer:     "test-issuer",
		Audience:   "test-audience",
	}
	
	validator := NewJWTValidator(config)
	
	if validator == nil {
		t.Fatal("Expected NewJWTValidator to return non-nil validator")
	}
	
	if validator.config.Algorithm != "HS256" {
		t.Errorf("Expected default algorithm HS256, got %s", validator.config.Algorithm)
	}
	
	if validator.config.ClockSkew != 5*time.Minute {
		t.Errorf("Expected default clock skew 5m, got %v", validator.config.ClockSkew)
	}
}

func TestJWTValidator_ValidateToken_ValidToken(t *testing.T) {
	signingKey := []byte("test-secret-key")
	issuer := "test-issuer"
	audience := "test-audience"
	
	config := JWTConfig{
		SigningKey: signingKey,
		Issuer:     issuer,
		Audience:   audience,
	}
	
	validator := NewJWTValidator(config)
	
	// Create a valid token
	now := time.Now()
	claims := map[string]interface{}{
		"sub":   "test-user",
		"iss":   issuer,
		"aud":   audience,
		"exp":   now.Add(1 * time.Hour).Unix(),
		"iat":   now.Unix(),
		"email": "test@example.com",
		"roles": []interface{}{"user", "admin"},
		"scope": "read write",
	}
	
	token, err := CreateJWT(signingKey, claims)
	if err != nil {
		t.Fatalf("Failed to create test JWT: %v", err)
	}
	
	// Validate the token
	user, err := validator.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("Token validation failed: %v", err)
	}
	
	// Verify user properties
	if user.ID != "test-user" {
		t.Errorf("Expected user ID 'test-user', got '%s'", user.ID)
	}
	
	if user.Email != "test@example.com" {
		t.Errorf("Expected email 'test@example.com', got '%s'", user.Email)
	}
	
	if !user.HasRole("user") || !user.HasRole("admin") {
		t.Error("Expected user to have 'user' and 'admin' roles")
	}
	
	if !user.HasScope("read") || !user.HasScope("write") {
		t.Error("Expected user to have 'read' and 'write' scopes")
	}
	
	if user.Issuer != issuer {
		t.Errorf("Expected issuer '%s', got '%s'", issuer, user.Issuer)
	}
}

func TestJWTValidator_ValidateToken_ExpiredToken(t *testing.T) {
	signingKey := []byte("test-secret-key")
	
	config := JWTConfig{
		SigningKey: signingKey,
		ClockSkew:  1 * time.Second, // Small clock skew for testing
	}
	
	validator := NewJWTValidator(config)
	
	// Create an expired token
	now := time.Now()
	claims := map[string]interface{}{
		"sub": "test-user",
		"exp": now.Add(-2 * time.Hour).Unix(), // Expired 2 hours ago
		"iat": now.Add(-3 * time.Hour).Unix(),
	}
	
	token, err := CreateJWT(signingKey, claims)
	if err != nil {
		t.Fatalf("Failed to create test JWT: %v", err)
	}
	
	// Validate the token - should fail
	_, err = validator.ValidateToken(context.Background(), token)
	if err == nil {
		t.Error("Expected token validation to fail for expired token")
	}
	
	authErr, ok := err.(*AuthError)
	if !ok {
		t.Errorf("Expected AuthError, got %T", err)
	} else if authErr.Code != ErrCodeExpiredToken {
		t.Errorf("Expected error code %s, got %s", ErrCodeExpiredToken, authErr.Code)
	}
}

func TestJWTValidator_ValidateToken_InvalidIssuer(t *testing.T) {
	signingKey := []byte("test-secret-key")
	
	config := JWTConfig{
		SigningKey: signingKey,
		Issuer:     "expected-issuer",
	}
	
	validator := NewJWTValidator(config)
	
	// Create token with wrong issuer
	now := time.Now()
	claims := map[string]interface{}{
		"sub": "test-user",
		"iss": "wrong-issuer",
		"exp": now.Add(1 * time.Hour).Unix(),
		"iat": now.Unix(),
	}
	
	token, err := CreateJWT(signingKey, claims)
	if err != nil {
		t.Fatalf("Failed to create test JWT: %v", err)
	}
	
	// Validate the token - should fail
	_, err = validator.ValidateToken(context.Background(), token)
	if err == nil {
		t.Error("Expected token validation to fail for invalid issuer")
	}
	
	authErr, ok := err.(*AuthError)
	if !ok {
		t.Errorf("Expected AuthError, got %T", err)
	} else if authErr.Code != ErrCodeInvalidIssuer {
		t.Errorf("Expected error code %s, got %s", ErrCodeInvalidIssuer, authErr.Code)
	}
}

func TestJWTValidator_ValidateToken_InvalidAudience(t *testing.T) {
	signingKey := []byte("test-secret-key")
	
	config := JWTConfig{
		SigningKey: signingKey,
		Audience:   "expected-audience",
	}
	
	validator := NewJWTValidator(config)
	
	// Create token with wrong audience
	now := time.Now()
	claims := map[string]interface{}{
		"sub": "test-user",
		"aud": "wrong-audience",
		"exp": now.Add(1 * time.Hour).Unix(),
		"iat": now.Unix(),
	}
	
	token, err := CreateJWT(signingKey, claims)
	if err != nil {
		t.Fatalf("Failed to create test JWT: %v", err)
	}
	
	// Validate the token - should fail
	_, err = validator.ValidateToken(context.Background(), token)
	if err == nil {
		t.Error("Expected token validation to fail for invalid audience")
	}
	
	authErr, ok := err.(*AuthError)
	if !ok {
		t.Errorf("Expected AuthError, got %T", err)
	} else if authErr.Code != ErrCodeInvalidAudience {
		t.Errorf("Expected error code %s, got %s", ErrCodeInvalidAudience, authErr.Code)
	}
}

func TestJWTValidator_ValidateToken_InvalidSignature(t *testing.T) {
	signingKey := []byte("test-secret-key")
	wrongKey := []byte("wrong-secret-key")
	
	config := JWTConfig{
		SigningKey: signingKey,
	}
	
	validator := NewJWTValidator(config)
	
	// Create token with wrong signing key
	now := time.Now()
	claims := map[string]interface{}{
		"sub": "test-user",
		"exp": now.Add(1 * time.Hour).Unix(),
		"iat": now.Unix(),
	}
	
	token, err := CreateJWT(wrongKey, claims)
	if err != nil {
		t.Fatalf("Failed to create test JWT: %v", err)
	}
	
	// Validate the token - should fail
	_, err = validator.ValidateToken(context.Background(), token)
	if err == nil {
		t.Error("Expected token validation to fail for invalid signature")
	}
	
	authErr, ok := err.(*AuthError)
	if !ok {
		t.Errorf("Expected AuthError, got %T", err)
	} else if authErr.Code != ErrCodeInvalidToken {
		t.Errorf("Expected error code %s, got %s", ErrCodeInvalidToken, authErr.Code)
	}
}

func TestJWTValidator_ValidateToken_MalformedToken(t *testing.T) {
	config := JWTConfig{
		SigningKey: []byte("test-secret-key"),
	}
	
	validator := NewJWTValidator(config)
	
	// Test various malformed tokens
	malformedTokens := []string{
		"",
		"invalid",
		"header.payload", // Missing signature
		"header.payload.signature.extra", // Too many parts
		"invalid.header.signature",
	}
	
	for _, token := range malformedTokens {
		_, err := validator.ValidateToken(context.Background(), token)
		if err == nil {
			t.Errorf("Expected validation to fail for malformed token: %s", token)
		}
		
		authErr, ok := err.(*AuthError)
		if !ok {
			t.Errorf("Expected AuthError for malformed token: %s, got %T", token, err)
		} else if authErr.Code != ErrCodeMalformedToken {
			t.Errorf("Expected error code %s for malformed token: %s, got %s", ErrCodeMalformedToken, token, authErr.Code)
		}
	}
}

func TestJWTValidator_ValidateTokenWithClaims(t *testing.T) {
	signingKey := []byte("test-secret-key")
	
	config := JWTConfig{
		SigningKey:   signingKey,
		CustomClaims: []string{"custom_field", "department"},
	}
	
	validator := NewJWTValidator(config)
	
	// Create token with custom claims
	now := time.Now()
	claims := map[string]interface{}{
		"sub":          "test-user",
		"exp":          now.Add(1 * time.Hour).Unix(),
		"iat":          now.Unix(),
		"custom_field": "custom_value",
		"department":   "engineering",
		"extra_field":  "not_extracted",
	}
	
	token, err := CreateJWT(signingKey, claims)
	if err != nil {
		t.Fatalf("Failed to create test JWT: %v", err)
	}
	
	// Validate token with claims
	user, extractedClaims, err := validator.ValidateTokenWithClaims(context.Background(), token)
	if err != nil {
		t.Fatalf("Token validation failed: %v", err)
	}
	
	// Check custom claims were extracted to user metadata
	if user.Metadata["custom_field"] != "custom_value" {
		t.Error("Expected custom_field to be extracted to user metadata")
	}
	
	if user.Metadata["department"] != "engineering" {
		t.Error("Expected department to be extracted to user metadata")
	}
	
	// Check all claims are returned
	if extractedClaims["sub"] != "test-user" {
		t.Error("Expected all claims to be returned")
	}
	
	if extractedClaims["custom_field"] != "custom_value" {
		t.Error("Expected custom claims to be returned")
	}
}

func TestCreateJWT(t *testing.T) {
	signingKey := []byte("test-secret-key")
	claims := map[string]interface{}{
		"sub": "test-user",
		"iss": "test-issuer",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	}
	
	token, err := CreateJWT(signingKey, claims)
	if err != nil {
		t.Fatalf("Failed to create JWT: %v", err)
	}
	
	if token == "" {
		t.Error("Expected non-empty token")
	}
	
	// Token should have 3 parts separated by dots
	parts := len(token)
	dotCount := 0
	for i := 0; i < parts; i++ {
		if token[i] == '.' {
			dotCount++
		}
	}
	
	if dotCount != 2 {
		t.Errorf("Expected JWT to have 2 dots (3 parts), found %d dots", dotCount)
	}
}