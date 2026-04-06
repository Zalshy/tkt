package models

import "time"

// Role represents the role of a session actor.
type Role string

const (
	RoleArchitect   Role = "architect"
	RoleImplementer Role = "implementer"
)

// Session maps to the sessions table.
type Session struct {
	ID           string
	Role         Role  // raw DB value — the role name as stored
	EffectiveRole Role  // resolved base role: always RoleArchitect or RoleImplementer
	Name         string
	CreatedAt    time.Time
	LastActive   time.Time
	ExpiredAt    *time.Time
}
