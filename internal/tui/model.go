package tui

import (
	"database/sql"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zalshy/tkt/internal/config"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/tui/footer"
	"github.com/zalshy/tkt/internal/tui/header"
	"github.com/zalshy/tkt/internal/tui/help"
	"github.com/zalshy/tkt/internal/tui/kanban"
	"github.com/zalshy/tkt/internal/tui/modal"
	"github.com/zalshy/tkt/internal/tui/search"
	"github.com/zalshy/tkt/internal/tui/styles"
	"github.com/zalshy/tkt/internal/tui/ticketdetail"
	"github.com/zalshy/tkt/internal/tui/toast"
)

const headerHeight = 2
const footerHeight = 1

// pollTickMsg is sent by pollCmd on each poll interval.
type pollTickMsg struct{}

// pollCmd returns a tea.Cmd that fires after d, sending a pollTickMsg.
func pollCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return pollTickMsg{}
	})
}

// RootModel is the top-level BubbleTea model that owns layout, size management,
// and Kanban board state. All child components are mounted from here.
type RootModel struct {
	db    *sql.DB
	cfg   *config.ProjectConfig
	root  string
	width int
	height int

	// Child components
	board  kanban.Board
	detail ticketdetail.Model
	search search.Model
	hdr    header.Model // "hdr" to avoid collision with the "header" import
	ftr    footer.Model // "ftr" to avoid collision with the "footer" import

	// Layout / interaction state
	epoch        int           // monotonically increasing; tags each LoadCmd call
	tickN        int           // comet animation tick counter
	pollInterval time.Duration // how often to auto-refresh board data

	// Full unfiltered ticket list, needed to re-apply search.Filter after query changes.
	allTickets []models.Ticket

	// modals holds the active overlay state. Named "modals" (not "modal") to avoid
	// shadowing the modal package identifier inside RootModel methods.
	modals modal.Manager
}

