package side

import (
	"database/sql"
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/zalshy/tkt/internal/tui/styles"
)

const sparklineBuckets = 120              // one bucket per minute over 2 hours
const sparklineBucketSec = 7200 / sparklineBuckets // 60 s per bucket

// ── Data ───────────────────────────────────────────────────────────────────

// sparklineData holds per-minute transition counts for the last 2 hours.
// Index 0 = oldest (2 h ago), index len-1 = most recent (now).
type sparklineData struct {
	buckets []int
}

// loadSparkline queries ticket_log for 'transition' events in the last 2 hours
// grouped into sparklineBuckets one-minute buckets.
func loadSparkline(db *sql.DB) (sparklineData, error) {
	empty := sparklineData{buckets: make([]int, sparklineBuckets)}
	if db == nil {
		return empty, nil
	}

	rows, err := db.Query(`
		SELECT
			CAST(
				(strftime('%s', tl.created_at) - strftime('%s', datetime('now', '-2 hours')))
				/ ? AS INTEGER
			) AS bucket,
			COUNT(*) AS cnt
		FROM ticket_log tl
		WHERE tl.kind       = 'transition'
		  AND tl.deleted_at IS NULL
		  AND tl.created_at >= datetime('now', '-2 hours')
		GROUP BY bucket
		ORDER BY bucket
	`, sparklineBucketSec)
	if err != nil {
		return empty, fmt.Errorf("sparkline.loadSparkline: query: %w", err)
	}
	defer rows.Close()

	buckets := make([]int, sparklineBuckets)
	for rows.Next() {
		var b, cnt int
		if err := rows.Scan(&b, &cnt); err != nil {
			return empty, fmt.Errorf("sparkline.loadSparkline: scan: %w", err)
		}
		if b >= 0 && b < sparklineBuckets {
			buckets[b] = cnt
		}
	}
	if err := rows.Err(); err != nil {
		return empty, fmt.Errorf("sparkline.loadSparkline: rows: %w", err)
	}
	return sparklineData{buckets: buckets}, nil
}

// ── Signal processing ──────────────────────────────────────────────────────

// smoothGaussian applies a Gaussian-weighted kernel with the given sigma
// (in bucket units) over src. A Gaussian kernel produces symmetric,
// bell-shaped peaks from clustered events — unlike a box filter which
// gives trapezoidal (flat-topped) shapes. Boundaries are handled by
// clamping to the nearest edge sample.
func smoothGaussian(src []int, sigma float64) []float64 {
	n := len(src)
	dst := make([]float64, n)
	radius := int(math.Ceil(3 * sigma)) // 3σ covers 99.7 % of the bell
	for i := range src {
		sum, weight := 0.0, 0.0
		for j := i - radius; j <= i+radius; j++ {
			k := j
			if k < 0 {
				k = 0
			} else if k >= n {
				k = n - 1
			}
			d := float64(j - i)
			w := math.Exp(-d * d / (2 * sigma * sigma))
			sum += float64(src[k]) * w
			weight += w
		}
		if weight > 0 {
			dst[i] = sum / weight
		}
	}
	return dst
}

// statusPaletteColor returns the colour at position pos ∈ [0..1] along a
// two-stop gradient that mirrors the side-monitor header bar exactly:
//
//	left  (pos=0) → #C678DD  pillBg  (purple-pink title area)
//	right (pos=1) → #56B6C2  clockBg (cyan clock badge)
//
// Every element that uses positional colouring (comet, velocity waveform)
// therefore reads as a continuous extension of the header's own colours.
func statusPaletteColor(pos float64) lipgloss.Color {
	if pos < 0 {
		pos = 0
	}
	if pos > 1 {
		pos = 1
	}
	a := [3]float64{0xC6, 0x78, 0xDD} // #C678DD  pillBg
	b := [3]float64{0x56, 0xB6, 0xC2} // #56B6C2  clockBg
	r := a[0]*(1-pos) + b[0]*pos
	g := a[1]*(1-pos) + b[1]*pos
	bv := a[2]*(1-pos) + b[2]*pos
	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", int(r), int(g), int(bv)))
}

// interpF linearly upsamples (or downsamples) a float64 slice to targetN points.
func interpF(src []float64, targetN int) []float64 {
	dst := make([]float64, targetN)
	N := len(src)
	if N == 0 || targetN == 0 {
		return dst
	}
	if N == 1 {
		for i := range dst {
			dst[i] = src[0]
		}
		return dst
	}
	for i := range dst {
		t := float64(i) * float64(N-1) / float64(targetN-1)
		lo := int(t)
		hi := lo + 1
		if hi >= N {
			dst[i] = src[N-1]
		} else {
			frac := t - float64(lo)
			dst[i] = src[lo]*(1-frac) + src[hi]*frac
		}
	}
	return dst
}

