package ticketdetail

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/glamour"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/zalshy/tkt/internal/log"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/tui/styles"
)

// DetailLoadedMsg is the async result returned by LoadCmd.
type DetailLoadedMsg struct {
	Logs  []models.LogEntry
	Plan  *models.LogEntry
	Epoch int
	Err   error
}

// Model is the ticket detail component. All methods use value receivers and
// return updated copies — callers must replace their stored model with the
// return value.
type Model struct {
	ticket        *models.Ticket
	logs          []models.LogEntry
	plan          *models.LogEntry
	offset        int
	focused       bool
	width         int
	height        int
	epoch         int
	rendered      string                // cached glamour output; rebuilt on SetTicket, SetDetail, SetSize
	renderer      *glamour.TermRenderer // cached renderer; reused when width unchanged
	rendererWidth int                   // width the cached renderer was built for
}

// New constructs a Model with the given dimensions and focus state.
func New(width, height int, focused bool) Model {
	return Model{
		width:   width,
		height:  height,
		focused: focused,
	}
}

// SetTicket replaces the current ticket, clears logs and plan, resets offset to
// 0, and rebuilds the rendered cache.
func (m Model) SetTicket(t *models.Ticket, epoch int) Model {
	m.ticket = t
	m.logs = nil
	m.plan = nil
	m.offset = 0
	m.epoch = epoch
	m = m.buildRendered()
	return m
}

// SetDetail stores log entries and the plan entry then rebuilds the rendered
// cache. It does NOT reset offset — intentional: offset reset belongs to
// SetTicket, not SetDetail, so that async detail loads do not jump the scroll
// position when the user has already scrolled.
func (m Model) SetDetail(logs []models.LogEntry, plan *models.LogEntry, epoch int) Model {
	m.logs = logs
	m.plan = plan
	m.epoch = epoch
	m = m.buildRendered()
	return m
}

// SetFocus returns a copy with the focused field updated.
func (m Model) SetFocus(focused bool) Model {
	m.focused = focused
	return m
}

// SetSize returns a copy with width and height updated and the rendered cache
// rebuilt.
func (m Model) SetSize(width, height int) Model {
	m.width = width
	m.height = height
	m = m.buildRendered()
	return m
}

// Update handles DetailLoadedMsg (with epoch guard) and j/k scroll keys (only
// when focused). All other messages are ignored. The returned cmd is always nil.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case DetailLoadedMsg:
		if msg.Epoch != m.epoch {
			// stale message — a newer LoadCmd is in flight or has already arrived
			return m, nil
		}
		return m.SetDetail(msg.Logs, msg.Plan, msg.Epoch), nil

	case tea.KeyMsg:
		if !m.focused {
			return m, nil
		}
		innerHeight := m.height - 2
		lines := strings.Split(m.rendered, "\n")
		maxOffset := len(lines) - innerHeight
		if maxOffset < 0 {
			maxOffset = 0
		}
		switch msg.String() {
		case "j", "down":
			m.offset++
			if m.offset > maxOffset {
				m.offset = maxOffset
			}
		case "k", "up":
			m.offset--
			if m.offset < 0 {
				m.offset = 0
			}
		}
	}

	return m, nil
}

// View renders the component. Returns "" if width or height is 0. When no
// ticket is selected, renders a placeholder inside the appropriate panel style.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	innerWidth := m.width - 2
	innerHeight := m.height - 2

	borderStyle := styles.PanelInactive
	if m.focused {
		borderStyle = styles.PanelActive
	}

	if m.ticket == nil {
		return borderStyle.
			Width(innerWidth).
			Height(innerHeight).
			Render("Select a ticket")
	}

	lines := strings.Split(m.rendered, "\n")
	maxOffset := len(lines) - innerHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
	if m.offset < 0 {
		m.offset = 0
	}

	end := m.offset + innerHeight
	if end > len(lines) {
		end = len(lines)
	}
	visible := lines[m.offset:end]
	content := strings.Join(visible, "\n")

	return borderStyle.
		Width(innerWidth).
		Height(innerHeight).
		Render(content)
}

// buildRendered rebuilds the cached glamour-rendered markdown string. It is
// called by SetTicket, SetDetail, and SetSize. View() reads only the cache and
// never triggers a rebuild.
func (m Model) buildRendered() Model {
	if m.width == 0 || m.height == 0 {
		m.rendered = ""
		return m
	}

	innerWidth := m.width - 2

	var planBody string
	if m.plan != nil {
		planBody = m.plan.Body
	} else {
		planBody = "(none)"
	}

	var descBody string
	if m.ticket != nil && m.ticket.Description != "" {
		descBody = m.ticket.Description
	} else {
		descBody = "(none)"
	}

	var titleLine, statusLine string
	if m.ticket != nil {
		titleLine = fmt.Sprintf("# %s", m.ticket.Title)
		statusLine = fmt.Sprintf("**Status:** %s", string(m.ticket.Status))
	}

	var logLines []string
	for _, entry := range m.logs {
		// Skip plan entries — already shown in the Plan section above.
		if entry.Kind == "plan" {
			continue
		}
		logLines = append(logLines, fmt.Sprintf("- %s %s: %s",
			entry.CreatedAt.Format("2006-01-02 15:04"),
			entry.Kind,
			entry.Body,
		))
	}
	logSection := strings.Join(logLines, "\n")
	if logSection == "" {
		logSection = "(none)"
	}

	markdown := fmt.Sprintf("%s\n%s\n\n## Description\n%s\n\n## Plan\n%s\n\n## Log\n%s\n",
		titleLine,
		statusLine,
		descBody,
		planBody,
		logSection,
	)

	if m.renderer == nil || m.rendererWidth != innerWidth {
		r, err := glamour.NewTermRenderer(
			glamour.WithStandardStyle("dark"),
			glamour.WithWordWrap(innerWidth),
		)
		if err != nil {
			m.rendered = markdown
			return m
		}
		m.renderer = r
		m.rendererWidth = innerWidth
	}

	out, err := m.renderer.Render(markdown)
	if err != nil {
		m.rendered = markdown
		return m
	}

	m.rendered = out
	return m
}

// LoadCmd returns a tea.Cmd that asynchronously fetches the log entries and
// plan for the given ticket. The result is tagged with epoch for staleness
// detection in Update.
func LoadCmd(db *sql.DB, ticketID int64, epoch int) tea.Cmd {
	return func() tea.Msg {
		idStr := strconv.FormatInt(ticketID, 10)
		logs, err := log.GetAll(idStr, db)
		if err != nil {
			return DetailLoadedMsg{Epoch: epoch, Err: err}
		}
		plan, err := log.LatestPlan(idStr, db)
		if err != nil {
			return DetailLoadedMsg{Epoch: epoch, Err: err}
		}
		return DetailLoadedMsg{Logs: logs, Plan: plan, Epoch: epoch}
	}
}
