package models

import "time"

// Context maps to the project_context table.
type Context struct {
	ID        int
	Title     string
	Body      string
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}
