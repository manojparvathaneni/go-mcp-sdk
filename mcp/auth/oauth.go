package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OAuthConfig contains configuration for OAuth 2.0 token introspection
type OAuthConfig struct {
	// Introspection endpoint URL
	IntrospectionURL string
	
	// Client credentials for introspection
	ClientID     string
	ClientSecret string
	
	// HTTP client for making requests
	HTTPClient *http.Client
	
	// Cache TTL for introspection results
	CacheTTL time.Duration
	
	// Required scopes for MCP access
	RequiredScopes []string
	
	// Issuer validation
	Issuer string
	
	// Audience validation 
	Audience string
}

// OAuthIntrospector validates OAuth 2.0 tokens via introspection
type OAuthIntrospector struct {
	config OAuthConfig
	cache  map[string]*introspectionResult
}

// introspectionResult represents the result of token introspection
type introspectionResult struct {
	Active     bool      `json:"active"`
	ClientID   string    `json:"client_id,omitempty"`
	Username   string    `json:"username,omitempty"`
	Scope      string    `json:"scope,omitempty"`
	TokenType  string    `json:"token_type,omitempty"`
	Exp        int64     `json:"exp,omitempty"`
	Iat        int64     `json:"iat,omitempty"`
	Nbf        int64     `json:"nbf,omitempty"`
	Sub        string    `json:"sub,omitempty"`
	Aud        string    `json:"aud,omitempty"`
	Iss        string    `json:"iss,omitempty"`
	Jti        string    `json:"jti,omitempty"`
	Email      string    `json:"email,omitempty"`
	
	// Cache metadata
	CachedAt time.Time `json:"-"`
}

// NewOAuthIntrospector creates a new OAuth token introspector
func NewOAuthIntrospector(config OAuthConfig) *OAuthIntrospector {
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{
			Timeout: 10 * time.Second,
		}
	}
	
	if config.CacheTTL == 0 {
		config.CacheTTL = 5 * time.Minute
	}
	
	return &OAuthIntrospector{
		config: config,
		cache:  make(map[string]*introspectionResult),
	}
}

// ValidateToken validates an OAuth 2.0 token via introspection
func (o *OAuthIntrospector) ValidateToken(ctx context.Context, token string) (*User, error) {
	user, _, err := o.ValidateTokenWithClaims(ctx, token)
	return user, err
}

// ValidateTokenWithClaims validates an OAuth 2.0 token and returns additional claims
func (o *OAuthIntrospector) ValidateTokenWithClaims(ctx context.Context, token string) (*User, map[string]interface{}, error) {
	// Check cache first
	if result, ok := o.getCachedResult(token); ok {
		if time.Since(result.CachedAt) < o.config.CacheTTL {
			return o.introspectionToUser(result), o.introspectionToClaims(result), nil
		}
		// Remove expired cache entry
		delete(o.cache, token)
	}
	
	// Perform introspection
	result, err := o.introspectToken(ctx, token)
	if err != nil {
		return nil, nil, NewAuthError(ErrCodeInvalidToken, "Token introspection failed", err.Error())
	}
	
	// Cache the result
	result.CachedAt = time.Now()
	o.cache[token] = result
	
	// Validate the result
	if err := o.validateIntrospectionResult(result); err != nil {
		return nil, nil, err
	}
	
	user := o.introspectionToUser(result)
	claims := o.introspectionToClaims(result)
	
	return user, claims, nil
}

// getCachedResult retrieves a cached introspection result
func (o *OAuthIntrospector) getCachedResult(token string) (*introspectionResult, bool) {
	result, ok := o.cache[token]
	return result, ok
}

// introspectToken performs OAuth 2.0 token introspection
func (o *OAuthIntrospector) introspectToken(ctx context.Context, token string) (*introspectionResult, error) {
	// Prepare introspection request
	data := url.Values{}
	data.Set("token", token)
	data.Set("token_type_hint", "access_token")
	
	req, err := http.NewRequestWithContext(ctx, "POST", o.config.IntrospectionURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create introspection request: %w", err)
	}
	
	// Set authentication and headers
	req.SetBasicAuth(o.config.ClientID, o.config.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	
	// Make the request
	resp, err := o.config.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("introspection request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("introspection failed with status: %d", resp.StatusCode)
	}
	
	// Parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read introspection response: %w", err)
	}
	
	var result introspectionResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse introspection response: %w", err)
	}
	
	return &result, nil
}

