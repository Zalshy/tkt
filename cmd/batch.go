package cmd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zalshy/tkt/internal/db"
	"github.com/zalshy/tkt/internal/models"
	"github.com/zalshy/tkt/internal/output"
	"github.com/zalshy/tkt/internal/ticket"
)

var batchN int

var batchCmd = &cobra.Command{
	Use:   "batch",
	Short: "Display the next N executable phases",
	Args:  cobra.NoArgs,
	RunE:  runBatch,
}

func init() {
	batchCmd.Flags().IntVar(&batchN, "n", 6, "number of phases to display")
	rootCmd.AddCommand(batchCmd)
}

func runBatch(cmd *cobra.Command, args []string) error {
	if batchN < 1 {
		return fmt.Errorf("--n must be >= 1")
	}

	root, err := requireRoot()
	if err != nil {
		return err
	}

	database, err := db.Open(root)
	if err != nil {
		return fmt.Errorf("batch: open db: %w", err)
	}
	defer database.Close()

	out := cmd.OutOrStdout()

	// Load active tickets (not VERIFIED, not CANCELED, not soft-deleted).
	activeTickets, err := ticket.ListActive(database)
	if err != nil {
		return fmt.Errorf("batch: query tickets: %w", err)
	}

	if len(activeTickets) == 0 {
		fmt.Fprint(out, "No active tickets.\n")
		return nil
	}

	// Load edges (Query 4 from spec).
	edges, err := ticket.ListDependencyEdges(database)
	if err != nil {
		return fmt.Errorf("batch: query dependencies: %w", err)
	}

	// Build BFS structures.
	depCount := make(map[int64]int)
	unlockedBy := make(map[int64][]int64)

	// Initialize depCount for every active ticket.
	for id := range activeTickets {
		depCount[id] = 0
	}

	for _, e := range edges {
		// Skip if dep is already resolved (VERIFIED or CANCELED).
		if e.DepStat == models.StatusVerified || e.DepStat == models.StatusCanceled {
			continue
		}
		// Skip if the ticket itself is not in active set.
		if _, ok := activeTickets[e.TicketID]; !ok {
			continue
		}
		depCount[e.TicketID]++
		unlockedBy[e.DependsOn] = append(unlockedBy[e.DependsOn], e.TicketID)
	}

	// Seed phase 0: all tickets with depCount == 0, sorted by ID.
	var seed []int64
	for id, cnt := range depCount {
		if cnt == 0 {
			seed = append(seed, id)
		}
	}
	sort.Slice(seed, func(i, j int) bool { return seed[i] < seed[j] })

	if len(seed) == 0 {
		fmt.Fprint(out, "No unblocked tickets found.\n")
		return nil
	}

	assigned := make(map[int64]bool)
	var phases [][]int64

	current := seed
	for len(phases) < batchN {
		if len(current) == 0 {
			break
		}
		phases = append(phases, current)
		for _, id := range current {
			assigned[id] = true
		}

		// Build next phase.
		var next []int64
		for _, id := range current {
			for _, downstream := range unlockedBy[id] {
				depCount[downstream]--
				if depCount[downstream] == 0 && !assigned[downstream] {
					next = append(next, downstream)
				}
			}
		}
		sort.Slice(next, func(i, j int) bool { return next[i] < next[j] })
		current = next
	}

	// Render phases.
	for phaseIdx, phase := range phases {
		label := fmt.Sprintf("Phase %d", phaseIdx+1)
		// Left-pad label to 10 chars.
		padded := fmt.Sprintf("%-10s", label)

		var ticketParts []string
		for _, id := range phase {
			status := activeTickets[id]
			var color, glyph string
			switch {
			case status == models.StatusInProgress:
				color = output.Cyan
				glyph = "⟳"
			case status == models.StatusPlanning || status == models.StatusDone:
				color = output.Reset
				glyph = "●"
			case status == models.StatusTodo && phaseIdx == 0:
				color = output.Reset
				glyph = "◎"
			default:
				// TODO in phase > 0
				color = output.Dim
				glyph = "○"
			}
			ticketParts = append(ticketParts, color+glyph+" #"+strconv.FormatInt(id, 10)+output.Reset)
		}

		fmt.Fprintf(out, "%s%s\n", padded, strings.Join(ticketParts, "  "))

		// Blank line between phases (not after last).
		if phaseIdx < len(phases)-1 {
			fmt.Fprintln(out)
		}
	}

	// Summary line.
	totalTickets := 0
	inProgressCount := 0
	for _, phase := range phases {
		totalTickets += len(phase)
		for _, id := range phase {
			if activeTickets[id] == models.StatusInProgress {
				inProgressCount++
			}
		}
	}

	separator := strings.Repeat("─", 45)
	fmt.Fprintf(out, "\n%s\n", separator)
	fmt.Fprintf(out, "%d phases remaining · %d tickets · %d in progress\n",
		len(phases), totalTickets, inProgressCount)

	return nil
}
