package auth

import (
	"context"
	"testing"
)

func TestNewBasicPermissionChecker(t *testing.T) {
	pc := NewBasicPermissionChecker()
	
	if pc == nil {
		t.Fatal("Expected NewBasicPermissionChecker to return non-nil checker")
	}
	
	if pc.rolePermissions == nil {
		t.Error("Expected rolePermissions to be initialized")
	}
	
	if pc.resourcePermissions == nil {
		t.Error("Expected resourcePermissions to be initialized")
	}
	
	if pc.toolPermissions == nil {
		t.Error("Expected toolPermissions to be initialized")
	}
}

func TestBasicPermissionChecker_AddRolePermission(t *testing.T) {
	pc := NewBasicPermissionChecker()
	
	pc.AddRolePermission("admin", "delete", []string{"file/*", "user/*"})
	pc.AddRolePermission("user", "read", []string{"file/*"})
	
	// Verify role permissions were added
	if len(pc.rolePermissions) != 2 {
		t.Errorf("Expected 2 roles, got %d", len(pc.rolePermissions))
	}
	
	adminPerms, ok := pc.rolePermissions["admin"]
	if !ok {
		t.Error("Expected admin role to exist")
	}
	
	deleteResources, ok := adminPerms["delete"]
	if !ok {
		t.Error("Expected delete action for admin")
	}
	
	if len(deleteResources) != 2 {
		t.Errorf("Expected 2 resources for admin delete, got %d", len(deleteResources))
	}
}

func TestBasicPermissionChecker_AddResourcePermission(t *testing.T) {
	pc := NewBasicPermissionChecker()
	
	pc.AddResourcePermission("file", "doc1.txt", "user123", []string{"read", "write"})
	pc.AddResourcePermission("file", "doc1.txt", "user456", []string{"read"})
	
	// Verify resource permissions were added
	filePerms, ok := pc.resourcePermissions["file"]
	if !ok {
		t.Error("Expected file resource type to exist")
	}
	
	doc1Perms, ok := filePerms["doc1.txt"]
	if !ok {
		t.Error("Expected doc1.txt resource to exist")
	}
	
	user123Actions, ok := doc1Perms["user123"]
	if !ok {
		t.Error("Expected user123 permissions to exist")
	}
	
	if len(user123Actions) != 2 {
		t.Errorf("Expected 2 actions for user123, got %d", len(user123Actions))
	}
}

func TestBasicPermissionChecker_AddToolPermission(t *testing.T) {
	pc := NewBasicPermissionChecker()
	
	toolPerm := ToolPermission{
		AllowedRoles:  []string{"admin"},
		AllowedScopes: []string{"tools:admin"},
		CustomCheck: func(user *User, args map[string]interface{}) bool {
			return user.HasRole("admin")
		},
	}
	
	pc.AddToolPermission("dangerous-tool", toolPerm)
	
	// Verify tool permission was added
	retrievedPerm, ok := pc.toolPermissions["dangerous-tool"]
	if !ok {
		t.Error("Expected dangerous-tool permission to exist")
	}
	
	if len(retrievedPerm.AllowedRoles) != 1 || retrievedPerm.AllowedRoles[0] != "admin" {
		t.Error("Expected admin role permission")
	}
}

func TestBasicPermissionChecker_CheckPermission_RoleBased(t *testing.T) {
	pc := NewBasicPermissionChecker()
	
	// Setup role permissions
	pc.AddRolePermission("admin", "delete", []string{"file/*"})
	pc.AddRolePermission("user", "read", []string{"file/*"})
	
	ctx := context.Background()
	
	// Test admin can delete files
	adminUser := &User{
		ID:    "admin1",
		Roles: []string{"admin"},
	}
	
	allowed, err := pc.CheckPermission(ctx, adminUser, "delete", "file/test.txt")
	if err != nil {
		t.Fatalf("CheckPermission failed: %v", err)
	}
	if !allowed {
		t.Error("Expected admin to be allowed to delete files")
	}
	
	// Test user cannot delete files
	regularUser := &User{
		ID:    "user1",
		Roles: []string{"user"},
	}
	
	allowed, err = pc.CheckPermission(ctx, regularUser, "delete", "file/test.txt")
	if err != nil {
		t.Fatalf("CheckPermission failed: %v", err)
	}
	if allowed {
		t.Error("Expected user not to be allowed to delete files")
	}
	
	// Test user can read files
	allowed, err = pc.CheckPermission(ctx, regularUser, "read", "file/test.txt")
	if err != nil {
		t.Fatalf("CheckPermission failed: %v", err)
	}
	if !allowed {
		t.Error("Expected user to be allowed to read files")
	}
}

