package side

import (
	"database/sql"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zalshy/tkt/internal/config"
	"github.com/zalshy/tkt/internal/tui/styles"
)

// pollTickMsg is sent by pollCmd on each poll interval.
type pollTickMsg struct{}

func pollCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return pollTickMsg{} })
}

// statsLoadedMsg carries a freshly-computed statsData and the epoch it was
// requested at. Stale loads (epoch mismatch) are discarded.
type statsLoadedMsg struct {
	data  statsData
	epoch int
}

// feedLoadedMsg carries freshly-loaded feed entries and the epoch it was
// requested at. Stale loads (epoch mismatch) are discarded.
type feedLoadedMsg struct {
	entries []feedEntry
	epoch   int
}

// sessionsLoadedMsg carries freshly-loaded session events and counts and the
// epoch it was requested at. Stale loads (epoch mismatch) are discarded.
type sessionsLoadedMsg struct {
	events []sessionEvent
	counts sessionCounts
	epoch  int
}

// RootModel is the top-level BubbleTea model for the side monitor mode.
// It renders three sections (STATS, TICKET CHANGES, SESSIONS)
// with a minimal single-line header (including a HH:MM clock) and footer.
type RootModel struct {
	db           *sql.DB
	cfg          *config.ProjectConfig
	root         string
	width        int
	height       int
	pollInterval time.Duration
	clock        clockModel
	stats        statsData
	statsEpoch   int
	feed         []feedEntry
	feedEpoch    int
	feedLoaded   bool // true after the first successful feed load (baseline)
	sessionsData []sessionEvent
	sessCounts   sessionCounts
	sessEpoch    int
	sessLoaded   bool // true after the first successful sessions load (baseline)
}

