package side

import (
	"database/sql"
	"fmt"
	"strings"
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

// animTickMsg is sent by animCmd on each animation frame (50 ms).
type animTickMsg struct{}

func animCmd() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(time.Time) tea.Msg { return animTickMsg{} })
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

// sessionsLoadedMsg carries freshly-loaded session events and the epoch it
// was requested at. Stale loads (epoch mismatch) are discarded.
type sessionsLoadedMsg struct {
	events []sessionEvent
	epoch  int
}

// tokenBurnLoadedMsg carries freshly-loaded token burn totals and the epoch
// it was requested at. Stale loads (epoch mismatch) are discarded.
type tokenBurnLoadedMsg struct {
	data  tokenBurnData
	epoch int
}

// sparklineLoadedMsg carries freshly-loaded sparkline bucket data and the
// epoch it was requested at. Stale loads (epoch mismatch) are discarded.
type sparklineLoadedMsg struct {
	data  sparklineData
	epoch int
}

// RootModel is the top-level BubbleTea model for the side monitor mode.
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
	sessEpoch    int
	sessLoaded   bool // true after the first successful sessions load (baseline)
	tokenBurn      tokenBurnData
	burnEpoch      int
	sparkline      sparklineData
	sparklineEpoch int
	// Comet animation state — ping-pong left↔right.
	cometPos      float64 // 0..1 position of the head along the bar
	cometDir      int     // +1 = left→right, -1 = right→left (true direction)
	cometDirBlend float64 // smoothed direction in [-1..+1]; lags cometDir to soften reversals
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
		cometDir:      1,
		cometDirBlend: 1.0, // starts moving left→right
	}
}

