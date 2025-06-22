package auth

import (
	"context"
	"time"
)

// User represents an authenticated user with permissions and metadata
type User struct {
	ID       string                 `json:"id"`
	Username string                 `json:"username,omitempty"`
	Email    string                 `json:"email,omitempty"`
	Roles    []string               `json:"roles,omitempty"`
	Scopes   []string               `json:"scopes,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	
	// Token information
	TokenID   string    `json:"token_id,omitempty"`
	IssuedAt  time.Time `json:"issued_at,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	Issuer    string    `json:"issuer,omitempty"`
	Audience  []string  `json:"audience,omitempty"`
}

// HasRole checks if the user has a specific role
func (u *User) HasRole(role string) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasScope checks if the user has a specific scope
func (u *User) HasScope(scope string) bool {
	for _, s := range u.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// HasAnyScope checks if the user has any of the specified scopes
func (u *User) HasAnyScope(scopes []string) bool {
	for _, scope := range scopes {
		if u.HasScope(scope) {
			return true
		}
	}
	return false
}

// HasAllScopes checks if the user has all of the specified scopes
func (u *User) HasAllScopes(scopes []string) bool {
	for _, scope := range scopes {
		if !u.HasScope(scope) {
			return false
		}
	}
	return true
}

// IsExpired checks if the user's token has expired
func (u *User) IsExpired() bool {
	if u.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(u.ExpiresAt)
}

// TokenValidator defines the interface for validating authentication tokens
type TokenValidator interface {
	// ValidateToken validates a token and returns the authenticated user
	ValidateToken(ctx context.Context, token string) (*User, error)
	
	// ValidateTokenWithClaims validates a token and returns user with additional claims
	ValidateTokenWithClaims(ctx context.Context, token string) (*User, map[string]interface{}, error)
}

// PermissionChecker defines the interface for checking user permissions
type PermissionChecker interface {
	// CheckPermission checks if a user has permission to perform an action on a resource
	CheckPermission(ctx context.Context, user *User, action, resource string) (bool, error)
	
	// CheckResourceAccess checks if a user can access a specific resource instance
	CheckResourceAccess(ctx context.Context, user *User, resourceType, resourceID string) (bool, error)
	
	// CheckToolAccess checks if a user can execute a specific tool
	CheckToolAccess(ctx context.Context, user *User, toolName string, args map[string]interface{}) (bool, error)
}

// SessionManager defines the interface for secure session management
// Note: MCP servers MUST NOT use sessions for authentication, but can use them for application state
type SessionManager interface {
	// CreateSession creates a new session for the authenticated user
	CreateSession(ctx context.Context, user *User) (*Session, error)
	
	// GetSession retrieves a session by ID
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	
	// UpdateSession updates session data
	UpdateSession(ctx context.Context, session *Session) error
	
	// DeleteSession removes a session
	DeleteSession(ctx context.Context, sessionID string) error
	
	// CleanupExpiredSessions removes expired sessions
	CleanupExpiredSessions(ctx context.Context) error
}

// Session represents a user session for maintaining application state
// Following MCP guidelines: sessions are NOT used for authentication
type Session struct {
	ID        string                 `json:"id"`
	UserID    string                 `json:"user_id"` // Bound to user information per MCP guidelines
	Data      map[string]interface{} `json:"data,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	ExpiresAt time.Time              `json:"expires_at"`
}

// IsExpired checks if the session has expired
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// AuthContext represents authentication context keys
type AuthContext string

const (
	// UserContextKey is the context key for the authenticated user
	UserContextKey AuthContext = "auth:user"
	
	// SessionContextKey is the context key for the session
	SessionContextKey AuthContext = "auth:session"
	
	// TokenContextKey is the context key for the raw token
	TokenContextKey AuthContext = "auth:token"
	
	// PermissionsContextKey is the context key for computed permissions
	PermissionsContextKey AuthContext = "auth:permissions"
)

// GetUserFromContext extracts the authenticated user from context
func GetUserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(UserContextKey).(*User)
	return user, ok
}

// GetSessionFromContext extracts the session from context
func GetSessionFromContext(ctx context.Context) (*Session, bool) {
	session, ok := ctx.Value(SessionContextKey).(*Session)
	return session, ok
}

// GetTokenFromContext extracts the raw token from context
func GetTokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(TokenContextKey).(string)
	return token, ok
}

// WithUser adds a user to the context
func WithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, UserContextKey, user)
}

// WithSession adds a session to the context
func WithSession(ctx context.Context, session *Session) context.Context {
	return context.WithValue(ctx, SessionContextKey, session)
}

// WithToken adds a token to the context
func WithToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, TokenContextKey, token)
}

// AuthError represents authentication/authorization errors
type AuthError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

func (e *AuthError) Error() string {
	if e.Details != "" {
		return e.Message + ": " + e.Details
	}
	return e.Message
}

// Common authentication error codes
const (
	ErrCodeInvalidToken     = "invalid_token"
	ErrCodeExpiredToken     = "expired_token"
	ErrCodeInsufficientScope = "insufficient_scope"
	ErrCodeAccessDenied     = "access_denied"
	ErrCodeInvalidAudience  = "invalid_audience"
	ErrCodeInvalidIssuer    = "invalid_issuer"
	ErrCodeMalformedToken   = "malformed_token"
	ErrCodeTokenNotFound    = "token_not_found"
	ErrCodeSessionExpired   = "session_expired"
	ErrCodePermissionDenied = "permission_denied"
)

// NewAuthError creates a new authentication error
func NewAuthError(code, message, details string) *AuthError {
	return &AuthError{
		Code:    code,
		Message: message,
		Details: details,
	}
}