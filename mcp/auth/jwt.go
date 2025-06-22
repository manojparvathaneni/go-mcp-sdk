package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash"
	"strings"
	"time"
)

// JWTConfig contains configuration for JWT validation
type JWTConfig struct {
	// SigningKey for HMAC algorithms (HS256, HS384, HS512)
	SigningKey []byte
	
	// RSAPublicKey for RSA algorithms (RS256, RS384, RS512)
	RSAPublicKey *rsa.PublicKey
	
	// Required issuer claim
	Issuer string
	
	// Required audience claim
	Audience string
	
	// Clock skew tolerance for exp/nbf claims
	ClockSkew time.Duration
	
	// Algorithm to expect (default: HS256)
	Algorithm string
	
	// Custom claims to extract into user metadata
	CustomClaims []string
}

// JWTValidator validates JWT tokens according to MCP security requirements
type JWTValidator struct {
	config JWTConfig
}

// NewJWTValidator creates a new JWT validator with the given configuration
func NewJWTValidator(config JWTConfig) *JWTValidator {
	if config.Algorithm == "" {
		config.Algorithm = "HS256"
	}
	if config.ClockSkew == 0 {
		config.ClockSkew = 5 * time.Minute
	}
	
	return &JWTValidator{
		config: config,
	}
}

// ValidateToken validates a JWT token and returns the authenticated user
func (v *JWTValidator) ValidateToken(ctx context.Context, token string) (*User, error) {
	user, _, err := v.ValidateTokenWithClaims(ctx, token)
	return user, err
}

// ValidateTokenWithClaims validates a JWT token and returns user with additional claims
func (v *JWTValidator) ValidateTokenWithClaims(ctx context.Context, token string) (*User, map[string]interface{}, error) {
	// Parse the token
	header, payload, signature, err := v.parseJWT(token)
	if err != nil {
		return nil, nil, NewAuthError(ErrCodeMalformedToken, "Failed to parse JWT", err.Error())
	}
	
	// Verify the signature
	if err := v.verifySignature(header, payload, signature); err != nil {
		return nil, nil, NewAuthError(ErrCodeInvalidToken, "Invalid token signature", err.Error())
	}
	
	// Parse claims
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, nil, NewAuthError(ErrCodeMalformedToken, "Failed to parse claims", err.Error())
	}
	
	// Validate standard claims
	if err := v.validateStandardClaims(claims); err != nil {
		return nil, nil, err
	}
	
	// Extract user information
	user := v.extractUser(claims)
	
	return user, claims, nil
}

// parseJWT splits a JWT token into its components
func (v *JWTValidator) parseJWT(token string) (header, payload, signature []byte, err error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, nil, nil, fmt.Errorf("invalid JWT format")
	}
	
	header, err = base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to decode header: %w", err)
	}
	
	payload, err = base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to decode payload: %w", err)
	}
	
	signature, err = base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to decode signature: %w", err)
	}
	
	return header, payload, signature, nil
}

// verifySignature verifies the JWT signature
func (v *JWTValidator) verifySignature(header, payload, signature []byte) error {
	// Parse header to get algorithm
	var headerClaims map[string]interface{}
	if err := json.Unmarshal(header, &headerClaims); err != nil {
		return fmt.Errorf("failed to parse header: %w", err)
	}
	
	alg, ok := headerClaims["alg"].(string)
	if !ok {
		return fmt.Errorf("missing or invalid algorithm in header")
	}
	
	if alg != v.config.Algorithm {
		return fmt.Errorf("unexpected algorithm: %s, expected: %s", alg, v.config.Algorithm)
	}
	
	// Create signing input
	headerB64 := base64.RawURLEncoding.EncodeToString(header)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	signingInput := headerB64 + "." + payloadB64
	
	switch alg {
	case "HS256":
		return v.verifyHMAC(signingInput, signature, sha256.New)
	case "none":
		return fmt.Errorf("unsigned tokens not allowed")
	default:
		return fmt.Errorf("unsupported algorithm: %s", alg)
	}
}

// verifyHMAC verifies HMAC signature
func (v *JWTValidator) verifyHMAC(signingInput string, signature []byte, hashFunc func() hash.Hash) error {
	if len(v.config.SigningKey) == 0 {
		return fmt.Errorf("signing key not configured for HMAC")
	}
	
	mac := hmac.New(hashFunc, v.config.SigningKey)
	mac.Write([]byte(signingInput))
	expectedSignature := mac.Sum(nil)
	
	if !hmac.Equal(signature, expectedSignature) {
		return fmt.Errorf("signature verification failed")
	}
	
	return nil
}