func TestBasicPermissionChecker_CheckPermission_ResourceBased(t *testing.T) {
	pc := NewBasicPermissionChecker()
	
	// Setup resource-specific permissions
	pc.AddResourcePermission("file", "secret.txt", "user1", []string{"read"})
	pc.AddResourcePermission("file", "public.txt", "user1", []string{"read", "write"})
	
	ctx := context.Background()
	user := &User{ID: "user1"}
	
	// Test user can read secret file
	allowed, err := pc.CheckPermission(ctx, user, "read", "file/secret.txt")
	if err != nil {
		t.Fatalf("CheckPermission failed: %v", err)
	}
	if !allowed {
		t.Error("Expected user to be allowed to read secret file")
	}
	
	// Test user cannot write secret file
	allowed, err = pc.CheckPermission(ctx, user, "write", "file/secret.txt")
	if err != nil {
		t.Fatalf("CheckPermission failed: %v", err)
	}
	if allowed {
		t.Error("Expected user not to be allowed to write secret file")
	}
	
	// Test user can write public file
	allowed, err = pc.CheckPermission(ctx, user, "write", "file/public.txt")
	if err != nil {
		t.Fatalf("CheckPermission failed: %v", err)
	}
	if !allowed {
		t.Error("Expected user to be allowed to write public file")
	}
}

func TestBasicPermissionChecker_CheckPermission_ScopeBased(t *testing.T) {
	pc := NewBasicPermissionChecker()
	
	ctx := context.Background()
	user := &User{
		ID:     "user1",
		Scopes: []string{"file:read", "document:*"},
	}
	
	// Test scope-based permission (exact match)
	allowed, err := pc.CheckPermission(ctx, user, "read", "file")
	if err != nil {
		t.Fatalf("CheckPermission failed: %v", err)
	}
	if !allowed {
		t.Error("Expected user to be allowed based on exact scope match")
	}
	
	// Test wildcard scope
	allowed, err = pc.CheckPermission(ctx, user, "write", "document")
	if err != nil {
		t.Fatalf("CheckPermission failed: %v", err)
	}
	if !allowed {
		t.Error("Expected user to be allowed based on wildcard scope")
	}
	
	// Test denied permission
	allowed, err = pc.CheckPermission(ctx, user, "delete", "admin")
	if err != nil {
		t.Fatalf("CheckPermission failed: %v", err)
	}
	if allowed {
		t.Error("Expected user not to be allowed for admin actions")
	}
}

func TestBasicPermissionChecker_CheckResourceAccess(t *testing.T) {
	pc := NewBasicPermissionChecker()
	
	// Setup role-based resource access
	pc.AddRolePermission("user", "read", []string{"file/*"})
	
	ctx := context.Background()
	user := &User{
		ID:    "user1",
		Roles: []string{"user"},
	}
	
	allowed, err := pc.CheckResourceAccess(ctx, user, "file", "test.txt")
	if err != nil {
		t.Fatalf("CheckResourceAccess failed: %v", err)
	}
	if !allowed {
		t.Error("Expected user to have resource access")
	}
}

