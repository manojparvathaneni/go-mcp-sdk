package auth

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/manojparvathaneni/go-mcp-sdk/mcp/middleware"
	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

// AuthenticationConfig configures authentication middleware
type AuthenticationConfig struct {
	// TokenValidator for validating authentication tokens
	Validator TokenValidator
	
	// Optional token extraction function (defaults to extractBearerToken)
	TokenExtractor func(ctx context.Context, req *types.JSONRPCMessage) (string, error)
	
	// Skip authentication for specific methods
	SkipMethods []string
	
	// Require authentication for all requests (default: true)
	RequireAuth bool
}

// AuthorizationConfig configures authorization middleware
type AuthorizationConfig struct {
	// PermissionChecker for checking user permissions
	PermissionChecker PermissionChecker
	
	// Required scopes for accessing MCP resources
	RequiredScopes []string
	
	// Required roles for accessing MCP resources
	RequiredRoles []string
	
	// Method-specific permission requirements
	MethodPermissions map[string]MethodPermission
	
	// Skip authorization for specific methods
	SkipMethods []string
}

// MethodPermission defines permission requirements for a specific method
type MethodPermission struct {
	Scopes      []string `json:"scopes,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	Action      string   `json:"action,omitempty"`
	Resource    string   `json:"resource,omitempty"`
	CustomCheck func(ctx context.Context, user *User, req *types.JSONRPCMessage) error
}

// AuthenticationMiddleware creates middleware for token authentication
func AuthenticationMiddleware(config AuthenticationConfig) middleware.Middleware {
	if config.TokenExtractor == nil {
		config.TokenExtractor = extractBearerToken
	}
	
	if config.RequireAuth == false && len(config.SkipMethods) == 0 {
		// If auth not required and no skip methods specified, skip all methods
		config.RequireAuth = false
	} else if len(config.SkipMethods) == 0 {
		// Default: require auth for all methods
		config.RequireAuth = true
	}
	
	return func(next middleware.MiddlewareHandler) middleware.MiddlewareHandler {
		return &authenticationMiddlewareHandler{
			next:   next,
			config: config,
		}
	}
}

type authenticationMiddlewareHandler struct {
	next   middleware.MiddlewareHandler
	config AuthenticationConfig
}

func (h *authenticationMiddlewareHandler) HandleRequest(ctx context.Context, req *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
	// Check if authentication should be skipped for this method
	if h.shouldSkipAuth(req.Method) {
		return h.next.HandleRequest(ctx, req)
	}
	
	// Extract token from request
	token, err := h.config.TokenExtractor(ctx, req)
	if err != nil {
		if h.config.RequireAuth {
			return h.createErrorResponse(req, ErrCodeTokenNotFound, "Authentication required", ""), nil
		}
		// Continue without authentication
		return h.next.HandleRequest(ctx, req)
	}
	
	if token == "" {
		if h.config.RequireAuth {
			return h.createErrorResponse(req, ErrCodeTokenNotFound, "Authentication required", ""), nil
		}
		// Continue without authentication
		return h.next.HandleRequest(ctx, req)
	}
	
	// Validate token
	user, err := h.config.Validator.ValidateToken(ctx, token)
	if err != nil {
		if authErr, ok := err.(*AuthError); ok {
			return h.createErrorResponse(req, authErr.Code, authErr.Message, authErr.Details), nil
		}
		return h.createErrorResponse(req, ErrCodeInvalidToken, "Token validation failed", err.Error()), nil
	}
	
	// Add user and token to context
	ctx = WithUser(ctx, user)
	ctx = WithToken(ctx, token)
	
	return h.next.HandleRequest(ctx, req)
}

func (h *authenticationMiddlewareHandler) HandleNotification(ctx context.Context, notification *types.JSONRPCMessage) error {
	// Check if authentication should be skipped for this method
	if h.shouldSkipAuth(notification.Method) {
		return h.next.HandleNotification(ctx, notification)
	}
	
	// Extract token from notification
	token, err := h.config.TokenExtractor(ctx, notification)
	if err != nil && h.config.RequireAuth {
		return NewAuthError(ErrCodeTokenNotFound, "Failed to extract token", err.Error())
	}
	
	if token == "" && h.config.RequireAuth {
		return NewAuthError(ErrCodeTokenNotFound, "Authentication required", "")
	}
	
	if token != "" {
		// Validate token
		user, err := h.config.Validator.ValidateToken(ctx, token)
		if err != nil {
			return err
		}
		
		// Add user and token to context
		ctx = WithUser(ctx, user)
		ctx = WithToken(ctx, token)
	}
	
	return h.next.HandleNotification(ctx, notification)
}

func (h *authenticationMiddlewareHandler) shouldSkipAuth(method string) bool {
	for _, skipMethod := range h.config.SkipMethods {
		if method == skipMethod {
			return true
		}
	}
	return false
}

func (h *authenticationMiddlewareHandler) createErrorResponse(req *types.JSONRPCMessage, code, message, details string) *types.JSONRPCMessage {
	errorCode := types.InvalidRequest
	if code == ErrCodeExpiredToken || code == ErrCodeInvalidToken {
		errorCode = types.InvalidRequest
	}
	
	errorMessage := message
	if details != "" {
		errorMessage += ": " + details
	}
	
	return &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      req.ID,
		Error: &types.JSONRPCError{
			Code:    errorCode,
			Message: errorMessage,
		},
	}
}

// AuthorizationMiddleware creates middleware for permission-based authorization
func AuthorizationMiddleware(config AuthorizationConfig) middleware.Middleware {
	return func(next middleware.MiddlewareHandler) middleware.MiddlewareHandler {
		return &authorizationMiddlewareHandler{
			next:   next,
			config: config,
		}
	}
}

type authorizationMiddlewareHandler struct {
	next   middleware.MiddlewareHandler
	config AuthorizationConfig
}

func (h *authorizationMiddlewareHandler) HandleRequest(ctx context.Context, req *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
	// Check if authorization should be skipped for this method
	if h.shouldSkipAuthz(req.Method) {
		return h.next.HandleRequest(ctx, req)
	}
	
	// Get user from context (should be set by authentication middleware)
	user, ok := GetUserFromContext(ctx)
	if !ok {
		return h.createErrorResponse(req, ErrCodeAccessDenied, "No authenticated user found", ""), nil
	}
	
	// Check authorization
	if err := h.checkAuthorization(ctx, user, req); err != nil {
		if authErr, ok := err.(*AuthError); ok {
			return h.createErrorResponse(req, authErr.Code, authErr.Message, authErr.Details), nil
		}
		return h.createErrorResponse(req, ErrCodeAccessDenied, "Authorization failed", err.Error()), nil
	}
	
	return h.next.HandleRequest(ctx, req)
}

func (h *authorizationMiddlewareHandler) HandleNotification(ctx context.Context, notification *types.JSONRPCMessage) error {
	// Check if authorization should be skipped for this method
	if h.shouldSkipAuthz(notification.Method) {
		return h.next.HandleNotification(ctx, notification)
	}
	
	// Get user from context
	user, ok := GetUserFromContext(ctx)
	if !ok {
		return NewAuthError(ErrCodeAccessDenied, "No authenticated user found", "")
	}
	
	// Check authorization
	if err := h.checkAuthorization(ctx, user, notification); err != nil {
		return err
	}
	
	return h.next.HandleNotification(ctx, notification)
}

func (h *authorizationMiddlewareHandler) checkAuthorization(ctx context.Context, user *User, req *types.JSONRPCMessage) error {
	// Check method-specific permissions first
	if methodPerm, ok := h.config.MethodPermissions[req.Method]; ok {
		return h.checkMethodPermission(ctx, user, req, methodPerm)
	}
	
	// Check global required scopes
	if len(h.config.RequiredScopes) > 0 {
		if !user.HasAllScopes(h.config.RequiredScopes) {
			missing := []string{}
			for _, scope := range h.config.RequiredScopes {
				if !user.HasScope(scope) {
					missing = append(missing, scope)
				}
			}
			return NewAuthError(ErrCodeInsufficientScope, "Insufficient scope", "missing: "+strings.Join(missing, ", "))
		}
	}
	
	// Check global required roles
	if len(h.config.RequiredRoles) > 0 {
		for _, role := range h.config.RequiredRoles {
			if !user.HasRole(role) {
				return NewAuthError(ErrCodeAccessDenied, "Insufficient role", "missing: "+role)
			}
		}
	}
	
	return nil
}

func (h *authorizationMiddlewareHandler) checkMethodPermission(ctx context.Context, user *User, req *types.JSONRPCMessage, perm MethodPermission) error {
	// Check scopes
	if len(perm.Scopes) > 0 && !user.HasAllScopes(perm.Scopes) {
		return NewAuthError(ErrCodeInsufficientScope, "Insufficient scope for method", req.Method)
	}
	
	// Check roles
	if len(perm.Roles) > 0 {
		hasRole := false
		for _, role := range perm.Roles {
			if user.HasRole(role) {
				hasRole = true
				break
			}
		}
		if !hasRole {
			return NewAuthError(ErrCodeAccessDenied, "Insufficient role for method", req.Method)
		}
	}
	
	// Check resource permission
	if perm.Action != "" && perm.Resource != "" && h.config.PermissionChecker != nil {
		allowed, err := h.config.PermissionChecker.CheckPermission(ctx, user, perm.Action, perm.Resource)
		if err != nil {
			return NewAuthError(ErrCodeAccessDenied, "Permission check failed", err.Error())
		}
		if !allowed {
			return NewAuthError(ErrCodePermissionDenied, "Access denied", "action: "+perm.Action+", resource: "+perm.Resource)
		}
	}
	
	// Run custom check
	if perm.CustomCheck != nil {
		if err := perm.CustomCheck(ctx, user, req); err != nil {
			return err
		}
	}
	
	return nil
}

func (h *authorizationMiddlewareHandler) shouldSkipAuthz(method string) bool {
	for _, skipMethod := range h.config.SkipMethods {
		if method == skipMethod {
			return true
		}
	}
	return false
}

func (h *authorizationMiddlewareHandler) createErrorResponse(req *types.JSONRPCMessage, code, message, details string) *types.JSONRPCMessage {
	errorMessage := message
	if details != "" {
		errorMessage += ": " + details
	}
	
	return &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      req.ID,
		Error: &types.JSONRPCError{
			Code:    types.InvalidRequest,
			Message: errorMessage,
		},
	}
}

// extractBearerToken extracts a Bearer token from the request context or headers
func extractBearerToken(ctx context.Context, req *types.JSONRPCMessage) (string, error) {
	// For JSON-RPC over HTTP, we'd typically extract from HTTP headers
	// Since MCP can run over various transports, we check the context first
	
	// Check if token is already in context (set by transport layer)
	if token, ok := ctx.Value("Authorization").(string); ok {
		if strings.HasPrefix(token, "Bearer ") {
			return strings.TrimPrefix(token, "Bearer "), nil
		}
	}
	
	// Check if there's an authorization field in the request params
	// This is a custom extension for MCP over non-HTTP transports
	if req.Params != nil {
		var params map[string]interface{}
		if err := json.Unmarshal(req.Params, &params); err == nil {
			if auth, ok := params["authorization"].(string); ok {
				if strings.HasPrefix(auth, "Bearer ") {
					return strings.TrimPrefix(auth, "Bearer "), nil
				}
			}
		}
	}
	
	return "", NewAuthError(ErrCodeTokenNotFound, "No bearer token found", "")
}

// SessionMiddleware creates middleware for session management
func SessionMiddleware(sessionManager SessionManager) middleware.Middleware {
	return func(next middleware.MiddlewareHandler) middleware.MiddlewareHandler {
		return &sessionMiddlewareHandler{
			next:           next,
			sessionManager: sessionManager,
		}
	}
}

type sessionMiddlewareHandler struct {
	next           middleware.MiddlewareHandler
	sessionManager SessionManager
}

func (h *sessionMiddlewareHandler) HandleRequest(ctx context.Context, req *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
	// Get user from context (should be set by authentication middleware)
	user, ok := GetUserFromContext(ctx)
	if !ok {
		// No user, continue without session
		return h.next.HandleRequest(ctx, req)
	}
	
	// Try to get existing session or create new one
	session, err := h.getOrCreateSession(ctx, user)
	if err != nil {
		// Log error but continue without session
		// Sessions are for application state, not authentication per MCP spec
	} else {
		ctx = WithSession(ctx, session)
	}
	
	return h.next.HandleRequest(ctx, req)
}

func (h *sessionMiddlewareHandler) HandleNotification(ctx context.Context, notification *types.JSONRPCMessage) error {
	// Get user from context
	user, ok := GetUserFromContext(ctx)
	if !ok {
		return h.next.HandleNotification(ctx, notification)
	}
	
	// Try to get existing session or create new one
	session, err := h.getOrCreateSession(ctx, user)
	if err == nil {
		ctx = WithSession(ctx, session)
	}
	
	return h.next.HandleNotification(ctx, notification)
}

func (h *sessionMiddlewareHandler) getOrCreateSession(ctx context.Context, user *User) (*Session, error) {
	// For MCP, we'll use the user ID as a simple session key
	// In practice, you might extract session ID from request context
	sessionID := "session_" + user.ID
	
	// Try to get existing session
	session, err := h.sessionManager.GetSession(ctx, sessionID)
	if err == nil && !session.IsExpired() {
		return session, nil
	}
	
	// Create new session
	return h.sessionManager.CreateSession(ctx, user)
}