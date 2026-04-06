package tui

import (
	"database/sql"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/zalshy/tkt/internal/config"
	ilog "github.com/zalshy/tkt/internal/log"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/ticket"
)

// Model is the root BubbleTea model for the read-only kanban monitor.
type Model struct {
	db          *sql.DB
	projectPath string
	width       int
	height      int

	// columns holds tickets bucketed by status index:
	// 0=TODO, 1=PLANNING, 2=IN_PROGRESS, 3=DONE, 4=VERIFIED, 5=CANCELED
	columns [6][]models.Ticket

	// cursor position
	colIdx int
	rowIdx int

	showCanceled bool

	// side panel state
	showPanel   bool
	panelLogs   []models.LogEntry
	panelPlan   *models.LogEntry
	panelTicket models.Ticket

	interval    time.Duration
	lastErr     error
	lastRefresh time.Time
}

// tickMsg is sent on each poll interval.
type tickMsg time.Time

// dataMsg carries the result of a background ticket fetch.
type dataMsg struct {
	columns [6][]models.Ticket
	err     error
}

// panelDataMsg carries the result of loading a ticket's log entries for the side panel.
type panelDataMsg struct {
	logs []models.LogEntry
	plan *models.LogEntry
	err  error
}

// New constructs a new Model. Caller must pass the result to tea.NewProgram.
func New(db *sql.DB, cfg *config.ProjectConfig, projectPath string) Model {
	interval := 2 * time.Second
	if cfg != nil && cfg.MonitorInterval > 0 {
		interval = time.Duration(cfg.MonitorInterval) * time.Second
	}
	return Model{
		db:          db,
		projectPath: projectPath,
		interval:    interval,
	}
}

// Init starts the first data fetch and the poll ticker.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadData(), m.scheduleTick())
}

// loadData returns a tea.Cmd that fetches all tickets and buckets them.
func (m Model) loadData() tea.Cmd {
	db := m.db
	return func() tea.Msg {
		result, err := ticket.List(ticket.ListOptions{All: true, IncludeVerified: true}, db)
		if err != nil {
			return dataMsg{err: err}
		}
		return dataMsg{columns: bucketTickets(result.Tickets)}
	}
}

// scheduleTick returns a tea.Cmd that fires once after m.interval.
// Update restarts it on each tick — this avoids drift from accumulating ticks.
func (m Model) scheduleTick() tea.Cmd {
	d := m.interval
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// panelLoad returns a tea.Cmd that loads log data for the given ticket.
func panelLoad(db *sql.DB, ticketID int64) tea.Cmd {
	idStr := strconv.FormatInt(ticketID, 10)
	return func() tea.Msg {
		logs, err := ilog.GetAll(idStr, db)
		if err != nil {
			return panelDataMsg{err: err}
		}
		plan, err := ilog.LatestPlan(idStr, db)
		if err != nil {
			return panelDataMsg{err: err}
		}
		return panelDataMsg{logs: logs, plan: plan}
	}
}

// bucketTickets groups a flat ticket slice into the 6-slot array by status.
// Index mapping: 0=TODO, 1=PLANNING, 2=IN_PROGRESS, 3=DONE, 4=VERIFIED, 5=CANCELED.
func bucketTickets(tickets []models.Ticket) [6][]models.Ticket {
	var cols [6][]models.Ticket
	for _, t := range tickets {
		idx := statusIndex(t.Status)
		cols[idx] = append(cols[idx], t)
	}
	return cols
}

// statusIndex maps a Status to its column index.
func statusIndex(s models.Status) int {
	switch s {
	case models.StatusTodo:
		return 0
	case models.StatusPlanning:
		return 1
	case models.StatusInProgress:
		return 2
	case models.StatusDone:
		return 3
	case models.StatusVerified:
		return 4
	case models.StatusCanceled:
		return 5
	default:
		return 0
	}
}

// clampCursor ensures colIdx and rowIdx stay within valid ranges given the
// current columns content and showCanceled flag.
func clampCursor(colIdx, rowIdx int, columns [6][]models.Ticket, showCanceled bool) (int, int) {
	visible := visibleCols(showCanceled)
	numCols := len(visible)

	if numCols == 0 {
		return 0, 0
	}

	if colIdx < 0 {
		colIdx = 0
	}
	if colIdx >= numCols {
		colIdx = numCols - 1
	}

	col := visible[colIdx]
	colLen := len(columns[col])

	if colLen == 0 {
		return colIdx, 0
	}

	if rowIdx < 0 {
		rowIdx = 0
	}
	if rowIdx >= colLen {
		rowIdx = colLen - 1
	}

	return colIdx, rowIdx
}

// visibleCols returns the column indices that should be shown.
func visibleCols(showCanceled bool) []int {
	if showCanceled {
		return []int{0, 1, 2, 3, 4, 5}
	}
	return []int{0, 1, 2, 3, 4}
}

// truncateTitle returns the title truncated to maxRunes runes.
// If the title is shorter or equal to maxRunes, it is returned unchanged.
func truncateTitle(title string, maxRunes int) string {
	runes := []rune(title)
	if len(runes) <= maxRunes {
		return title
	}
	return string(runes[:maxRunes])
}
