package models

import "time"

type LogEntry struct {
	ID        int64
	TicketID  int64
	SessionID string
	Kind      string
	Body      string
	FromState *Status
	ToState   *Status
	CreatedAt time.Time
	DeletedAt *time.Time
}
