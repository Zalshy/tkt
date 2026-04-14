package models

import "time"

// Status represents the state of a ticket in the state machine.
type Status string

const (
	StatusTodo       Status = "TODO"
	StatusPlanning   Status = "PLANNING"
	StatusInProgress Status = "IN_PROGRESS"
	StatusDone       Status = "DONE"
	StatusVerified   Status = "VERIFIED"
	StatusCanceled   Status = "CANCELED"
	StatusArchived   Status = "ARCHIVED"
)

// Ticket maps to the tickets table.
type Ticket struct {
	ID          int64
	Title       string
	Description string
	Status      Status
	Tier        string // "critical", "standard", "low"
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}
