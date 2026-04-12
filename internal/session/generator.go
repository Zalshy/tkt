package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mrand "math/rand"
	"os"
	"regexp"
	"strings"

	"github.com/zalshy/tkt/internal/models"
)

// nonAlphanumeric strips anything that is not a lowercase ASCII letter or digit.
var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]`)

// GenerateID produces a session ID in the format:
//
//	{role_prefix}-{username}-{4_char_hex}
//
// Examples: arch-alice-3f9a, impl-bob-7c2e
//
// baseRole must be the resolved base role (models.RoleArchitect or models.RoleImplementer),
// not a custom role name. Call rolepkg.ResolveBase before invoking this function.
func GenerateID(baseRole models.Role) string {
	var prefix string
	switch baseRole {
	case models.RoleArchitect:
		prefix = "arch"
	default:
		prefix = "impl"
	}

	// Derive username from environment.
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("USERNAME")
	}
	username = nonAlphanumeric.ReplaceAllString(strings.ToLower(username), "")
	if username == "" {
		username = "user"
	}

	// Generate 4-char hex suffix using crypto/rand; fall back to math/rand.
	suffix := randomHex4()

	return fmt.Sprintf("%s-%s-%s", prefix, username, suffix)
}

// randomHex4 returns a 4-character lowercase hex string.
// Uses crypto/rand for security; falls back to math/rand if unavailable.
// The non-criticality of session IDs makes the math/rand fallback acceptable.
func randomHex4() string {
	buf := make([]byte, 2)
	if _, err := rand.Read(buf); err != nil {
		// Extremely rare; fall back to math/rand global source.
		n := mrand.Int63() //nolint:gosec
		buf[0] = byte(n)
		buf[1] = byte(n >> 8)
	}
	return hex.EncodeToString(buf)
}
