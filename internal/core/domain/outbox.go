package domain

import "time"

const (
	OutboxStatusPending = "pending"
	OutboxStatusFailed  = "failed"
)

// OutboxMessage is a durable email intent row (sys.outbox).
type OutboxMessage struct {
	ID          string
	Purpose     string
	Payload     []byte
	Status      string
	Attempts    int
	AvailableAt time.Time
	CreatedAt   time.Time
	ProcessedAt *time.Time
	LastError   *string
}
