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
// with a minimal single-line header and footer. Child components are added
// in subsequent tickets (#202–#204).
type RootModel struct {
	db           *sql.DB
	cfg          *config.ProjectConfig
	root         string
	width        int
	height       int
	pollInterval time.Duration
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
	}
}

// Init satisfies tea.Model. Starts the poll tick — no board or animation init.
func (m RootModel) Init() tea.Cmd {
	return pollCmd(m.pollInterval)
}

// Update handles window resize, poll ticks, and quit keys.
func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case pollTickMsg:
		return m, pollCmd(m.pollInterval)

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC || msg.String() == "q" {
			return m, tea.Quit
		}
	}

	return m, nil
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

	header := lipgloss.NewStyle().
		Foreground(styles.Primary).
		Bold(true).
		Width(m.width).
		Render("  tkt side")

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