// validateIntrospectionResult validates the introspection response
func (o *OAuthIntrospector) validateIntrospectionResult(result *introspectionResult) error {
	// Check if token is active
	if !result.Active {
		return NewAuthError(ErrCodeInvalidToken, "Token is not active", "")
	}
	
	// Check expiration
	if result.Exp > 0 {
		expTime := time.Unix(result.Exp, 0)
		if time.Now().After(expTime) {
			return NewAuthError(ErrCodeExpiredToken, "Token has expired", "")
		}
	}
	
	// Check not before
	if result.Nbf > 0 {
		nbfTime := time.Unix(result.Nbf, 0)
		if time.Now().Before(nbfTime) {
			return NewAuthError(ErrCodeInvalidToken, "Token not yet valid", "")
		}
	}
	
	// Validate issuer if configured
	if o.config.Issuer != "" && result.Iss != o.config.Issuer {
		return NewAuthError(ErrCodeInvalidIssuer, "Invalid issuer", fmt.Sprintf("expected: %s, got: %s", o.config.Issuer, result.Iss))
	}
	
	// Validate audience if configured
	if o.config.Audience != "" && result.Aud != o.config.Audience {
		return NewAuthError(ErrCodeInvalidAudience, "Invalid audience", fmt.Sprintf("expected: %s, got: %s", o.config.Audience, result.Aud))
	}
	
	// Check required scopes
	if len(o.config.RequiredScopes) > 0 {
		tokenScopes := strings.Fields(result.Scope)
		for _, requiredScope := range o.config.RequiredScopes {
			found := false
			for _, tokenScope := range tokenScopes {
				if tokenScope == requiredScope {
					found = true
					break
				}
			}
			if !found {
				return NewAuthError(ErrCodeInsufficientScope, "Insufficient scope", fmt.Sprintf("missing required scope: %s", requiredScope))
			}
		}
	}
	
	return nil
}

// introspectionToUser converts introspection result to User
func (o *OAuthIntrospector) introspectionToUser(result *introspectionResult) *User {
	user := &User{
		ID:        result.Sub,
		Username:  result.Username,
		Email:     result.Email,
		TokenID:   result.Jti,
		Issuer:    result.Iss,
		Metadata:  make(map[string]interface{}),
	}
	
	// Set time fields
	if result.Iat > 0 {
		user.IssuedAt = time.Unix(result.Iat, 0)
	}
	if result.Exp > 0 {
		user.ExpiresAt = time.Unix(result.Exp, 0)
	}
	
	// Set audience
	if result.Aud != "" {
		user.Audience = []string{result.Aud}
	}
	
	// Set scopes
	if result.Scope != "" {
		user.Scopes = strings.Fields(result.Scope)
	}
	
	// Add OAuth-specific metadata
	user.Metadata["client_id"] = result.ClientID
	user.Metadata["token_type"] = result.TokenType
	
	return user
}

// introspectionToClaims converts introspection result to claims map
func (o *OAuthIntrospector) introspectionToClaims(result *introspectionResult) map[string]interface{} {
	claims := make(map[string]interface{})
	
	if result.Active {
		claims["active"] = result.Active
	}
	if result.ClientID != "" {
		claims["client_id"] = result.ClientID
	}
	if result.Username != "" {
		claims["username"] = result.Username
	}
	if result.Scope != "" {
		claims["scope"] = result.Scope
	}
	if result.TokenType != "" {
		claims["token_type"] = result.TokenType
	}
	if result.Exp > 0 {
		claims["exp"] = result.Exp
	}
	if result.Iat > 0 {
		claims["iat"] = result.Iat
	}
	if result.Nbf > 0 {
		claims["nbf"] = result.Nbf
	}
	if result.Sub != "" {
		claims["sub"] = result.Sub
	}
	if result.Aud != "" {
		claims["aud"] = result.Aud
	}
	if result.Iss != "" {
		claims["iss"] = result.Iss
	}
	if result.Jti != "" {
		claims["jti"] = result.Jti
	}
	if result.Email != "" {
		claims["email"] = result.Email
	}
	
	return claims
}

// ClearCache clears the introspection cache
func (o *OAuthIntrospector) ClearCache() {
	o.cache = make(map[string]*introspectionResult)
}

// CleanupExpiredCache removes expired entries from the cache
func (o *OAuthIntrospector) CleanupExpiredCache() {
	now := time.Now()
	for token, result := range o.cache {
		if now.Sub(result.CachedAt) > o.config.CacheTTL {
			delete(o.cache, token)
		}
	}
}