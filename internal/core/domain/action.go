package domain

// Action bits (ADR-004). Shared vocabulary across resources.
type Action int32

const (
	ActionCreate Action = 1 << 0 // 1
	ActionRead   Action = 1 << 1 // 2
	ActionUpdate Action = 1 << 2 // 4
	ActionDelete Action = 1 << 3 // 8
)

// Allows reports whether have includes every bit in need.
func Allows(have, need Action) bool {
	return have&need == need
}

// Resource names are catalog codes (usr_resource) or the sentinel "*".
const ResourceAll = "*"
