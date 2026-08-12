package payloads

import "context"

type ListPayload struct {
	Skip          int               `json:"skip"`
	Limit         int               `json:"limit"`
	Filters       map[string]string `json:"filters"`
	Search        string            `json:"search"`
	SortBy        string            `json:"sortBy"`
	SortDirection string            `json:"sort"`
}

type MutationRequest struct {
	ID      int64 `json:"-" validate:"required,gt=0"`
	Version int64 `json:"version" validate:"required,gt=0"`
}

// Context keys for authorization
type contextKey string

const (
	requiredPermissionKey     contextKey = "required_permission"
	requiredAnyPermissionKey  contextKey = "required_any_permission"
	requiredAllPermissionKey  contextKey = "required_all_permissions"
	requiredRoleKey           contextKey = "required_role"
)

// WithRequiredPermission adds a required permission to the context
func WithRequiredPermission(ctx context.Context, permission string) context.Context {
	return context.WithValue(ctx, requiredPermissionKey, permission)
}

// GetRequiredPermission retrieves the required permission from context
func GetRequiredPermission(ctx context.Context) (string, bool) {
	v := ctx.Value(requiredPermissionKey)
	if v == nil {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// WithRequiredAnyPermission adds a list of required permissions (any) to the context
func WithRequiredAnyPermission(ctx context.Context, permissions []string) context.Context {
	return context.WithValue(ctx, requiredAnyPermissionKey, permissions)
}

// GetRequiredAnyPermission retrieves the required any permissions from context
func GetRequiredAnyPermission(ctx context.Context) ([]string, bool) {
	v := ctx.Value(requiredAnyPermissionKey)
	if v == nil {
		return nil, false
	}
	s, ok := v.([]string)
	return s, ok
}

// WithRequiredAllPermissions adds a list of required permissions (all) to the context
func WithRequiredAllPermissions(ctx context.Context, permissions []string) context.Context {
	return context.WithValue(ctx, requiredAllPermissionKey, permissions)
}

// GetRequiredAllPermissions retrieves the required all permissions from context
func GetRequiredAllPermissions(ctx context.Context) ([]string, bool) {
	v := ctx.Value(requiredAllPermissionKey)
	if v == nil {
		return nil, false
	}
	s, ok := v.([]string)
	return s, ok
}

// WithRequiredRole adds a required role to the context
func WithRequiredRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, requiredRoleKey, role)
}

// GetRequiredRole retrieves the required role from context
func GetRequiredRole(ctx context.Context) (string, bool) {
	v := ctx.Value(requiredRoleKey)
	if v == nil {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}
