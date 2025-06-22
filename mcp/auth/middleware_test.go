package auth

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)

// Mock validator for testing
type mockTokenValidator struct {
	validateFunc func(ctx context.Context, token string) (*User, error)
}

func (m *mockTokenValidator) ValidateToken(ctx context.Context, token string) (*User, error) {
	if m.validateFunc != nil {
		return m.validateFunc(ctx, token)
	}
	return &User{ID: "test-user"}, nil
}

func (m *mockTokenValidator) ValidateTokenWithClaims(ctx context.Context, token string) (*User, map[string]interface{}, error) {
	user, err := m.ValidateToken(ctx, token)
	return user, make(map[string]interface{}), err
}

// Mock permission checker for testing
type mockPermissionChecker struct {
	checkPermissionFunc func(ctx context.Context, user *User, action, resource string) (bool, error)
}

func (m *mockPermissionChecker) CheckPermission(ctx context.Context, user *User, action, resource string) (bool, error) {
	if m.checkPermissionFunc != nil {
		return m.checkPermissionFunc(ctx, user, action, resource)
	}
	return true, nil
}

func (m *mockPermissionChecker) CheckResourceAccess(ctx context.Context, user *User, resourceType, resourceID string) (bool, error) {
	return true, nil
}

func (m *mockPermissionChecker) CheckToolAccess(ctx context.Context, user *User, toolName string, args map[string]interface{}) (bool, error) {
	return true, nil
}

// Mock middleware handler for testing
type mockHandler struct {
	handleRequestFunc     func(ctx context.Context, req *types.JSONRPCMessage) (*types.JSONRPCMessage, error)
	handleNotificationFunc func(ctx context.Context, notification *types.JSONRPCMessage) error
}

func (m *mockHandler) HandleRequest(ctx context.Context, req *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
	if m.handleRequestFunc != nil {
		return m.handleRequestFunc(ctx, req)
	}
	return &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  json.RawMessage(`{"success": true}`),
	}, nil
}

func (m *mockHandler) HandleNotification(ctx context.Context, notification *types.JSONRPCMessage) error {
	if m.handleNotificationFunc != nil {
		return m.handleNotificationFunc(ctx, notification)
	}
	return nil
}