func TestBasicPermissionChecker_CheckToolAccess(t *testing.T) {
	pc := NewBasicPermissionChecker()
	
	// Setup tool permission
	toolPerm := ToolPermission{
		AllowedRoles: []string{"admin", "developer"},
		CustomCheck: func(user *User, args map[string]interface{}) bool {
			// Additional check: only allow if user has specific metadata
			return user.Metadata["clearance"] == "high"
		},
	}
	pc.AddToolPermission("sensitive-tool", toolPerm)
	
	ctx := context.Background()
	
	// Test admin user without metadata
	adminUser := &User{
		ID:       "admin1",
		Roles:    []string{"admin"},
		Metadata: make(map[string]interface{}),
	}
	
	allowed, err := pc.CheckToolAccess(ctx, adminUser, "sensitive-tool", nil)
	if err != nil {
		t.Fatalf("CheckToolAccess failed: %v", err)
	}
	if allowed {
		t.Error("Expected admin without high clearance to be denied")
	}
	
	// Test admin user with metadata
	adminUser.Metadata["clearance"] = "high"
	allowed, err = pc.CheckToolAccess(ctx, adminUser, "sensitive-tool", nil)
	if err != nil {
		t.Fatalf("CheckToolAccess failed: %v", err)
	}
	if !allowed {
		t.Error("Expected admin with high clearance to be allowed")
	}
	
	// Test user without admin role
	regularUser := &User{
		ID:       "user1",
		Roles:    []string{"user"},
		Metadata: map[string]interface{}{"clearance": "high"},
	}
	
	allowed, err = pc.CheckToolAccess(ctx, regularUser, "sensitive-tool", nil)
	if err != nil {
		t.Fatalf("CheckToolAccess failed: %v", err)
	}
	if allowed {
		t.Error("Expected regular user to be denied regardless of clearance")
	}
}

func TestBasicPermissionChecker_CheckToolAccess_ScopeBased(t *testing.T) {
	pc := NewBasicPermissionChecker()
	
	// Setup tool permission with scope requirement
	toolPerm := ToolPermission{
		AllowedScopes: []string{"tools:admin", "tools:write"},
	}
	pc.AddToolPermission("admin-tool", toolPerm)
	
	ctx := context.Background()
	
	// Test user with required scope
	userWithScope := &User{
		ID:     "user1",
		Scopes: []string{"tools:admin", "data:read"},
	}
	
	allowed, err := pc.CheckToolAccess(ctx, userWithScope, "admin-tool", nil)
	if err != nil {
		t.Fatalf("CheckToolAccess failed: %v", err)
	}
	if !allowed {
		t.Error("Expected user with required scope to be allowed")
	}
	
	// Test user without required scope
	userWithoutScope := &User{
		ID:     "user2",
		Scopes: []string{"data:read", "data:write"},
	}
	
	allowed, err = pc.CheckToolAccess(ctx, userWithoutScope, "admin-tool", nil)
	if err != nil {
		t.Fatalf("CheckToolAccess failed: %v", err)
	}
	if allowed {
		t.Error("Expected user without required scope to be denied")
	}
}

func TestBasicPermissionChecker_CheckToolAccess_Fallback(t *testing.T) {
	pc := NewBasicPermissionChecker()
	
	// Setup general tool access permission
	pc.AddRolePermission("user", "execute", []string{"tool/*"})
	
	ctx := context.Background()
	user := &User{
		ID:    "user1",
		Roles: []string{"user"},
	}
	
	// Test tool without specific permission (should fall back to general permission)
	allowed, err := pc.CheckToolAccess(ctx, user, "generic-tool", nil)
	if err != nil {
		t.Fatalf("CheckToolAccess failed: %v", err)
	}
	if !allowed {
		t.Error("Expected user to be allowed to execute generic tool")
	}
}

func TestNewMCPPermissionChecker(t *testing.T) {
	pc := NewMCPPermissionChecker()
	
	if pc == nil {
		t.Fatal("Expected NewMCPPermissionChecker to return non-nil checker")
	}
	
	// Verify default permissions were setup
	if len(pc.rolePermissions) == 0 {
		t.Error("Expected default role permissions to be setup")
	}
	
	if len(pc.toolPermissions) == 0 {
		t.Error("Expected default tool permissions to be setup")
	}
}

