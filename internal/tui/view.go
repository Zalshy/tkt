package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Package-level style variables. These are mutable so a theme system can
// rewrite them in-place without changing call sites.
var (
	styleHeaderTodo       = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Bold(true)
	styleHeaderPlanning   = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Bold(true)
	styleHeaderInProgress = lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6")).Bold(true)
	styleHeaderDone       = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Bold(true)
	styleHeaderVerified   = lipgloss.NewStyle().Foreground(lipgloss.Color("#7C3AED")).Bold(true)
	styleHeaderCanceled   = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Bold(true)
	styleSelected         = lipgloss.NewStyle().Background(lipgloss.Color("#1F2937")).Bold(true)
	styleBorderActive     = lipgloss.Color("#7C3AED")
	styleAppHeader        = lipgloss.NewStyle().Background(lipgloss.Color("#111827")).Padding(0, 1)
	styleKeyHint          = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Background(lipgloss.Color("#374151")).Padding(0, 1)
	styleError            = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Bold(true)
	styleStatusBar        = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	styleMuted            = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
)

// colNames holds the display name for each column index.
var colNames = [6]string{
	"TODO", "PLANNING", "IN_PROGRESS", "DONE", "VERIFIED", "CANCELED",
}

// colHeaderStyles maps column index to its header style.
var colHeaderStyles = [6]lipgloss.Style{
	styleHeaderTodo,
	styleHeaderPlanning,
	styleHeaderInProgress,
	styleHeaderDone,
	styleHeaderVerified,
	styleHeaderCanceled,
}

// View renders the complete TUI. It satisfies tea.Model.
func (m Model) View() string {
	// Minimum size guard.
	if m.width < 80 || m.height < 24 {
		msg := fmt.Sprintf("Terminal too small (%dx%d)\nMinimum: 80x24", m.width, m.height)
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			styleError.Render(msg))
	}

	header := m.renderHeader()
	footer := m.renderFooter()

	contentHeight := m.height - 2 // 1 for header + 1 for footer

	cols := m.visibleColumns()
	panelWidth := 0
	if m.showPanel {
		panelWidth = min(50, m.width/3)
		if panelWidth < 20 {
			panelWidth = 20
		}
	}

	kanbanWidth := m.width
	if m.showPanel {
		kanbanWidth = m.width - panelWidth - 1 // 1 for divider char
	}

	kanban := m.renderKanban(kanbanWidth, contentHeight, cols)

	var content string
	if m.showPanel {
		panel := m.renderPanel(panelWidth, contentHeight)
		content = lipgloss.JoinHorizontal(lipgloss.Top, kanban, "│", panel)
	} else {
		content = kanban
	}

	return header + "\n" + content + "\n" + footer
}

// renderHeader returns the 1-line header bar.
func (m Model) renderHeader() string {
	var parts []string
	parts = append(parts, "tkt monitor")
	if m.projectPath != "" {
		parts = append(parts, m.projectPath)
	}
	if !m.lastRefresh.IsZero() {
		parts = append(parts, "last refresh "+m.lastRefresh.Format("15:04:05"))
	}
	if m.lastErr != nil {
		parts = append(parts, styleError.Render("ERR: "+m.lastErr.Error()))
	}

	text := strings.Join(parts, "  ·  ")
	return styleAppHeader.Width(m.width).Render(text)
}

// renderFooter returns the 1-line footer with key hints.
func (m Model) renderFooter() string {
	type hint struct {
		key   string
		label string
	}
	hints := []hint{
		{"r", "refresh"},
		{"q", "quit"},
		{"↑↓", "navigate"},
		{"enter", "detail"},
		{"esc", "close"},
		{"c", "canceled"},
	}

	var result string
	sep := "  "
	for i, h := range hints {
		part := styleKeyHint.Render(h.key) + " " + styleMuted.Render(h.label)
		candidate := result
		if i > 0 {
			candidate += sep
		}
		candidate += part
		if lipgloss.Width(candidate) > m.width {
			break
		}
		result = candidate
	}

	return styleStatusBar.Width(m.width).Render(result)
}

// renderKanban renders all visible columns side by side.
func (m Model) renderKanban(width, height int, cols []int) string {
	numCols := len(cols)
	if numCols == 0 {
		return lipgloss.NewStyle().Width(width).Height(height).Render("")
	}

	baseWidth := width / numCols
	remainder := width - baseWidth*numCols

	var rendered []string
	for i, colIdx := range cols {
		w := baseWidth
		if i == numCols-1 {
			w = baseWidth + remainder
		}
		isActive := i == m.colIdx
		col := m.renderColumn(colIdx, w, height, isActive)
		rendered = append(rendered, col)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

// renderColumn renders a single kanban column.
func (m Model) renderColumn(colIdx, width, height int, isActive bool) string {
	// Build column content: header + rule + cards.
	tickets := m.columns[colIdx]
	name := colNames[colIdx]
	headerStyle := colHeaderStyles[colIdx]

	header := headerStyle.Render(fmt.Sprintf("%s (%d)", name, len(tickets)))
	rule := strings.Repeat("─", width)

	var lines []string
	lines = append(lines, header)
	lines = append(lines, rule)

	for rowIdx, t := range tickets {
		isCursor := isActive && rowIdx == m.rowIdx

		idLine := fmt.Sprintf("#%d", t.ID)
		titleLine := truncateTitle(t.Title, width-2)

		card := idLine + "\n" + titleLine

		if isCursor {
			card = styleSelected.Width(width).Render(card)
		} else {
			card = lipgloss.NewStyle().Width(width).Render(card)
		}

		lines = append(lines, card)
		lines = append(lines, "") // blank separator between cards
	}

	content := strings.Join(lines, "\n")

	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		MaxHeight(height).
		Render(content)
}

// renderPanel renders the side panel for the selected ticket.
func (m Model) renderPanel(width, height int) string {
	innerWidth := width - 4 // account for border + padding
	if innerWidth < 1 {
		innerWidth = 1
	}

	var lines []string

	// Ticket header.
	t := m.panelTicket
	lines = append(lines, fmt.Sprintf("#%d  %s", t.ID, string(t.Status)))
	lines = append(lines, truncateTitle(t.Title, innerWidth))
	lines = append(lines, "")

	// Latest plan.
	lines = append(lines, styleMuted.Render("Plan:"))
	if m.panelPlan != nil {
		planBody := wrapText(m.panelPlan.Body, innerWidth)
		lines = append(lines, planBody)
	} else {
		lines = append(lines, styleMuted.Render("(none)"))
	}
	lines = append(lines, "")

	// Last 5 log entries.
	lines = append(lines, styleMuted.Render("Recent:"))
	logs := m.panelLogs
	start := 0
	if len(logs) > 5 {
		start = len(logs) - 5
	}
	for _, entry := range logs[start:] {
		ts := entry.CreatedAt.Format("01-02 15:04")
		line := fmt.Sprintf("[%s] %s: %s", ts, entry.Kind, entry.Body)
		line = truncateTitle(line, innerWidth)
		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")

	panelStyle := lipgloss.NewStyle().
		Width(innerWidth).
		Height(height - 2). // border takes 2 rows
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styleBorderActive).
		Padding(0, 1)

	return panelStyle.Render(content)
}

// wrapText wraps text to the given width by inserting newlines.
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	var lines []string
	currentLine := ""
	for _, word := range words {
		if currentLine == "" {
			currentLine = word
		} else if len(currentLine)+1+len(word) <= width {
			currentLine += " " + word
		} else {
			lines = append(lines, currentLine)
			currentLine = word
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}
	return strings.Join(lines, "\n")
}

// min returns the smaller of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
