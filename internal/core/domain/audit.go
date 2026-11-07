package domain

// AuditEntry records an administrative change. The action names the kind of
// target ("role.update"); an empty ActorID means the operator CLI.
type AuditEntry struct {
	ActorID   string
	RequestID string
	Action    string
	TargetID  string
	Details   map[string]any
}