func TestMCPPermissionChecker_DefaultPermissions(t *testing.T) {
	pc := NewMCPPermissionChecker()
	ctx := context.Background()
	
	// Test admin has full access
	adminUser := &User{
		ID:    "admin1",
		Roles: []string{"admin"},
	}
	
	allowed, err := pc.CheckPermission(ctx, adminUser, "delete", "resource/anything")
	if err != nil {
		t.Fatalf("CheckPermission failed: %v", err)
	}
	if !allowed {
		t.Error("Expected admin to have full access")
	}
	
	// Test user has basic access
	regularUser := &User{
		ID:    "user1",
		Roles: []string{"user"},
	}
	
	allowed, err = pc.CheckPermission(ctx, regularUser, "read", "resource/test")
	if err != nil {
		t.Fatalf("CheckPermission failed: %v", err)
	}
	if !allowed {
		t.Error("Expected user to have read access to resources")
	}
	
	// Test reader has read-only access
	readerUser := &User{
		ID:    "reader1",
		Roles: []string{"reader"},
	}
	
	allowed, err = pc.CheckPermission(ctx, readerUser, "read", "resource/test")
	if err != nil {
		t.Fatalf("CheckPermission failed: %v", err)
	}
	if !allowed {
		t.Error("Expected reader to have read access")
	}
	
	allowed, err = pc.CheckPermission(ctx, readerUser, "write", "resource/test")
	if err != nil {
		t.Fatalf("CheckPermission failed: %v", err)
	}
	if allowed {
		t.Error("Expected reader not to have write access")
	}
}

func TestHasMCPAccess(t *testing.T) {
	// Test user with MCP scope
	userWithScope := &User{
		Scopes: []string{"mcp:read", "other:scope"},
	}
	
	if !HasMCPAccess(userWithScope) {
		t.Error("Expected user with mcp:read scope to have MCP access")
	}
	
	// Test user with MCP role
	userWithRole := &User{
		Roles: []string{"user", "other-role"},
	}
	
	if !HasMCPAccess(userWithRole) {
		t.Error("Expected user with user role to have MCP access")
	}
	
	// Test user without MCP access
	userWithoutAccess := &User{
		Scopes: []string{"other:scope"},
		Roles:  []string{"other-role"},
	}
	
	if HasMCPAccess(userWithoutAccess) {
		t.Error("Expected user without MCP scope or role not to have MCP access")
	}
}

func TestCanExecuteTools(t *testing.T) {
	// Test user with tools scope
	userWithScope := &User{
		Scopes: []string{"mcp:tools"},
	}
	
	if !CanExecuteTools(userWithScope) {
		t.Error("Expected user with mcp:tools scope to execute tools")
	}
	
	// Test user with user role
	userWithRole := &User{
		Roles: []string{"user"},
	}
	
	if !CanExecuteTools(userWithRole) {
		t.Error("Expected user with user role to execute tools")
	}
	
	// Test user without tool access
	userWithoutAccess := &User{
		Scopes: []string{"other:scope"},
		Roles:  []string{"reader"},
	}
	
	if !CanExecuteTools(userWithoutAccess) {
		t.Error("Expected reader role to be able to execute tools")
	}
}

func TestCanAccessResources(t *testing.T) {
	// Test user with resources scope
	userWithScope := &User{
		Scopes: []string{"mcp:resources"},
	}
	
	if !CanAccessResources(userWithScope) {
		t.Error("Expected user with mcp:resources scope to access resources")
	}
	
	// Test reader role
	readerUser := &User{
		Roles: []string{"reader"},
	}
	
	if !CanAccessResources(readerUser) {
		t.Error("Expected reader role to access resources")
	}
}

func TestCanAccessPrompts(t *testing.T) {
	// Test user with prompts scope
	userWithScope := &User{
		Scopes: []string{"mcp:prompts"},
	}
	
	if !CanAccessPrompts(userWithScope) {
		t.Error("Expected user with mcp:prompts scope to access prompts")
	}
	
	// Test reader role
	readerUser := &User{
		Roles: []string{"reader"},
	}
	
	if !CanAccessPrompts(readerUser) {
		t.Error("Expected reader role to access prompts")
	}
}