func TestAuthenticationMiddleware_ValidToken(t *testing.T) {
	validator := &mockTokenValidator{
		validateFunc: func(ctx context.Context, token string) (*User, error) {
			return &User{
				ID:       "test-user",
				Username: "testuser",
				Roles:    []string{"user"},
			}, nil
		},
	}
	
	config := AuthenticationConfig{
		Validator:   validator,
		RequireAuth: true,
	}
	
	authMiddleware := AuthenticationMiddleware(config)
	
	var capturedUser *User
	mockNext := &mockHandler{
		handleRequestFunc: func(ctx context.Context, req *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
			user, ok := GetUserFromContext(ctx)
			if ok {
				capturedUser = user
			}
			return &types.JSONRPCMessage{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"success": true}`),
			}, nil
		},
	}
	
	handler := authMiddleware(mockNext)
	
	// Create request with token in context
	ctx := context.WithValue(context.Background(), "Authorization", "Bearer valid-token")
	id := json.RawMessage(`"test-id"`)
	req := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "test/method",
	}
	
	resp, err := handler.HandleRequest(ctx, req)
	if err != nil {
		t.Fatalf("Authentication failed: %v", err)
	}
	
	if resp.Error != nil {
		t.Errorf("Expected successful response, got error: %v", resp.Error)
	}
	
	if capturedUser == nil {
		t.Error("Expected user to be added to context")
	} else if capturedUser.ID != "test-user" {
		t.Errorf("Expected user ID 'test-user', got '%s'", capturedUser.ID)
	}
}

func TestAuthenticationMiddleware_InvalidToken(t *testing.T) {
	validator := &mockTokenValidator{
		validateFunc: func(ctx context.Context, token string) (*User, error) {
			return nil, NewAuthError(ErrCodeInvalidToken, "Invalid token", "")
		},
	}
	
	config := AuthenticationConfig{
		Validator:   validator,
		RequireAuth: true,
	}
	
	authMiddleware := AuthenticationMiddleware(config)
	handler := authMiddleware(&mockHandler{})
	
	// Create request with invalid token
	ctx := context.WithValue(context.Background(), "Authorization", "Bearer invalid-token")
	id := json.RawMessage(`"test-id"`)
	req := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "test/method",
	}
	
	resp, err := handler.HandleRequest(ctx, req)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}
	
	if resp.Error == nil {
		t.Error("Expected error response for invalid token")
	}
	
	if resp.Error.Message != "Invalid token" {
		t.Errorf("Expected error message 'Invalid token', got '%s'", resp.Error.Message)
	}
}

func TestAuthenticationMiddleware_MissingToken(t *testing.T) {
	validator := &mockTokenValidator{}
	
	config := AuthenticationConfig{
		Validator:   validator,
		RequireAuth: true,
	}
	
	authMiddleware := AuthenticationMiddleware(config)
	handler := authMiddleware(&mockHandler{})
	
	// Create request without token
	id := json.RawMessage(`"test-id"`)
	req := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "test/method",
	}
	
	resp, err := handler.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}
	
	if resp.Error == nil {
		t.Error("Expected error response for missing token")
	}
	
	if resp.Error.Message != "Authentication required" {
		t.Errorf("Expected error message 'Authentication required', got '%s'", resp.Error.Message)
	}
}

func TestAuthenticationMiddleware_SkipMethods(t *testing.T) {
	validator := &mockTokenValidator{}
	
	config := AuthenticationConfig{
		Validator:   validator,
		RequireAuth: true,
		SkipMethods: []string{"ping", "health"},
	}
	
	authMiddleware := AuthenticationMiddleware(config)
	handler := authMiddleware(&mockHandler{})
	
	// Test skipped method
	id := json.RawMessage(`"test-id"`)
	req := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "ping",
	}
	
	resp, err := handler.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}
	
	if resp.Error != nil {
		t.Error("Expected successful response for skipped method")
	}
}

func TestAuthenticationMiddleware_OptionalAuth(t *testing.T) {
	validator := &mockTokenValidator{}
	
	config := AuthenticationConfig{
		Validator:   validator,
		RequireAuth: false, // Optional authentication
	}
	
	authMiddleware := AuthenticationMiddleware(config)
	handler := authMiddleware(&mockHandler{})
	
	// Request without token should succeed
	id := json.RawMessage(`"test-id"`)
	req := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "test/method",
	}
	
	resp, err := handler.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}
	
	if resp.Error != nil {
		t.Error("Expected successful response for optional auth")
	}
}

func TestAuthenticationMiddleware_TokenInParams(t *testing.T) {
	validator := &mockTokenValidator{
		validateFunc: func(ctx context.Context, token string) (*User, error) {
			if token == "param-token" {
				return &User{ID: "param-user"}, nil
			}
			return nil, NewAuthError(ErrCodeInvalidToken, "Invalid token", "")
		},
	}
	
	config := AuthenticationConfig{
		Validator:   validator,
		RequireAuth: true,
	}
	
	authMiddleware := AuthenticationMiddleware(config)
	
	var capturedUser *User
	mockNext := &mockHandler{
		handleRequestFunc: func(ctx context.Context, req *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
			user, ok := GetUserFromContext(ctx)
			if ok {
				capturedUser = user
			}
			return &types.JSONRPCMessage{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"success": true}`),
			}, nil
		},
	}
	
	handler := authMiddleware(mockNext)
	
	// Create request with token in params
	params := map[string]interface{}{
		"authorization": "Bearer param-token",
		"other_param":   "value",
	}
	paramsJSON, _ := json.Marshal(params)
	
	id := json.RawMessage(`"test-id"`)
	req := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "test/method",
		Params:  paramsJSON,
	}
	
	resp, err := handler.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Authentication failed: %v", err)
	}
	
	if resp.Error != nil {
		t.Errorf("Expected successful response, got error: %v", resp.Error)
	}
	
	if capturedUser == nil {
		t.Error("Expected user to be added to context")
	} else if capturedUser.ID != "param-user" {
		t.Errorf("Expected user ID 'param-user', got '%s'", capturedUser.ID)
	}
}

func TestAuthorizationMiddleware_ValidPermissions(t *testing.T) {
	permChecker := &mockPermissionChecker{
		checkPermissionFunc: func(ctx context.Context, user *User, action, resource string) (bool, error) {
			return user.HasRole("admin"), nil
		},
	}
	
	config := AuthorizationConfig{
		PermissionChecker: permChecker,
		RequiredRoles:     []string{"admin"},
	}
	
	authzMiddleware := AuthorizationMiddleware(config)
	handler := authzMiddleware(&mockHandler{})
	
	// Create context with admin user
	user := &User{
		ID:    "admin-user",
		Roles: []string{"admin"},
	}
	ctx := WithUser(context.Background(), user)
	
	id := json.RawMessage(`"test-id"`)
	req := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "admin/action",
	}
	
	resp, err := handler.HandleRequest(ctx, req)
	if err != nil {
		t.Fatalf("Authorization failed: %v", err)
	}
	
	if resp.Error != nil {
		t.Error("Expected successful response for authorized user")
	}
}

func TestAuthorizationMiddleware_InsufficientPermissions(t *testing.T) {
	permChecker := &mockPermissionChecker{}
	
	config := AuthorizationConfig{
		PermissionChecker: permChecker,
		RequiredRoles:     []string{"admin"},
	}
	
	authzMiddleware := AuthorizationMiddleware(config)
	handler := authzMiddleware(&mockHandler{})
	
	// Create context with regular user
	user := &User{
		ID:    "regular-user",
		Roles: []string{"user"},
	}
	ctx := WithUser(context.Background(), user)
	
	id := json.RawMessage(`"test-id"`)
	req := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "admin/action",
	}
	
	resp, err := handler.HandleRequest(ctx, req)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}
	
	if resp.Error == nil {
		t.Error("Expected error response for unauthorized user")
	}
}

