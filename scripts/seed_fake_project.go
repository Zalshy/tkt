//go:build ignore

// seed_fake_project seeds the fake project DB used for screenshots.
// Run from the repository root:
//
//	go run scripts/seed_fake_project.go
package main

import (
	"fmt"
	"log"

	"github.com/zalshy/tkt/internal/db"
)

func main() {
	database, err := db.Open(".screenshots/fake-project")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer database.Close()

	// Insert recent ticket_log transition rows so the changes feed shows live activity.
	_, err = database.Exec(`
		INSERT INTO ticket_log (ticket_id, session_name, kind, body, from_state, to_state, created_at)
		VALUES
		  (2, 'bob-impl',   'transition', '', 'TODO',        'PLANNING',    datetime('now', '-2 minutes')),
		  (4, 'alice-arch', 'transition', '', 'PLANNING',    'IN_PROGRESS', datetime('now', '-8 minutes')),
		  (1, 'bob-impl',   'transition', '', 'IN_PROGRESS', 'DONE',        datetime('now', '-1 hour')),
		  (5, 'carol-impl', 'transition', '', 'TODO',        'PLANNING',    datetime('now', '-3 hours'))
	`)
	if err != nil {
		log.Fatalf("insert ticket_log rows: %v", err)
	}
	fmt.Println("inserted 4 ticket_log transition rows")

	// Insert an active session so the sessions panel shows a live participant.
	// expired_at is left NULL so the session is treated as active.
	_, err = database.Exec(`
		INSERT OR IGNORE INTO sessions (id, role, name, created_at, last_active)
		VALUES ('fake-arch-01', 'architect', 'alice-arch', datetime('now', '-30 minutes'), datetime('now', '-1 minute'))
	`)
	if err != nil {
		log.Fatalf("insert session row: %v", err)
	}
	fmt.Println("inserted active session row (alice-arch)")

	fmt.Println("done — fake project DB seeded")
}
