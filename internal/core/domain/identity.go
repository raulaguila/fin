package domain

import "time"

// UserStatus codes match chk_usr_user_status.
const (
	UserStatusPending  = "pending"
	UserStatusActive   = "active"
	UserStatusLocked   = "locked"
	UserStatusDisabled = "disabled"
)

// Role groups permissions for its members.
type Role struct {
	ID          string
	Name        string
	Enabled     bool
	IsDefault   bool
	IsSuperuser bool
	Permissions map[string]Action
	CreatedAt   time.Time
}

// RolePatch holds the role fields to change; nil keeps the current value.
type RolePatch struct {
	Name    *string
	Enabled *bool
}

// Protected reports whether the role is out of reach of the HTTP API: it holds
// full access, so only the operator commands of cmd/admin touch its members.
func (r Role) Protected() bool {
	_, all := r.Permissions[ResourceAll]
	return r.IsSuperuser || all
}

// User is the identity aggregate root (no transport tags).
type User struct {
	ID        string
	Name      string
	Email     string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// UserFilter narrows a user list; empty fields filter nothing.
type UserFilter struct {
	Page   Page
	Search string
	Status string
}

// UserDetails is a user with the roles it holds.
type UserDetails struct {
	User
	Roles []Role
}

// Credential is 1:1 with User.
type Credential struct {
	UserID              string
	PasswordHash        *string
	FailedLoginAttempts int
	LockedUntil         *time.Time
	LastLoginAt         *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// Session stores the current refresh jti hash and rotation state.
type Session struct {
	ID                       string
	UserID                   string
	ExpiresAt                time.Time
	RevokedAt                *time.Time
	RevocationReason         *string
	RefreshTokenHash         []byte
	PreviousRefreshTokenHash []byte
	IPAddress                *string
	UserAgent                *string
	CreatedAt                time.Time
	LastSeenAt               time.Time
}

// Principal is who an authenticated request acts as.
type Principal struct {
	UserID    string
	SessionID string
}

// EffectivePermissions is the authz snapshot of a user, loaded live on each check.
type EffectivePermissions struct {
	UserID      string
	IsSuperuser bool
	ByResource  map[string]Action // includes "*" if granted
}
