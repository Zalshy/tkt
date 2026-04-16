package models

import "time"

// UsageEntry maps to the ticket_usage table.
type UsageEntry struct {
	ID         int64
	TicketID   int64
	SessionID  string
	Tokens     int
	Tools      int
	DurationMs int
	Agent      string
	Label      string
	CreatedAt  time.Time
	DeletedAt  *time.Time
}
