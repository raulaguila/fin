package output

import (
	"context"
	"time"

	"github.com/raulaguila/authenticity/internal/core/domain"
)

// UserRepository persists usr.usr_user.
type UserRepository interface {
	FindByID(ctx context.Context, id string) (*domain.User, error)
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	Create(ctx context.Context, user *domain.User) error
	// UpdateStatus leaves a disabled account as it is: a login or activation already
	// running when the account was disabled must not undo that.
	UpdateStatus(ctx context.Context, id, status string) error
	// Enable lifts a disable: the account goes back to active, or to pending if it
	// never set a password. Any other status stays.
	Enable(ctx context.Context, id string) error
	// List pages through the users without a protected role.
	List(ctx context.Context, filter domain.UserFilter) ([]domain.User, int64, error)
}

// CredentialRepository persists usr.usr_credential.
type CredentialRepository interface {
	FindByUserID(ctx context.Context, userID string) (*domain.Credential, error)
	// Create inserts the empty credential of a new user.
	Create(ctx context.Context, userID string) error
	// SetPassword stores a new hash and clears the lockout.
	SetPassword(ctx context.Context, userID, hash, algorithm string) error
	// RecordFailure atomically counts a bad password, restarting the count after
	// a lapsed lock, and locks until lockUntil once maxAttempts is reached.
	RecordFailure(ctx context.Context, userID string, maxAttempts int, lockUntil, now time.Time) (lockedUntil *time.Time, err error)
	// RecordSuccess clears the counters only if the verified hash is still current
	// and no lock started meanwhile. False means the login must be refused.
	RecordSuccess(ctx context.Context, userID, verifiedHash string, now time.Time) (bool, error)
}

// SessionRepository persists usr.usr_session.
type SessionRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Session, error)
	Create(ctx context.Context, session *domain.Session) error
	// Rotate swaps the refresh hash only if currentHash is still the live one.
	// False means another request already rotated or revoked the session.
	Rotate(ctx context.Context, id string, currentHash, newHash []byte, expiresAt, now time.Time) (bool, error)
	// Revoke is idempotent: an unknown or already revoked session is not an error.
	Revoke(ctx context.Context, id string, now time.Time, reason string) error
	RevokeAllForUser(ctx context.Context, userID string, now time.Time, reason string) error
	// ListActive returns the live sessions of a user, most recently used first.
	ListActive(ctx context.Context, userID string, now time.Time, limit int) ([]domain.Session, error)
	// RevokeOwned revokes a live session of this user. False means there is none.
	RevokeOwned(ctx context.Context, id, userID string, now time.Time, reason string) (bool, error)
	// RevokeOthers revokes every session of the user except keepID.
	RevokeOthers(ctx context.Context, userID, keepID string, now time.Time, reason string) error
	// KnownClient reports whether this client already logged in to the account.
	// An account with no session yet knows every client: its first login is no news.
	KnownClient(ctx context.Context, userID, ip string) (bool, error)
}

// TokenRepository persists usr.usr_token.
type TokenRepository interface {
	Create(ctx context.Context, token *domain.EmailToken) error
	FindActiveByHash(ctx context.Context, purpose string, hash []byte) (*domain.EmailToken, error)
	MarkUsed(ctx context.Context, id string, usedAt time.Time) error
	IncrementAttempts(ctx context.Context, id string) (attempts int, err error)
	InvalidateActive(ctx context.Context, userID, purpose string) error
	// IssuedSince reports whether a live token of this purpose was created at or after since.
	IssuedSince(ctx context.Context, userID, purpose string, since time.Time) (bool, error)
	// CountIssuedSince counts tokens of this purpose created at or after since, spent ones too.
	CountIssuedSince(ctx context.Context, userID, purpose string, since time.Time) (int, error)
}

// LoginAttemptRepository appends login audits.
type LoginAttemptRepository interface {
	Create(ctx context.Context, attempt *domain.LoginAttempt) error
	CountRecentFailuresByEmail(ctx context.Context, email string, since time.Time) (int, error)
	// CountRecentFailuresByIP counts the whole client: an IPv6 address counts its /64.
	CountRecentFailuresByIP(ctx context.Context, ip string, since time.Time) (int, error)
	// CountRecentFailuresByEmailAndIP counts one client's failures against one account.
	CountRecentFailuresByEmailAndIP(ctx context.Context, email, ip string, since time.Time) (int, error)
	// ClearFailures forgets what counts against an account since a moment, older
	// history stays. A password reset proves the owner, who must get back in.
	ClearFailures(ctx context.Context, email string, since time.Time) error
	// Lock makes concurrent logins sharing a key wait until the transaction ends.
	Lock(ctx context.Context, key string) error
	// Settle replaces the outcome of a reserved attempt.
	Settle(ctx context.Context, id string, success bool, failureKind *string) error
	// Release drops a reserved attempt that tested no password.
	Release(ctx context.Context, id string) error
}

// RoleRepository resolves roles and assigns them to users.
type RoleRepository interface {
	FindDefaultRoleID(ctx context.Context) (string, error)
	FindIDByName(ctx context.Context, name string) (string, error)
	Assign(ctx context.Context, userID, roleID string) error
	// HasMembers reports whether any user holds the role.
	HasMembers(ctx context.Context, roleID string) (bool, error)
	List(ctx context.Context, limit int) ([]domain.Role, error)
	FindByID(ctx context.Context, id string) (*domain.Role, error)
	Create(ctx context.Context, name string) (*domain.Role, error)
	Delete(ctx context.Context, id string) error
	Update(ctx context.Context, id string, patch domain.RolePatch) error
	ClearPermissions(ctx context.Context, roleID string) error
	Grant(ctx context.Context, roleID, resource string, actions domain.Action) error
	Members(ctx context.Context, roleID string, page domain.Page) ([]domain.User, int64, error)
	// ForUser returns every role of a user, enabled or not.
	ForUser(ctx context.Context, userID string) ([]domain.Role, error)
	Unassign(ctx context.Context, userID, roleID string) error
}
