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

// Init satisfies tea.Model. Starts the poll tick, clock tick, and initial stats load.
// Note: Init uses a value receiver — mutations inside Init are discarded.
// statsEpoch is 0 on construction; pass it directly without mutating.
func (m RootModel) Init() tea.Cmd {
	return tea.Batch(
		pollCmd(m.pollInterval),
		clockCmd(),
		loadStatsCmd(m.db, m.statsEpoch),
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
		cmds = append(cmds, pollCmd(m.pollInterval))
		cmds = append(cmds, loadStatsCmd(m.db, m.statsEpoch))
		return m, tea.Batch(cmds...)

	case statsLoadedMsg:
		if msg.epoch == m.statsEpoch {
			m.stats = msg.data
		}
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
func (m RootModel) View() string {
	if m.width < 60 || m.height < 20 {
		errMsg := fmt.Sprintf("Terminal too small (%dx%d)\nMinimum: 60×20", m.width, m.height)
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().Foreground(styles.Danger).Render(errMsg))
	}

	sectionStyle := styles.PanelInactive.
		Width(m.width - 2).
		MarginBottom(1)

	header := renderHeader(m.clock, m.width)

	stats := sectionStyle.Render(renderStats(m.stats, m.width-2))

	changes := sectionStyle.Render(
		lipgloss.NewStyle().Foreground(styles.Muted).Render("[ TICKET CHANGES ]"))

	sessions := sectionStyle.Render(
		lipgloss.NewStyle().Foreground(styles.Muted).Render("[ SESSIONS ]"))

	footer := lipgloss.NewStyle().
		Foreground(styles.Muted).
		Width(m.width).
		Render("  q quit")

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		stats,
		changes,
		sessions,
		footer,
	)
}
