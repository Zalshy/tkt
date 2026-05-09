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

// RootModel is the top-level BubbleTea model for the side monitor mode.
// It renders three placeholder sections (STATS, TICKET CHANGES, SESSIONS)
// with a minimal single-line header (including a HH:MM clock) and footer.
// Child components are added in subsequent tickets (#203–#204).
type RootModel struct {
	db           *sql.DB
	cfg          *config.ProjectConfig
	root         string
	width        int
	height       int
	pollInterval time.Duration
	clock        clockModel
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

// Init satisfies tea.Model. Starts the poll tick and the clock tick.
func (m RootModel) Init() tea.Cmd {
	return tea.Batch(pollCmd(m.pollInterval), clockCmd())
}

// Update handles window resize, poll ticks, clock ticks, and quit keys.
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
		cmds = append(cmds, pollCmd(m.pollInterval))
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

	stats := sectionStyle.Render(
		lipgloss.NewStyle().Foreground(styles.Muted).Render("[ STATS ]"))

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
