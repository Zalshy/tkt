package kanban

import (
	"database/sql"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/ticket"
)

// TickMsg is sent by TickCmd on each animation frame for the comet bar.
type TickMsg struct{}

// TickCmd returns a tea.Cmd that fires after 32ms sending a TickMsg.
// The self-scheduling loop (each TickMsg handler calls TickCmd again) has no
// explicit off-switch. This is the standard BubbleTea tick pattern: the loop
// terminates when tea.Quit is processed and BubbleTea stops dispatching messages.
func TickCmd() tea.Cmd {
	return tea.Tick(32*time.Millisecond, func(time.Time) tea.Msg {
		return TickMsg{}
	})
}

// BoardLoadedMsg is the async result of LoadCmd.
type BoardLoadedMsg struct {
	Tickets []models.Ticket
	Epoch   int
	Err     error
}

// numCols is the number of visual columns in the board.
const numCols = 3

// boardLoadLimit is the maximum number of tickets fetched per board load.
// The board renders at most a few dozen visible tickets; this cap prevents
// runaway queries while staying effectively unbounded for any real project.
const boardLoadLimit = 10000

// Board is the top-level Kanban component: 3 status columns side by side.
// IN_PROGRESS tickets appear visually inside the PLANNING column.
// VERIFIED tickets appear visually inside the DONE column.
type Board struct {
	columns   [numCols]Column
	activeCol int
	width     int
	height    int
	tickN     int
}

// SetTickN returns a copy of the board with tickN set. Pure — safe to call in View().
func (b Board) SetTickN(n int) Board { b.tickN = n; return b }

var columnDefs = [numCols]struct {
	status models.Status
	label  string
}{
	{models.StatusTodo, "TODO"},
	{models.StatusPlanning, "PLANNING"},
	{models.StatusDone, "DONE"},
}

// New constructs a Board. Call SetSize after a WindowSizeMsg to set real dimensions.
func New(width, height int) Board {
	b := Board{width: width, height: height}
	colW := 0
	if width > 0 {
		colW = width / numCols
	}
	for i, def := range columnDefs {
		w := colW
		if i == numCols-1 {
			w = width - (numCols-1)*colW // absorb remainder in last column
		}
		b.columns[i] = newColumn(def.status, def.label, w, height)
	}
	b.columns[0] = b.columns[0].SetFocus(true)
	return b
}

// SetSize recomputes column widths.
func (b Board) SetSize(width, height int) Board {
	b.width = width
	b.height = height
	colW := width / numCols
	for i := range b.columns {
		w := colW
		if i == numCols-1 {
			w = width - (numCols-1)*colW
		}
		b.columns[i] = b.columns[i].SetSize(w, height)
	}
	return b
}

// SetTickets distributes the flat ticket slice into visual columns:
//   - StatusTodo       → TODO column
//   - StatusPlanning   → PLANNING column  (rendered with plain style)
//   - StatusInProgress → PLANNING column  (rendered with vivid IN PROGRESS tag)
//   - StatusDone       → DONE column      (rendered with plain style)
//   - StatusVerified   → DONE column      (rendered with VERIFIED tag)
func (b Board) SetTickets(tickets []models.Ticket) Board {
	buckets := make(map[models.Status][]models.Ticket)
	for _, t := range tickets {
		switch t.Status {
		case models.StatusInProgress:
			buckets[models.StatusPlanning] = append(buckets[models.StatusPlanning], t)
		case models.StatusVerified:
			buckets[models.StatusDone] = append(buckets[models.StatusDone], t)
		default:
			buckets[t.Status] = append(buckets[t.Status], t)
		}
	}
	for i, def := range columnDefs {
		b.columns[i] = b.columns[i].SetTickets(buckets[def.status])
	}
	return b
}

// SelectedTicket returns the ticket under the cursor in the active column, or nil.
func (b Board) SelectedTicket() *models.Ticket {
	return b.columns[b.activeCol].SelectedTicket()
}

// ActiveStatus returns the status of the active column.
func (b Board) ActiveStatus() models.Status {
	return columnDefs[b.activeCol].status
}

// ActiveCol returns the index of the active column (0–numCols-1).
func (b Board) ActiveCol() int {
	return b.activeCol
}

// Update routes key messages to column navigation or the active column.
func (b Board) Update(msg tea.Msg) (Board, tea.Cmd) {
	switch msg := msg.(type) {
	case TickMsg:
		b.tickN++
		return b, TickCmd()
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			if b.activeCol > 0 {
				b.columns[b.activeCol] = b.columns[b.activeCol].SetFocus(false)
				b.activeCol--
				b.columns[b.activeCol] = b.columns[b.activeCol].SetFocus(true)
			}
			return b, nil
		case "right", "l":
			if b.activeCol < numCols-1 {
				b.columns[b.activeCol] = b.columns[b.activeCol].SetFocus(false)
				b.activeCol++
				b.columns[b.activeCol] = b.columns[b.activeCol].SetFocus(true)
			}
			return b, nil
		case "j", "k", "down", "up":
			var cmd tea.Cmd
			b.columns[b.activeCol], cmd = b.columns[b.activeCol].Update(msg)
			return b, cmd
		}
	}
	return b, nil
}

// View renders all columns side by side.
func (b Board) View() string {
	views := make([]string, numCols)
	for i := range b.columns {
		views[i] = b.columns[i].View(b.tickN)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, views...)
}

// LoadCmd fetches all non-cancelled tickets for the board.
func LoadCmd(db *sql.DB, epoch int) tea.Cmd {
	return func() tea.Msg {
		opts := ticket.ListOptions{
			IncludeVerified: true,
			ExcludeCanceled: true,
			All:             false,
			Sort:            "id",
			Limit:           boardLoadLimit,
		}
		result, err := ticket.List(opts, db)
		if err != nil {
			return BoardLoadedMsg{Epoch: epoch, Err: err}
		}
		return BoardLoadedMsg{Tickets: result.Tickets, Epoch: epoch}
	}
}
