package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/zalshy/tkt/internal/state"
)

// WriteAdvanceCheck writes a formatted advance check result to out.
func WriteAdvanceCheck(out io.Writer, check state.CheckResult, explain bool) {
	status := "would advance"
	if !check.Allowed {
		status = "blocked"
	}
	fmt.Fprintf(out, "#%s  %s → %s  %s\n", check.TicketID, check.From, check.To, status)
	if !explain {
		if !check.Allowed {
			fmt.Fprintf(out, "Reason: %s\n", check.Reason)
		}
		return
	}
	fmt.Fprintf(out, "Allowed: %t\n", check.Allowed)
	fmt.Fprintf(out, "Forced: %t\n", check.Forced)
	fmt.Fprintf(out, "Reason: %s\n", check.Reason)
	if check.PlanRequired {
		fmt.Fprintf(out, "Plan required: true\n")
		fmt.Fprintf(out, "Plan present: %t\n", check.PlanPresent)
	}
	if len(check.Hints) > 0 {
		fmt.Fprintf(out, "See: %s\n", strings.Join(check.Hints, ", "))
	}
}

// Plural returns singular if n == 1, otherwise pluralForm.
func Plural(n int, singular, pluralForm string) string {
	if n == 1 {
		return singular
	}
	return pluralForm
}