// ── Rendering ──────────────────────────────────────────────────────────────
//
// The waveform uses TWO rows of ▁▂▃▄▅▆▇█ characters for 16 height levels:
//
//   ░░░▁▃▅▇█████▇▅▃▁░░░▁▃▅▇████▇▅▃▁░░   ← top row    (levels 8–15 only)
//   ▁▁▃▅▇████████▇▅▃▁▁▃▅▇████████▇▅▃    ← bottom row  (levels 1–8, full=█)
//
// Height encoding per column h ∈ [0..15]:
//   h == 0          → bottom='▁'(faint), top=''(empty)
//   h ∈ [1..7]      → bottom=bars[h-1],  top=''(empty)
//   h == 8          → bottom='█',         top=''(empty)
//   h ∈ [9..15]     → bottom='█',         top=bars[h-9]
//   h == 16 (cap)   → bottom='█',         top='█'

var bars = []rune("▁▂▃▄▅▆▇█") // 8 characters, index 0..7

// waveChars returns the (bottom, top) block runes for a height in [0..16].
// returns (0, 0) to signal "use faint baseline char" for h == 0.
func waveChars(h int) (bottom, top rune, active bool) {
	if h <= 0 {
		return 0, 0, false
	}
	if h > 16 {
		h = 16
	}
	if h <= 8 {
		return bars[h-1], 0, true // only bottom row active
	}
	return '█', bars[h-9], true // both rows active
}

// renderSparkline renders the VELOCITY box content (4 content lines):
//
//	VELOCITY                               21 transitions
//	░░▁▃▅▇████▇▅▃▁░░▁▃▅▇████████▇▅▃▁░░▁▃▅▇████▇▅▃▁   ← top row
//	▁▁▃▅▇████████▇▅▁▁▃▅▇█████████▇▅▁▁▁▃▅▇███████▇▅▁  ← bottom row
//	-2h                                           now
func renderSparkline(d sparklineData, width int) string {
	var sb strings.Builder

	// — Line 1: title + total count —
	total := 0
	for _, v := range d.buckets {
		total += v
	}
	var countLabel string
	switch {
	case total == 0:
		countLabel = "n/a"
	case total == 1:
		countLabel = "1 transition"
	default:
		countLabel = fmt.Sprintf("%d transitions", total)
	}

	titleStr := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true).Render("VELOCITY")
	countStr := lipgloss.NewStyle().Foreground(styles.Muted).Render(countLabel)
	pad := width - lipgloss.Width(titleStr) - lipgloss.Width(countStr)
	if pad < 1 {
		pad = 1
	}
	sb.WriteString(titleStr)
	sb.WriteString(strings.Repeat(" ", pad))
	sb.WriteString(countStr)
	sb.WriteString("\n")

	// — Lines 2-3: two-row waveform —
	//
	// Pipeline:
	//   1. Float64 moving-average smooth (radius 5 = 11-min window).
	//      Float arithmetic is essential — integer division destroys sparse data.
	//   2. Linear interpolation to exactly `width` display columns.
	//   3. Scale [0..maxVal] → [0..16] height levels.
	//   4. Render top row then bottom row, each `width` characters wide.

	smoothed := smoothGaussian(d.buckets, 8)
	display := interpF(smoothed, width)

	maxVal := 0.0
	for _, v := range display {
		if v > maxVal {
			maxVal = v
		}
	}

	// Pre-compute heights for every column.
	heights := make([]int, width)
	if maxVal > 0 {
		for i, v := range display {
			h := int(v * 16 / maxVal)
			if v > 0 && h == 0 {
				h = 1 // guarantee a visible notch for any non-zero value
			}
			heights[i] = h
		}
	}

	// Status-palette gradient: each column gets a smoothly interpolated colour
	// from todo (left/oldest) → planning → in_progress → done → verified (right/newest).
	faintStyle := lipgloss.NewStyle().Foreground(styles.Faint)
	colStyle := func(i int) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(statusPaletteColor(float64(i) / float64(width)))
	}

	// Top row (only rendered for h > 8).
	for i, h := range heights {
		_, top, active := waveChars(h)
		if !active || top == 0 {
			sb.WriteString(" ")
		} else {
			sb.WriteString(colStyle(i).Render(string(top)))
		}
	}
	sb.WriteString("\n")

	// Bottom row (always rendered; faint baseline for h == 0).
	for i, h := range heights {
		bottom, _, active := waveChars(h)
		if !active {
			sb.WriteString(faintStyle.Render("▁"))
		} else {
			sb.WriteString(colStyle(i).Render(string(bottom)))
		}
	}
	sb.WriteString("\n")

	// — Line 4: time endpoint labels — -2h far left, now far right —
	l1, l3 := "-2h", "now"
	gap := width - len(l1) - len(l3)
	if gap < 1 {
		gap = 1
	}
	sb.WriteString(
		lipgloss.NewStyle().Foreground(styles.Faint).Render(
			l1 + strings.Repeat(" ", gap) + l3,
		),
	)

	return sb.String()
}
