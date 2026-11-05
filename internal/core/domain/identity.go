package domain

import "time"

// UserStatus codes match usr.usr_user_status.
const (
	UserStatusPending  = "pending"
	UserStatusActive   = "active"
	UserStatusExpired  = "expired"
	UserStatusLocked   = "locked"
	UserStatusDisabled = "disabled"
)

// User is the identity aggregate root (no transport tags).
type User struct {
	ID        string
	Name      string
	Email     string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Credential is 1:1 with User.
type Credential struct {
	UserID              string
	PasswordHash        *string
	PasswordAlgorithm   *string
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

// Role is an RBAC role.
type Role struct {
	ID          string
	Name        string
	Enabled     bool
	IsDefault   bool
	IsSuperuser bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// EffectivePermissions is the authz snapshot for a user (live load / cache).
type EffectivePermissions struct {
	UserID      string
	IsSuperuser bool
	ByResource  map[string]Action // includes "*" if granted
}
