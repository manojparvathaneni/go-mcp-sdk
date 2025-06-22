package auth

import (
	"context"
	"fmt"
	"strings"
)

// BasicPermissionChecker provides a simple role and scope-based permission system
type BasicPermissionChecker struct {
	// Role-based permissions: role -> actions -> resources
	rolePermissions map[string]map[string][]string
	
	// Resource-level permissions: resource_type -> resource_id -> user_id -> actions
	resourcePermissions map[string]map[string]map[string][]string
	
	// Tool permissions: tool_name -> allowed_roles/scopes
	toolPermissions map[string]ToolPermission
}

// ToolPermission defines who can execute a specific tool
type ToolPermission struct {
	AllowedRoles  []string                                    `json:"allowed_roles,omitempty"`
	AllowedScopes []string                                    `json:"allowed_scopes,omitempty"`
	CustomCheck   func(user *User, args map[string]interface{}) bool `json:"-"`
}

// NewBasicPermissionChecker creates a basic permission checker
func NewBasicPermissionChecker() *BasicPermissionChecker {
	return &BasicPermissionChecker{
		rolePermissions:     make(map[string]map[string][]string),
		resourcePermissions: make(map[string]map[string]map[string][]string),
		toolPermissions:     make(map[string]ToolPermission),
	}
}

// AddRolePermission adds a permission for a role to perform an action on resources
func (pc *BasicPermissionChecker) AddRolePermission(role, action string, resources []string) {
	if pc.rolePermissions[role] == nil {
		pc.rolePermissions[role] = make(map[string][]string)
	}
	pc.rolePermissions[role][action] = resources
}

// AddResourcePermission adds permission for a user to perform actions on a specific resource
func (pc *BasicPermissionChecker) AddResourcePermission(resourceType, resourceID, userID string, actions []string) {
	if pc.resourcePermissions[resourceType] == nil {
		pc.resourcePermissions[resourceType] = make(map[string]map[string][]string)
	}
	if pc.resourcePermissions[resourceType][resourceID] == nil {
		pc.resourcePermissions[resourceType][resourceID] = make(map[string][]string)
	}
	pc.resourcePermissions[resourceType][resourceID][userID] = actions
}

// AddToolPermission adds permission configuration for a tool
func (pc *BasicPermissionChecker) AddToolPermission(toolName string, permission ToolPermission) {
	pc.toolPermissions[toolName] = permission
}

// CheckPermission checks if a user has permission to perform an action on a resource
func (pc *BasicPermissionChecker) CheckPermission(ctx context.Context, user *User, action, resource string) (bool, error) {
	// Check role-based permissions
	for _, role := range user.Roles {
		if actions, ok := pc.rolePermissions[role]; ok {
			// Check for exact action match
			if resources, ok := actions[action]; ok {
				for _, allowedResource := range resources {
					if pc.matchResource(resource, allowedResource) {
						return true, nil
					}
				}
			}
			// Check for wildcard action match
			if resources, ok := actions["*"]; ok {
				for _, allowedResource := range resources {
					if pc.matchResource(resource, allowedResource) {
						return true, nil
					}
				}
			}
		}
	}
	
	// Check resource-level permissions
	resourceType, resourceID := pc.parseResource(resource)
	if resourceType != "" && resourceID != "" {
		if resources, ok := pc.resourcePermissions[resourceType]; ok {
			if users, ok := resources[resourceID]; ok {
				if actions, ok := users[user.ID]; ok {
					for _, allowedAction := range actions {
						if action == allowedAction || allowedAction == "*" {
							return true, nil
						}
					}
				}
			}
		}
	}
	
	// Check scope-based permissions (simple scope-to-action mapping)
	requiredScope := fmt.Sprintf("%s:%s", resource, action)
	if user.HasScope(requiredScope) {
		return true, nil
	}
	
	// Check wildcard scopes
	resourceType, _ = pc.parseResource(resource)
	if resourceType != "" {
		wildcardScope := fmt.Sprintf("%s:*", resourceType)
		if user.HasScope(wildcardScope) {
			return true, nil
		}
	}
	
	return false, nil
}

// CheckResourceAccess checks if a user can access a specific resource instance
func (pc *BasicPermissionChecker) CheckResourceAccess(ctx context.Context, user *User, resourceType, resourceID string) (bool, error) {
	resource := fmt.Sprintf("%s/%s", resourceType, resourceID)
	
	// Check read permission (default for resource access)
	return pc.CheckPermission(ctx, user, "read", resource)
}

