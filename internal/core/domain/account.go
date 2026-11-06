package domain

import "time"

// Profile is what a client needs to know about the logged in user.
type Profile struct {
	User        User
	IsSuperuser bool
	Permissions map[string]Action
	Preferences Preferences
}

// Theme values match usr.usr_preference.
const (
	ThemeSystem = "system"
	ThemeLight  = "light"
	ThemeDark   = "dark"

	DefaultLanguage = "pt-BR"
)

// Preferences holds a user's interface choices. A user with no row gets the defaults.
type Preferences struct {
	UserID   string
	Theme    string
	Language string
}

// Notification levels match usr.usr_notification.
const (
	NotificationInfo     = "info"
	NotificationWarning  = "warning"
	NotificationCritical = "critical"
)

// Notification is a notice shown to one user.
type Notification struct {
	ID        string
	UserID    string
	Level     string
	Title     string
	Message   string
	ReadAt    *time.Time
	CreatedAt time.Time
}