// loadStatsCmd returns a tea.Cmd that loads stats in the background and sends
// a statsLoadedMsg tagged with the given epoch.
func loadStatsCmd(db *sql.DB, epoch int) tea.Cmd {
	return func() tea.Msg {
		data, err := loadStats(db)
		if err != nil {
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
		events, err := loadSessions(db)
		if err != nil {
			return sessionsLoadedMsg{epoch: epoch}
		}
		return sessionsLoadedMsg{events: events, epoch: epoch}
	}
}

// loadTokenBurnCmd returns a tea.Cmd that loads token burn totals in the
// background and sends a tokenBurnLoadedMsg tagged with the given epoch.
func loadTokenBurnCmd(db *sql.DB, epoch int) tea.Cmd {
	return func() tea.Msg {
		data, err := loadTokenBurn(db)
		if err != nil {
			return tokenBurnLoadedMsg{epoch: epoch}
		}
		return tokenBurnLoadedMsg{data: data, epoch: epoch}
	}
}

// loadSparklineCmd returns a tea.Cmd that loads the velocity sparkline data in
// the background and sends a sparklineLoadedMsg tagged with the given epoch.
func loadSparklineCmd(db *sql.DB, epoch int) tea.Cmd {
	return func() tea.Msg {
		data, err := loadSparkline(db)
		if err != nil {
			return sparklineLoadedMsg{data: sparklineData{buckets: make([]int, sparklineBuckets)}, epoch: epoch}
		}
		return sparklineLoadedMsg{data: data, epoch: epoch}
	}
}

// Init satisfies tea.Model. Starts the poll tick, clock tick, and initial data loads.
// Note: Init uses a value receiver — mutations inside Init are discarded.
func (m RootModel) Init() tea.Cmd {
	return tea.Batch(
		pollCmd(m.pollInterval),
		clockCmd(),
		animCmd(),
		loadStatsCmd(m.db, m.statsEpoch),
		loadFeedCmd(m.db, m.feedEpoch),
		loadSessionsCmd(m.db, m.sessEpoch),
		loadTokenBurnCmd(m.db, m.burnEpoch),
		loadSparklineCmd(m.db, m.sparklineEpoch),
	)
}

// Update handles window resize, poll ticks, clock ticks, data loads, and quit keys.
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
		cmds = append(cmds, tea.ClearScreen)
		return m, tea.Batch(cmds...)

	case pollTickMsg:
		m.statsEpoch++
		m.feedEpoch++
		m.sessEpoch++
		m.burnEpoch++
		m.sparklineEpoch++
		cmds = append(cmds, pollCmd(m.pollInterval))
		cmds = append(cmds, loadStatsCmd(m.db, m.statsEpoch))
		cmds = append(cmds, loadFeedCmd(m.db, m.feedEpoch))
		cmds = append(cmds, loadSessionsCmd(m.db, m.sessEpoch))
		cmds = append(cmds, loadTokenBurnCmd(m.db, m.burnEpoch))
		cmds = append(cmds, loadSparklineCmd(m.db, m.sparklineEpoch))
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
			m.feedLoaded = true
		} else {
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
			m.sessLoaded = true
		} else {
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
		return m, tea.Batch(cmds...)

	case tokenBurnLoadedMsg:
		if msg.epoch == m.burnEpoch {
			m.tokenBurn = msg.data
		}
		return m, tea.Batch(cmds...)

	case sparklineLoadedMsg:
		if msg.epoch == m.sparklineEpoch {
			m.sparkline = msg.data
		}
		return m, tea.Batch(cmds...)

	case animTickMsg:
		// Comet ping-pong: ~8 s to cross the full width (160 × 50 ms ticks).
		// cometDir flips instantly at each edge; cometDirBlend lerps toward it
		// at 25 % per tick (~300 ms half-transition) so the tail blends smoothly
		// from one side to the other instead of snapping.
		const speedPerTick = 1.0 / 160.0
		const blendRate = 0.25
		m.cometPos += speedPerTick * float64(m.cometDir)
		if m.cometPos >= 1.0 {
			m.cometPos = 1.0
			m.cometDir = -1
		} else if m.cometPos <= 0.0 {
			m.cometPos = 0.0
			m.cometDir = 1
		}
		target := float64(m.cometDir)
		m.cometDirBlend += (target - m.cometDirBlend) * blendRate
		cmds = append(cmds, animCmd())
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC || msg.String() == "q" {
			return m, tea.Quit
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the side monitor layout:
//
//	header
//	[By Status] [By Attention] [By Type]          ← three equal stat boxes
//	[SESSIONS    ] [TICKET CHANGES             ]  ← 1/3 + 2/3 width
//	[TOKEN BURN  ]                                ← under sessions, same width
//	footer
//
// Everything is sized to fit m.height — no scrolling.
func (m RootModel) View() string {
	if m.width < 50 || m.height < 16 {
		errMsg := fmt.Sprintf("Terminal too small (%dx%d)\nMinimum: 50×16", m.width, m.height)
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().Foreground(styles.Danger).Render(errMsg))
	}

	// — Header —
	header := renderHeader(m.clock, m.width)
	headerH := lipgloss.Height(header)

	// — Footer —
	footer := lipgloss.NewStyle().
		Foreground(styles.Muted).
		Width(m.width).
		Render("  q quit")
	footerH := lipgloss.Height(footer)

	// — Stats row (3 boxes, fixed height) —
	statsRow := renderStatsRow(m.stats, m.width)
	statsH := lipgloss.Height(statsRow)

	// — Comet bar — borderless, full width, single row —
	const cometBoxH = 1
	cometBox := strings.TrimRight(renderCometBar(m.cometPos, m.cometDirBlend, m.width), "\n")

	// — Bottom area —
	bottomH := max(m.height-headerH-statsH-cometBoxH-footerH, 2)

	// Left column: 1/3 width. Feed: 2/3 width.
	sessW := m.width / 3
	feedW := m.width - sessW

	// Bottom boxes are 6 rendered rows each (title + content + border = 6).
	// Both the token burn (left) and velocity sparkline (right) use this height.
	const smallBoxH = 6

	// Decide how much space the small boxes (burn / sparkline) get.
	// They need at least 3 rows each (1 content + 2 border).
	const minSmallH = 3
	smallH := min(smallBoxH, max(bottomH-minSmallH, minSmallH))
	smallContentH := max(smallH-2, 1)

	// Left column: sessions fills the space above token burn.
	sessRenderedH := max(bottomH-smallH, minSmallH)
	sessContentH := max(sessRenderedH-2, 1)

	// Right column: feed fills the space above velocity sparkline.
	feedRenderedH := max(bottomH-smallH, minSmallH)
	feedContentH := max(feedRenderedH-2, 1)

	// maxEntries: feed content = 1 title line + N entry lines.
	maxEntries := max(feedContentH-1, 1)

	sessBox := styles.PanelInactive.
		Width(sessW - 2).
		Height(sessContentH).
		Render(strings.TrimRight(renderSessions(m.sessionsData, sessW-2), "\n"))

	burnContentW := max(1, sessW-2)
	burnBox := styles.PanelInactive.
		Width(burnContentW).
		Height(smallContentH).
		Render(strings.TrimRight(renderTokenBurn(m.tokenBurn, burnContentW), "\n"))

	feedBox := styles.PanelInactive.
		Width(feedW - 2).
		Height(feedContentH).
		Render(strings.TrimRight(renderFeed(m.feed, feedW-2, maxEntries), "\n"))

	sparklineBox := styles.PanelInactive.
		Width(feedW - 2).
		Height(smallContentH).
		Render(strings.TrimRight(renderSparkline(m.sparkline, feedW-2), "\n"))

	smallRow := burnBox
	leftCol := lipgloss.JoinVertical(lipgloss.Left, sessBox, smallRow)
	rightCol := lipgloss.JoinVertical(lipgloss.Left, feedBox, sparklineBox)
	bottomRow := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, rightCol)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		statsRow,
		cometBox,
		bottomRow,
		footer,
	)
}
