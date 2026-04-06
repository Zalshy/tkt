package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Update handles all incoming messages and returns the updated model plus any
// follow-up commands. This is the Elm Architecture update function.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		return m, tea.Batch(m.loadData(), m.scheduleTick())

	case dataMsg:
		if msg.err != nil {
			m.lastErr = msg.err
			return m, nil
		}
		m.columns = msg.columns
		m.lastRefresh = time.Now()
		m.colIdx, m.rowIdx = clampCursor(m.colIdx, m.rowIdx, m.columns, m.showCanceled)
		m.lastErr = nil

	case panelDataMsg:
		if msg.err != nil {
			m.lastErr = msg.err
			return m, nil
		}
		m.panelLogs = msg.logs
		m.panelPlan = msg.plan

	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyCtrlC || (msg.Type == tea.KeyRunes && string(msg.Runes) == "q"):
			return m, tea.Quit

		case msg.Type == tea.KeyRunes && string(msg.Runes) == "r":
			return m, tea.Batch(m.loadData(), m.scheduleTick())

		case msg.Type == tea.KeyUp || (msg.Type == tea.KeyRunes && string(msg.Runes) == "k"):
			m.rowIdx--
			m.colIdx, m.rowIdx = clampCursor(m.colIdx, m.rowIdx, m.columns, m.showCanceled)

		case msg.Type == tea.KeyDown || (msg.Type == tea.KeyRunes && string(msg.Runes) == "j"):
			m.rowIdx++
			m.colIdx, m.rowIdx = clampCursor(m.colIdx, m.rowIdx, m.columns, m.showCanceled)

		case msg.Type == tea.KeyLeft || (msg.Type == tea.KeyRunes && string(msg.Runes) == "h"):
			m.colIdx--
			m.colIdx, m.rowIdx = clampCursor(m.colIdx, m.rowIdx, m.columns, m.showCanceled)

		case msg.Type == tea.KeyRight || (msg.Type == tea.KeyRunes && string(msg.Runes) == "l"):
			m.colIdx++
			m.colIdx, m.rowIdx = clampCursor(m.colIdx, m.rowIdx, m.columns, m.showCanceled)

		case msg.Type == tea.KeyEnter:
			cols := m.visibleColumns()
			if m.colIdx < len(cols) {
				col := cols[m.colIdx]
				if m.rowIdx < len(m.columns[col]) {
					m.showPanel = true
					m.panelTicket = m.columns[col][m.rowIdx]
					return m, panelLoad(m.db, m.panelTicket.ID)
				}
			}

		case msg.Type == tea.KeyEsc:
			m.showPanel = false

		case msg.Type == tea.KeyRunes && string(msg.Runes) == "c":
			m.showCanceled = !m.showCanceled
			m.colIdx, m.rowIdx = clampCursor(m.colIdx, m.rowIdx, m.columns, m.showCanceled)
		}
	}

	return m, nil
}

// visibleColumns returns the column indices that are currently displayed.
func (m Model) visibleColumns() []int {
	return visibleCols(m.showCanceled)
}