// NewRootModel constructs a RootModel with zero-valued layout fields.
// The header is initialised here (not in Init) because Init uses a value
// receiver — any assignments inside Init are silently discarded.
// No I/O is performed.
func NewRootModel(db *sql.DB, cfg *config.ProjectConfig, root string) RootModel {
	interval := time.Duration(cfg.MonitorInterval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return RootModel{
		db:           db,
		cfg:          cfg,
		root:         root,
		hdr:          header.New(0, 0),
		board:        kanban.New(0, 0),
		detail:       ticketdetail.New(0, 0, false),
		search:       search.New(0),
		ftr:          footer.New(0, footer.ContextList),
		pollInterval: interval,
	}
}

// Init satisfies tea.Model. Initialises child components and kicks off the
// header animation and the initial board load.
func (m RootModel) Init() tea.Cmd {
	return tea.Batch(
		header.InitCmd(),
		kanban.LoadCmd(m.db, m.epoch),
		kanban.TickCmd(),
		pollCmd(m.pollInterval),
	)
}

// Update handles terminal resize and quit key events plus all child component
// message routing.
func (m RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// R1 — Forward ALL messages to the header unconditionally.
	// header.TickMsg is exported but we still forward all messages so the header
	// receives every message first for its animation ticks.
	var hdrCmd tea.Cmd
	m.hdr, hdrCmd = m.hdr.Update(msg)
	if hdrCmd != nil {
		cmds = append(cmds, hdrCmd)
	}

	// R2 — Forward kanban tick messages so the comet animation self-schedules.
	if _, ok := msg.(kanban.TickMsg); ok {
		m.tickN++
		return m, kanban.TickCmd()
	}

	// R3 — Poll tick: refresh board data and re-schedule.
	if _, ok := msg.(pollTickMsg); ok {
		return m, tea.Batch(
			kanban.LoadCmd(m.db, m.epoch),
			pollCmd(m.pollInterval),
		)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		contentHeight := m.height - headerHeight - footerHeight

		m.hdr = m.hdr.SetWidth(m.width)
		m.board = m.board.SetSize(m.width, contentHeight)
		m.detail = m.detail.SetSize(m.width, contentHeight)
		m.search = m.search.SetWidth(m.width)
		m.ftr = m.ftr.SetWidth(m.width)

		if m.modals.WidthFor(modal.KindHelp) != 0 && m.modals.WidthFor(modal.KindHelp) != m.width {
			m.modals = m.modals.Show(modal.KindHelp, help.Render(m.width), m.width)
		}
		if m.modals.WidthFor(modal.KindToast) != 0 {
			m.modals = m.modals.Dismiss(modal.KindToast)
		}

		return m, tea.Batch(cmds...)

	case kanban.BoardLoadedMsg:
		if msg.Err != nil {
			toastContent := toast.Render("Board load failed: "+msg.Err.Error(), toast.Error, m.width)
			m.modals = m.modals.Show(modal.KindToast, toastContent, m.width)
			return m, tea.Batch(append(cmds, toast.ExpireCmd())...)
		}
		m.allTickets = msg.Tickets
		if m.search.IsActive() {
			allForCol := ticketsForVisualCol(msg.Tickets, m.board.ActiveStatus())
			filtered := m.search.Filter(allForCol)
			boardTickets := replaceStatusTickets(msg.Tickets, m.board.ActiveStatus(), filtered)
			m.board = m.board.SetTickets(boardTickets)
		} else {
			m.board = m.board.SetTickets(msg.Tickets)
		}
		return m, tea.Batch(cmds...)

	case ticketdetail.DetailLoadedMsg:
		var dCmd tea.Cmd
		m.detail, dCmd = m.detail.Update(msg) // ticketdetail handles epoch guard internally
		if dCmd != nil {
			cmds = append(cmds, dCmd)
		}
		// Refresh detail modal content if it's currently open
		if k, _ := m.modals.Active(); k == modal.KindDetail {
			m.modals = m.modals.Show(modal.KindDetail, m.detail.View(), m.width)
		}
		return m, tea.Batch(cmds...)

	case toast.ToastExpiredMsg:
		m.modals = m.modals.Dismiss(modal.KindToast)

	case tea.KeyMsg:
		// Quit keys — handled before anything else.
		if msg.Type == tea.KeyCtrlC || (msg.String() == "q" && !m.search.IsActive()) {
			return m, tea.Quit
		}

		switch {
		case msg.Type == tea.KeyEsc && m.modals.HasActive():
			kind, _ := m.modals.Active()
			m.modals = m.modals.Dismiss(kind)
			if kind == modal.KindDetail {
				m.detail = m.detail.SetFocus(false)
			}

		case msg.Type == tea.KeyEsc && m.search.IsActive():
			m.search = m.search.Close()
			m.board = m.board.SetTickets(m.allTickets)
			m.ftr = m.ftr.SetContext(footerCtx(m))

		case msg.String() == "?" && !m.search.IsActive():
			m.modals = m.modals.Show(modal.KindHelp, help.Render(m.width), m.width)

		case msg.String() == "/" && !m.search.IsActive():
			m.search = m.search.Open()
			m.ftr = m.ftr.SetContext(footer.ContextSearch)

		// Column navigation: left/right arrows OR h/l vim keys.
		case isColNav(msg) && !m.search.IsActive() && !m.modals.HasActive():
			m.board, _ = m.board.Update(msg)
			m.hdr = m.hdr.SetActiveTab(m.board.ActiveCol())
			m.ftr = m.ftr.SetContext(footerCtx(m))

		// Scroll the detail modal with j/k/↑/↓ when it is the active overlay.
		case isCursorNav(msg) && !m.search.IsActive() && m.modals.HasActive():
			if kind, _ := m.modals.Active(); kind == modal.KindDetail {
				var dCmd tea.Cmd
				m.detail, dCmd = m.detail.Update(msg)
				if dCmd != nil {
					cmds = append(cmds, dCmd)
				}
				// Refresh modal content after scroll.
				m.modals = m.modals.Show(modal.KindDetail, m.detail.View(), m.width)
			}

		// Row navigation within the active column: j/k vim OR ↑/↓ arrows.
		case isCursorNav(msg) && !m.search.IsActive() && !m.modals.HasActive():
			m.board, _ = m.board.Update(msg)

		case msg.Type == tea.KeyEnter && !m.search.IsActive() && !m.modals.HasActive():
			if t := m.board.SelectedTicket(); t != nil {
				m.epoch++
				m.detail = m.detail.SetTicket(t, m.epoch).SetFocus(true)
				dCmd := ticketdetail.LoadCmd(m.db, t.ID, m.epoch)
				cmds = append(cmds, dCmd)
				m.modals = m.modals.Show(modal.KindDetail, m.detail.View(), m.width)
			}

		case msg.Type == tea.KeyEnter && m.search.IsActive():
			// search select — do nothing for now (search just filters the board)

		default:
			if m.search.IsActive() {
				var sCmd tea.Cmd
				m.search, sCmd = m.search.Update(msg)
				// Filter the active column only
				allForCol := ticketsForVisualCol(m.allTickets, m.board.ActiveStatus())
				filtered := m.search.Filter(allForCol)
				// Rebuild board with filtered active column
				boardTickets := replaceStatusTickets(m.allTickets, m.board.ActiveStatus(), filtered)
				m.board = m.board.SetTickets(boardTickets)
				if sCmd != nil {
					cmds = append(cmds, sCmd)
				}
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the root layout. If the terminal is smaller than 60×20 it
// renders a centred size-guard error instead of the normal chrome.
func (m RootModel) View() string {
	// Size guard — must be checked first.
	if m.width < 60 || m.height < 20 {
		errMsg := fmt.Sprintf("Terminal too small (%dx%d)\nMinimum: 60×20", m.width, m.height)
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().Foreground(styles.Danger).Render(errMsg))
	}

	contentHeight := m.height - headerHeight - footerHeight
	headerView := m.hdr.View()

	boardView := m.board.SetTickN(m.tickN).View()

	if m.search.IsActive() {
		boardView = m.search.View() + "\n" + boardView
	}

	if m.modals.HasActive() {
		_, content := m.modals.Active()
		boardView = modal.Overlay(content, m.width, contentHeight)
	}

	footerView := m.ftr.SetWidth(m.width).View()
	return headerView + "\n" + boardView + "\n" + footerView
}

// footerCtx derives the appropriate footer.Context from current model state.
func footerCtx(m RootModel) footer.Context {
	if m.modals.HasActive() {
		return footer.ContextModal
	}
	if m.search.IsActive() {
		return footer.ContextSearch
	}
	return footer.ContextList
}

// isColNav reports whether msg is a column-navigation key (left/right/h/l).
func isColNav(msg tea.KeyMsg) bool {
	s := msg.String()
	return s == "left" || s == "right" || s == "h" || s == "l"
}

// isCursorNav reports whether msg is a row-navigation key (up/down/k/j).
func isCursorNav(msg tea.KeyMsg) bool {
	s := msg.String()
	return s == "j" || s == "k" || msg.Type == tea.KeyUp || msg.Type == tea.KeyDown
}

// ticketsForVisualCol returns all tickets that visually belong to the column
// identified by status. Mirrors the bucketing in kanban/board.go SetTickets:
//
//	StatusInProgress → StatusPlanning column
//	StatusVerified   → StatusDone column
//
// Must stay in sync with board.go SetTickets bucketing logic.
func ticketsForVisualCol(all []models.Ticket, status models.Status) []models.Ticket {
	var out []models.Ticket
	for _, t := range all {
		visual := t.Status
		if visual == models.StatusInProgress {
			visual = models.StatusPlanning
		} else if visual == models.StatusVerified {
			visual = models.StatusDone
		}
		if visual == status {
			out = append(out, t)
		}
	}
	return out
}

// replaceStatusTickets rebuilds the ticket list replacing all tickets that
// visually belong to the given column with replacement. Uses same bucketing
// as ticketsForVisualCol — must stay in sync with board.go SetTickets.
func replaceStatusTickets(all []models.Ticket, status models.Status, replacement []models.Ticket) []models.Ticket {
	out := make([]models.Ticket, 0, len(all))
	for _, t := range all {
		visual := t.Status
		if visual == models.StatusInProgress {
			visual = models.StatusPlanning
		} else if visual == models.StatusVerified {
			visual = models.StatusDone
		}
		if visual != status {
			out = append(out, t)
		}
	}
	return append(out, replacement...)
}
