package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mrand "math/rand"
	"regexp"
)

// wordlist is the pool from which random session IDs are drawn.
var wordlist = []string{
	// original 30
	"cedar", "oak", "pine", "birch", "elm", "ash", "ivy", "vale", "cove", "moor",
	"dusk", "dawn", "fern", "reed", "moss", "clay", "flint", "sage", "wren", "crow",
	"haze", "gale", "tern", "lark", "bolt", "gust", "tide", "reef", "dune", "crag",
	// added ~70 short nature/landscape words
	"fog", "fen", "bay", "burn", "col", "dale", "dell", "fell", "ford", "glen",
	"gorge", "heath", "holm", "isle", "knoll", "lea", "loch", "mere", "mire", "nook",
	"peat", "rill", "shaw", "tarn", "tor", "beck", "bluff", "bog", "brow", "cape",
	"crest", "croft", "down", "eddy", "gill", "holt", "hurst", "lin", "marl", "ness",
	"pike", "pool", "scree", "sedge", "sump", "swale", "turf", "weir", "wold", "cay",
	"carr", "lade", "leat", "esker", "floe", "grot", "howe", "linn", "plash", "rake",
	"rigg", "voe", "brae", "lagg", "rand", "skerry", "sloe", "tump", "wath", "wynd",
}

// namePattern is the validation regex for user-supplied session names.
// Allows lowercase alphanumeric and internal hyphens; no leading/trailing/consecutive hyphens.
// Minimum 1 character; maximum 32 (enforced separately in ValidateName).
// Allows a single [a-z0-9] character OR a sequence starting and ending with
// [a-z0-9] with optional internal hyphens.
var namePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// consecutiveHyphens matches two or more hyphens in a row.
var consecutiveHyphens = regexp.MustCompile(`--`)

// GenerateID returns a session ID.
//
// If name is non-empty it is returned as-is (caller must have already validated it
// via ValidateName). If name is empty a random word is drawn from the wordlist.
//
// This function does NOT guarantee uniqueness — collision handling is the
// responsibility of Create, which retries on PK constraint errors.
func GenerateID(name string) string {
	if name != "" {
		return name
	}
	return wordlist[mrand.Intn(len(wordlist))] //nolint:gosec
}

// ValidateName checks that a user-supplied session name conforms to the naming rules:
//   - Lowercase alphanumeric + hyphens only
//   - No leading or trailing hyphen
//   - No consecutive hyphens
//   - At least 1 character, at most 32 characters
//
// Returns a descriptive error if the name is invalid, nil otherwise.
func ValidateName(name string) error {
	if len(name) == 0 {
		return fmt.Errorf("session name must not be empty")
	}
	if len(name) > 32 {
		return fmt.Errorf("session name %q exceeds 32-character limit (%d chars)", name, len(name))
	}
	if consecutiveHyphens.MatchString(name) {
		return fmt.Errorf("session name %q contains consecutive hyphens", name)
	}
	if !namePattern.MatchString(name) {
		return fmt.Errorf("session name %q is invalid: use lowercase letters, digits, and non-leading/trailing hyphens only", name)
	}
	return nil
}

// randomHex4 returns a 4-character lowercase hex string using crypto/rand.
func randomHex4() (string, error) {
	buf := make([]byte, 2)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("randomHex4: crypto/rand: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
