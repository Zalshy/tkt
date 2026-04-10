package kanban

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/tui/styles"
)

// Column is a single status lane in the Kanban board.
type Column struct {
	status       models.Status
	label        string
	tickets      []models.Ticket
	cursor       int
	scrollOffset int
	width        int
	height       int
	focused      bool
}

func newColumn(status models.Status, label string, width, height int) Column {
	return Column{status: status, label: label, width: width, height: height}
}

// SetFocus returns a copy of the column with focus updated.
func (c Column) SetFocus(v bool) Column { c.focused = v; return c }

// SetSize returns a copy of the column with size updated.
func (c Column) SetSize(w, h int) Column { c.width = w; c.height = h; return c }

// SetTickets replaces the ticket slice and resets cursor and scroll.
func (c Column) SetTickets(t []models.Ticket) Column {
	c.tickets = t
	c.cursor = 0
	c.scrollOffset = 0
	return c
}

// SelectedTicket returns a pointer to the ticket under the cursor, or nil.
func (c Column) SelectedTicket() *models.Ticket {
	if len(c.tickets) == 0 || c.cursor < 0 || c.cursor >= len(c.tickets) {
		return nil
	}
	t := c.tickets[c.cursor]
	return &t
}

// CursorUp moves the cursor up by one, clamped to 0.
func (c Column) CursorUp() Column {
	if c.cursor > 0 {
		c.cursor--
	}
	return c.adjustScroll()
}

// CursorDown moves the cursor down by one, clamped to last index.
func (c Column) CursorDown() Column {
	if c.cursor < len(c.tickets)-1 {
		c.cursor++
	}
	return c.adjustScroll()
}

func (c Column) adjustScroll() Column {
	visibleRows := c.visibleCards()
	if visibleRows < 1 {
		visibleRows = 1
	}
	if c.cursor < c.scrollOffset {
		c.scrollOffset = c.cursor
	}
	if c.cursor >= c.scrollOffset+visibleRows {
		c.scrollOffset = c.cursor - visibleRows + 1
	}
	return c
}

func (c Column) visibleCards() int {
	// column header (2 lines: label + divider) + border (2 lines: top + bottom)
	innerH := c.height - 4
	if innerH < 0 {
		innerH = 0
	}
	if CardHeight == 0 {
		return 0
	}
	// Each card takes CardHeight lines + 1 spacer line between cards.
	// For n cards: n*CardHeight + (n-1) spacers = n*(CardHeight+1) - 1
	// So: n = (innerH + 1) / (CardHeight + 1)
	return (innerH + 1) / (CardHeight + 1)
}

// Update handles key messages for cursor movement.
func (c Column) Update(msg tea.Msg) (Column, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			return c.CursorDown(), nil
		case "k", "up":
			return c.CursorUp(), nil
		}
	}
	return c, nil
}

// statusColor returns the color associated with a status lane for column headers.
func statusColor(s models.Status) lipgloss.Color {
	switch s {
	case models.StatusTodo:
		return styles.StatusTodo
	case models.StatusPlanning:
		return styles.StatusPlanning
	case models.StatusInProgress:
		return styles.StatusInProg
	case models.StatusDone:
		return styles.StatusDone
	case models.StatusVerified:
		return styles.StatusVerified
	default:
		return styles.Muted
	}
}

// View renders the column as a bordered panel.
func (c Column) View() string {
	if c.width == 0 || c.height == 0 {
		return ""
	}

	// innerWidth is the text content area; content lines are built as
	// " " + text(innerWidth) + " ", so total = innerWidth+2.
	// The border's inner area is c.width-2, so we need innerWidth+2 = c.width-2
	// → innerWidth = c.width-4.
	innerWidth := c.width - 4
	if innerWidth < 2 {
		innerWidth = 2
	}

	// Column header: label + count
	count := fmt.Sprintf("(%d)", len(c.tickets))
	headerText := c.label + " " + count
	headerText = truncate(headerText, innerWidth)
	headerText = padRight(headerText, innerWidth)

	headerStyle := lipgloss.NewStyle().
		Foreground(statusColor(c.status)).
		Background(styles.BgDeep).
		Bold(c.focused)

	header := " " + headerStyle.Render(headerText) + " "
	divider := " " + strings.Repeat("─", innerWidth) + " "

	// Cards
	visibleN := c.visibleCards()
	end := c.scrollOffset + visibleN
	if end > len(c.tickets) {
		end = len(c.tickets)
	}

	var lines []string
	lines = append(lines, header)
	lines = append(lines, divider)

	if len(c.tickets) == 0 {
		emptyMsg := truncate("— empty —", innerWidth)
		emptyMsg = padRight(emptyMsg, innerWidth)
		emptyStyle := lipgloss.NewStyle().Foreground(styles.Faint)
		lines = append(lines, " "+emptyStyle.Render(emptyMsg)+" ")
	} else {
		for i := c.scrollOffset; i < end; i++ {
			selected := (i == c.cursor) && c.focused
			card := renderCard(c.tickets[i], innerWidth, selected, c.focused)
			// Each card is CardHeight lines; pad with a blank spacer line between cards
			for _, l := range strings.Split(card, "\n") {
				lines = append(lines, " "+l+" ")
			}
			if i < end-1 {
				lines = append(lines, "") // spacer between cards (not after last)
			}
		}
	}

	// Pad remaining height with blank lines
	for len(lines) < c.height {
		lines = append(lines, "")
	}
	// Trim to height
	if len(lines) > c.height {
		lines = lines[:c.height]
	}

	// Wrap in a border
	borderStyle := styles.PanelInactive
	if c.focused {
		borderStyle = styles.PanelActive
	}
	content := strings.Join(lines, "\n")
	return borderStyle.Width(c.width - 2).Height(c.height - 2).Render(content)
}
