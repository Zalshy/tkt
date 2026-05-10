package side

import (
	"hash/fnv"

	"github.com/charmbracelet/lipgloss"
)

// sessionPalette is a fixed set of distinct bright colours used to colour-code
// session names. The same name always maps to the same colour (FNV hash), so
// colours are consistent across both the Sessions and Ticket Changes boxes.
var sessionPalette = []lipgloss.Color{
	"#FF6B6B", // coral red
	"#4ECDC4", // teal
	"#45B7D1", // sky blue
	"#96CEB4", // sage green
	"#FFEAA7", // warm yellow
	"#DDA0DD", // plum
	"#F0A500", // amber
	"#98D8C8", // mint
	"#FF8C94", // salmon
	"#A8E6CF", // pale green
}

// sessionColor returns a deterministic colour for the given session name.
func sessionColor(name string) lipgloss.Color {
	h := fnv.New32a()
	h.Write([]byte(name))
	return sessionPalette[h.Sum32()%uint32(len(sessionPalette))]
}