func TestAuthorizationMiddleware_NoUser(t *testing.T) {
	permChecker := &mockPermissionChecker{}
	
	config := AuthorizationConfig{
		PermissionChecker: permChecker,
		RequiredRoles:     []string{"admin"},
	}
	
	authzMiddleware := AuthorizationMiddleware(config)
	handler := authzMiddleware(&mockHandler{})
	
	// Request without user in context
	id := json.RawMessage(`"test-id"`)
	req := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "admin/action",
	}
	
	resp, err := handler.HandleRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}
	
	if resp.Error == nil {
		t.Error("Expected error response when no user in context")
	}
	
	if resp.Error.Message != "No authenticated user found" {
		t.Errorf("Expected specific error message, got '%s'", resp.Error.Message)
	}
}

func TestAuthorizationMiddleware_MethodPermissions(t *testing.T) {
	permChecker := &mockPermissionChecker{}
	
	config := AuthorizationConfig{
		PermissionChecker: permChecker,
		MethodPermissions: map[string]MethodPermission{
			"tools/list": {
				Scopes: []string{"tools:read"},
			},
			"tools/execute": {
				Roles: []string{"admin", "developer"},
			},
		},
	}
	
	authzMiddleware := AuthorizationMiddleware(config)
	handler := authzMiddleware(&mockHandler{})
	
	// Test user with required scope
	userWithScope := &User{
		ID:     "user1",
		Scopes: []string{"tools:read"},
	}
	ctx := WithUser(context.Background(), userWithScope)
	
	id := json.RawMessage(`"test-id"`)
	req := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "tools/list",
	}
	
	resp, err := handler.HandleRequest(ctx, req)
	if err != nil {
		t.Fatalf("Authorization failed: %v", err)
	}
	
	if resp.Error != nil {
		t.Error("Expected successful response for user with required scope")
	}
	
	// Test user without required role
	userWithoutRole := &User{
		ID:    "user2",
		Roles: []string{"user"},
	}
	ctx = WithUser(context.Background(), userWithoutRole)
	
	req.Method = "tools/execute"
	
	resp, err = handler.HandleRequest(ctx, req)
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}
	
	if resp.Error == nil {
		t.Error("Expected error response for user without required role")
	}
}

func TestSessionMiddleware(t *testing.T) {
	sessionManager := NewInMemorySessionManager(SessionManagerConfig{
		DefaultTTL: 1 * time.Hour,
	})
	defer sessionManager.Close()
	
	sessionMiddleware := SessionMiddleware(sessionManager)
	
	var capturedSession *Session
	mockNext := &mockHandler{
		handleRequestFunc: func(ctx context.Context, req *types.JSONRPCMessage) (*types.JSONRPCMessage, error) {
			session, ok := GetSessionFromContext(ctx)
			if ok {
				capturedSession = session
			}
			return &types.JSONRPCMessage{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  json.RawMessage(`{"success": true}`),
			}, nil
		},
	}
	
	handler := sessionMiddleware(mockNext)
	
	// Create context with user
	user := &User{ID: "test-user"}
	ctx := WithUser(context.Background(), user)
	
	id := json.RawMessage(`"test-id"`)
	req := &types.JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "test/method",
	}
	
	resp, err := handler.HandleRequest(ctx, req)
	if err != nil {
		t.Fatalf("Session middleware failed: %v", err)
	}
	
	if resp.Error != nil {
		t.Error("Expected successful response")
	}
	
	if capturedSession == nil {
		t.Error("Expected session to be created and added to context")
	} else if capturedSession.UserID != user.ID {
		t.Errorf("Expected session user ID '%s', got '%s'", user.ID, capturedSession.UserID)
	}
}

func TestExtractBearerToken(t *testing.T) {
	// Test token in context
	ctx := context.WithValue(context.Background(), "Authorization", "Bearer context-token")
	req := &types.JSONRPCMessage{}
	
	token, err := extractBearerToken(ctx, req)
	if err != nil {
		t.Fatalf("Failed to extract token from context: %v", err)
	}
	if token != "context-token" {
		t.Errorf("Expected token 'context-token', got '%s'", token)
	}
	
	// Test token in params
	params := map[string]interface{}{
		"authorization": "Bearer param-token",
	}
	paramsJSON, _ := json.Marshal(params)
	req.Params = paramsJSON
	
	token, err = extractBearerToken(context.Background(), req)
	if err != nil {
		t.Fatalf("Failed to extract token from params: %v", err)
	}
	if token != "param-token" {
		t.Errorf("Expected token 'param-token', got '%s'", token)
	}
	
	// Test no token
	_, err = extractBearerToken(context.Background(), &types.JSONRPCMessage{})
	if err == nil {
		t.Error("Expected error when no token found")
	}
	
	authErr, ok := err.(*AuthError)
	if !ok {
		t.Errorf("Expected AuthError, got %T", err)
	} else if authErr.Code != ErrCodeTokenNotFound {
		t.Errorf("Expected error code %s, got %s", ErrCodeTokenNotFound, authErr.Code)
	}
}