// CheckToolAccess checks if a user can execute a specific tool
func (pc *BasicPermissionChecker) CheckToolAccess(ctx context.Context, user *User, toolName string, args map[string]interface{}) (bool, error) {
	toolPerm, ok := pc.toolPermissions[toolName]
	if !ok {
		// No specific permission configured, check generic tool access
		return pc.CheckPermission(ctx, user, "execute", "tool/"+toolName)
	}
	
	// Check allowed roles
	if len(toolPerm.AllowedRoles) > 0 {
		hasRole := false
		for _, role := range toolPerm.AllowedRoles {
			if user.HasRole(role) {
				hasRole = true
				break
			}
		}
		if !hasRole {
			return false, nil
		}
	}
	
	// Check allowed scopes
	if len(toolPerm.AllowedScopes) > 0 {
		if !user.HasAnyScope(toolPerm.AllowedScopes) {
			return false, nil
		}
	}
	
	// Run custom check
	if toolPerm.CustomCheck != nil {
		if !toolPerm.CustomCheck(user, args) {
			return false, nil
		}
	}
	
	return true, nil
}

// matchResource checks if a resource matches a pattern (supports wildcards)
func (pc *BasicPermissionChecker) matchResource(resource, pattern string) bool {
	if pattern == "*" {
		return true
	}
	
	if pattern == resource {
		return true
	}
	
	// Simple wildcard matching
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(resource, prefix)
	}
	
	return false
}

// parseResource splits a resource string into type and ID
func (pc *BasicPermissionChecker) parseResource(resource string) (resourceType, resourceID string) {
	parts := strings.SplitN(resource, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return resource, ""
}

// SetupDefaultMCPPermissions configures default permissions for MCP operations
func (pc *BasicPermissionChecker) SetupDefaultMCPPermissions() {
	// Admin role - full access
	pc.AddRolePermission("admin", "*", []string{"*"})
	
	// User role - basic access
	pc.AddRolePermission("user", "read", []string{"resource/*", "prompt/*"})
	pc.AddRolePermission("user", "execute", []string{"tool/*"})
	
	// Reader role - read-only access
	pc.AddRolePermission("reader", "read", []string{"resource/*", "prompt/*"})
	pc.AddRolePermission("reader", "execute", []string{"tool/*"})
	
	// Tool executor role - can execute tools
	pc.AddRolePermission("tool_executor", "execute", []string{"tool/*"})
	
	// Resource manager - can manage resources
	pc.AddRolePermission("resource_manager", "*", []string{"resource/*"})
	
	// Common tool permissions
	pc.AddToolPermission("sensitive-tool", ToolPermission{
		AllowedRoles: []string{"admin", "senior_user"},
	})
	
	pc.AddToolPermission("read-only-tool", ToolPermission{
		AllowedRoles: []string{"admin", "user", "reader"},
	})
	
	pc.AddToolPermission("file-delete", ToolPermission{
		AllowedRoles: []string{"admin"},
		CustomCheck: func(user *User, args map[string]interface{}) bool {
			// Only allow deletion of user's own files unless admin
			if user.HasRole("admin") {
				return true
			}
			
			if filepath, ok := args["path"].(string); ok {
				// Simple check: path should contain user ID
				return strings.Contains(filepath, user.ID)
			}
			
			return false
		},
	})
}

// RBAC helper methods

// GrantUserRole grants a role to a user (utility for external use)
type RoleGrant struct {
	UserID string
	Role   string
}

// Permission helpers for common MCP patterns

// NewMCPPermissionChecker creates a permission checker with default MCP permissions
func NewMCPPermissionChecker() *BasicPermissionChecker {
	pc := NewBasicPermissionChecker()
	pc.SetupDefaultMCPPermissions()
	return pc
}

// HasMCPAccess checks if a user has basic MCP access
func HasMCPAccess(user *User) bool {
	// Basic MCP access requires at least one of these scopes or roles
	requiredScopes := []string{"mcp:read", "mcp:access"}
	requiredRoles := []string{"user", "admin", "reader"}
	
	if user.HasAnyScope(requiredScopes) {
		return true
	}
	
	for _, role := range requiredRoles {
		if user.HasRole(role) {
			return true
		}
	}
	
	return false
}

// CanExecuteTools checks if a user can execute tools
func CanExecuteTools(user *User) bool {
	return user.HasScope("mcp:tools") || user.HasRole("user") || user.HasRole("admin") || user.HasRole("tool_executor") || user.HasRole("reader")
}

// CanAccessResources checks if a user can access resources
func CanAccessResources(user *User) bool {
	return user.HasScope("mcp:resources") || user.HasRole("user") || user.HasRole("admin") || user.HasRole("reader")
}

// CanAccessPrompts checks if a user can access prompts
func CanAccessPrompts(user *User) bool {
	return user.HasScope("mcp:prompts") || user.HasRole("user") || user.HasRole("admin") || user.HasRole("reader")
}