// NewRootModel constructs a RootModel. It reads cfg.MonitorInterval for the
// poll interval (default 5s). cfg may be nil.
func NewRootModel(db *sql.DB, cfg *config.ProjectConfig, root string) RootModel {
	var interval time.Duration
	if cfg != nil {
		interval = time.Duration(cfg.MonitorInterval) * time.Second
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return RootModel{
		db:           db,
		cfg:          cfg,
		root:         root,
		pollInterval: interval,
		clock:        newClockModel(),
	}
}

// loadStatsCmd returns a tea.Cmd that loads stats in the background and sends
// a statsLoadedMsg tagged with the given epoch.
func loadStatsCmd(db *sql.DB, epoch int) tea.Cmd {
	return func() tea.Msg {
		data, err := loadStats(db)
		if err != nil {
			// Non-fatal: return empty statsData. The panel will show "loading…"
			// until the next successful poll rather than crashing the TUI.
			return statsLoadedMsg{data: statsData{}, epoch: epoch}
		}
		return statsLoadedMsg{data: data, epoch: epoch}
	}
}

// loadFeedCmd returns a tea.Cmd that loads the ticket changes feed in the
// background and sends a feedLoadedMsg tagged with the given epoch.
func loadFeedCmd(db *sql.DB, epoch int) tea.Cmd {
	return func() tea.Msg {
		entries, err := loadFeed(db)
		if err != nil {
			return feedLoadedMsg{entries: nil, epoch: epoch}
		}
		return feedLoadedMsg{entries: entries, epoch: epoch}
	}
}

// loadSessionsCmd returns a tea.Cmd that loads active sessions in the
// background and sends a sessionsLoadedMsg tagged with the given epoch.
func loadSessionsCmd(db *sql.DB, epoch int) tea.Cmd {
	return func() tea.Msg {
		events, counts, err := loadSessions(db)
		if err != nil {
			return sessionsLoadedMsg{epoch: epoch}
		}
		return sessionsLoadedMsg{events: events, counts: counts, epoch: epoch}
	}
}

// Init satisfies tea.Model. Starts the poll tick, clock tick, and initial
// stats/feed/sessions load.
// Note: Init uses a value receiver — mutations inside Init are discarded.
// All epoch values are 0 on construction; pass them directly without mutating.
func (m RootModel) Init() tea.Cmd {
	return tea.Batch(
		pollCmd(m.pollInterval),
		clockCmd(),
		loadStatsCmd(m.db, m.statsEpoch),
		loadFeedCmd(m.db, m.feedEpoch),
		loadSessionsCmd(m.db, m.sessEpoch),
	)
}

// Update handles window resize, poll ticks, clock ticks, stats loads, and quit keys.
func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Forward every message to the clock sub-model.
	var clockC tea.Cmd
	m.clock, clockC = m.clock.update(msg)
	if clockC != nil {
		cmds = append(cmds, clockC)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, tea.Batch(cmds...)

	case pollTickMsg:
		m.statsEpoch++
		m.feedEpoch++
		m.sessEpoch++
		cmds = append(cmds, pollCmd(m.pollInterval))
		cmds = append(cmds, loadStatsCmd(m.db, m.statsEpoch))
		cmds = append(cmds, loadFeedCmd(m.db, m.feedEpoch))
		cmds = append(cmds, loadSessionsCmd(m.db, m.sessEpoch))
		return m, tea.Batch(cmds...)

	case statsLoadedMsg:
		if msg.epoch == m.statsEpoch {
			m.stats = msg.data
		}
		return m, tea.Batch(cmds...)

	case feedLoadedMsg:
		if msg.epoch != m.feedEpoch {
			return m, tea.Batch(cmds...)
		}
		if !m.feedLoaded {
			// First load establishes the baseline — no entries are "new" yet.
			m.feedLoaded = true
		} else {
			// Subsequent loads: mark entries newer than previous latest as new.
			var latest time.Time
			for _, e := range m.feed {
				if e.createdAt.After(latest) {
					latest = e.createdAt
				}
			}
			for i := range msg.entries {
				if msg.entries[i].createdAt.After(latest) {
					msg.entries[i].arrivedAt = time.Now()
				}
			}
		}
		m.feed = msg.entries
		return m, tea.Batch(cmds...)

	case sessionsLoadedMsg:
		if msg.epoch != m.sessEpoch {
			return m, tea.Batch(cmds...)
		}
		if !m.sessLoaded {
			// First load establishes the baseline — no sessions are "new" yet.
			m.sessLoaded = true
		} else {
			// Subsequent loads: mark sessions not seen before as new.
			existing := make(map[string]bool, len(m.sessionsData))
			for _, e := range m.sessionsData {
				existing[e.name] = true
			}
			for i := range msg.events {
				if !existing[msg.events[i].name] {
					msg.events[i].arrivedAt = time.Now()
				}
			}
		}
		m.sessionsData = msg.events
		m.sessCounts = msg.counts
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC || msg.String() == "q" {
			return m, tea.Quit
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the side monitor layout. If the terminal is smaller than 60×20
// it renders a centred size-guard error instead of the normal layout.
//
// The layout is height-aware: stats and sessions are fixed-height, and the
// ticket changes feed receives whatever rows remain so the whole page always
// fits in the viewport without scrolling.
func (m RootModel) View() string {
	if m.width < 60 || m.height < 20 {
		errMsg := fmt.Sprintf("Terminal too small (%dx%d)\nMinimum: 60×20", m.width, m.height)
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().Foreground(styles.Danger).Render(errMsg))
	}

	innerWidth := m.width - 2 // subtract border columns

	// sectionStyle adds a rounded border + 1-row bottom margin.
	sectionStyle := styles.PanelInactive.
		Width(innerWidth).
		MarginBottom(1)

	// Render fixed-height sections first so we can measure them.
	header := renderHeader(m.clock, m.width)
	statsSection := sectionStyle.Render(renderStats(m.stats, innerWidth))
	sessionsSection := sectionStyle.Render(renderSessions(m.sessionsData, m.sessCounts, innerWidth))
	footer := lipgloss.NewStyle().
		Foreground(styles.Muted).
		Width(m.width).
		Render("  q quit")

	// Calculate how many rows are already consumed.
	used := lipgloss.Height(header) +
		lipgloss.Height(statsSection) +
		lipgloss.Height(sessionsSection) +
		lipgloss.Height(footer)

	// Give the remainder to the feed; minimum 3 rows (header + 1 entry + border).
	feedRows := m.height - used
	if feedRows < 3 {
		feedRows = 3
	}

	// maxEntries = feedRows minus the section frame (border top+bottom = 2,
	// section header line = 1, bottom margin = 1 → 4 fixed rows in the section).
	maxEntries := feedRows - 4
	if maxEntries < 1 {
		maxEntries = 1
	}

	changesSection := sectionStyle.
		Height(feedRows - 1). // -1 for the MarginBottom counted in sectionStyle
		Render(renderFeed(m.feed, innerWidth, maxEntries))

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		statsSection,
		changesSection,
		sessionsSection,
		footer,
	)
}