// validateStandardClaims validates standard JWT claims according to MCP requirements
func (v *JWTValidator) validateStandardClaims(claims map[string]interface{}) error {
	now := time.Now()
	
	// Validate expiration (exp)
	if exp, ok := claims["exp"]; ok {
		var expTime time.Time
		switch e := exp.(type) {
		case float64:
			expTime = time.Unix(int64(e), 0)
		case int64:
			expTime = time.Unix(e, 0)
		default:
			return NewAuthError(ErrCodeMalformedToken, "Invalid exp claim format", "")
		}
		
		if now.After(expTime.Add(v.config.ClockSkew)) {
			return NewAuthError(ErrCodeExpiredToken, "Token has expired", "")
		}
	}
	
	// Validate not before (nbf)
	if nbf, ok := claims["nbf"]; ok {
		var nbfTime time.Time
		switch n := nbf.(type) {
		case float64:
			nbfTime = time.Unix(int64(n), 0)
		case int64:
			nbfTime = time.Unix(n, 0)
		default:
			return NewAuthError(ErrCodeMalformedToken, "Invalid nbf claim format", "")
		}
		
		if now.Before(nbfTime.Add(-v.config.ClockSkew)) {
			return NewAuthError(ErrCodeInvalidToken, "Token not yet valid", "")
		}
	}
	
	// Validate issuer (iss) - MCP requirement
	if v.config.Issuer != "" {
		iss, ok := claims["iss"].(string)
		if !ok || iss != v.config.Issuer {
			return NewAuthError(ErrCodeInvalidIssuer, "Invalid or missing issuer", fmt.Sprintf("expected: %s", v.config.Issuer))
		}
	}
	
	// Validate audience (aud) - MCP requirement  
	if v.config.Audience != "" {
		aud, ok := claims["aud"]
		if !ok {
			return NewAuthError(ErrCodeInvalidAudience, "Missing audience claim", "")
		}
		
		var audiences []string
		switch a := aud.(type) {
		case string:
			audiences = []string{a}
		case []interface{}:
			for _, audience := range a {
				if audStr, ok := audience.(string); ok {
					audiences = append(audiences, audStr)
				}
			}
		default:
			return NewAuthError(ErrCodeInvalidAudience, "Invalid audience claim format", "")
		}
		
		validAudience := false
		for _, audience := range audiences {
			if audience == v.config.Audience {
				validAudience = true
				break
			}
		}
		
		if !validAudience {
			return NewAuthError(ErrCodeInvalidAudience, "Token not intended for this audience", fmt.Sprintf("expected: %s", v.config.Audience))
		}
	}
	
	return nil
}

// extractUser creates a User from JWT claims
func (v *JWTValidator) extractUser(claims map[string]interface{}) *User {
	user := &User{
		Metadata: make(map[string]interface{}),
	}
	
	// Standard claims
	if sub, ok := claims["sub"].(string); ok {
		user.ID = sub
	}
	
	if iss, ok := claims["iss"].(string); ok {
		user.Issuer = iss
	}
	
	if jti, ok := claims["jti"].(string); ok {
		user.TokenID = jti
	}
	
	// Time claims
	if iat, ok := claims["iat"]; ok {
		switch i := iat.(type) {
		case float64:
			user.IssuedAt = time.Unix(int64(i), 0)
		case int64:
			user.IssuedAt = time.Unix(i, 0)
		}
	}
	
	if exp, ok := claims["exp"]; ok {
		switch e := exp.(type) {
		case float64:
			user.ExpiresAt = time.Unix(int64(e), 0)
		case int64:
			user.ExpiresAt = time.Unix(e, 0)
		}
	}
	
	// Audience
	if aud, ok := claims["aud"]; ok {
		switch a := aud.(type) {
		case string:
			user.Audience = []string{a}
		case []interface{}:
			for _, audience := range a {
				if audStr, ok := audience.(string); ok {
					user.Audience = append(user.Audience, audStr)
				}
			}
		}
	}
	
	// Common custom claims
	if email, ok := claims["email"].(string); ok {
		user.Email = email
	}
	
	if username, ok := claims["username"].(string); ok {
		user.Username = username
	}
	
	if preferred_username, ok := claims["preferred_username"].(string); ok && user.Username == "" {
		user.Username = preferred_username
	}
	
	// Roles and scopes
	if roles, ok := claims["roles"]; ok {
		if roleSlice, ok := roles.([]interface{}); ok {
			for _, role := range roleSlice {
				if roleStr, ok := role.(string); ok {
					user.Roles = append(user.Roles, roleStr)
				}
			}
		}
	}
	
	if scope, ok := claims["scope"].(string); ok {
		user.Scopes = strings.Fields(scope)
	} else if scopes, ok := claims["scopes"]; ok {
		if scopeSlice, ok := scopes.([]interface{}); ok {
			for _, scope := range scopeSlice {
				if scopeStr, ok := scope.(string); ok {
					user.Scopes = append(user.Scopes, scopeStr)
				}
			}
		}
	}
	
	// Custom claims
	for _, claim := range v.config.CustomClaims {
		if value, ok := claims[claim]; ok {
			user.Metadata[claim] = value
		}
	}
	
	return user
}

// CreateJWT creates a JWT token with the given claims (utility function for testing)
func CreateJWT(signingKey []byte, claims map[string]interface{}) (string, error) {
	// Create header
	header := map[string]interface{}{
		"alg": "HS256",
		"typ": "JWT",
	}
	
	headerBytes, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	
	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	
	// Encode header and payload
	headerB64 := base64.RawURLEncoding.EncodeToString(headerBytes)
	payloadB64 := base64.RawURLEncoding.EncodeToString(claimsBytes)
	
	// Create signature
	signingInput := headerB64 + "." + payloadB64
	mac := hmac.New(sha256.New, signingKey)
	mac.Write([]byte(signingInput))
	signature := mac.Sum(nil)
	signatureB64 := base64.RawURLEncoding.EncodeToString(signature)
	
	return signingInput + "." + signatureB64, nil
}