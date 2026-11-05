package domain

import "time"

const (
	OutboxStatusPending    = "pending"
	OutboxStatusProcessing = "processing"
	OutboxStatusSent       = "sent"
	OutboxStatusFailed     = "failed"
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

// Product is the example app domain entity (app.product).
type Product struct {
	ID        string
	Name      string
	Price     string // decimal as string to avoid float
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}
