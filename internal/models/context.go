package models

import "time"

// Context maps to the context_entries table.
type Context struct {
	ID        int
	Title     string
	Body      string
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}
