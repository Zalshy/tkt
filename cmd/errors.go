package cmd

import "strings"

// msgNoSession is the standard error message for commands that require an active session.
// Use this constant in every session.ErrNoSession branch — do not write bespoke messages.
const msgNoSession = "no active session. Run: tkt session --role <architect|implementer>"

// msgExpiredSession is the standard error message when the current session row exists
// but has expired. Tell users how to recover and retry the command they attempted.
const msgExpiredSession = "session has expired after inactivity. Run: tkt session --role <architect|implementer>, then retry this command"

func withManualHint(msg string) string {
	if strings.Contains(msg, "See: tkt man") || strings.Contains(msg, "Run: tkt man") {
		return msg
	}

	lower := strings.ToLower(msg)
	hint := ""
	switch {
	case strings.Contains(lower, "unknown command"):
		hint = "Run: tkt man or tkt man minimal"
	case strings.Contains(lower, "invalid --status") || strings.Contains(lower, "invalid --to") || strings.Contains(lower, "status") && strings.Contains(lower, "must be one of"):
		hint = "See: tkt man state-machine"
	case strings.Contains(lower, "plan required") || strings.Contains(lower, "no editor found") || strings.Contains(lower, "plan") && strings.Contains(lower, "$editor"):
		hint = "See: tkt man plan"
	case strings.Contains(lower, "requires role") || strings.Contains(lower, "requires a different session") || strings.Contains(lower, "transition"):
		hint = "See: tkt man advance or tkt man state-machine"
	case strings.Contains(lower, "stats:") && (strings.Contains(lower, "invalid") || strings.Contains(lower, "since") || strings.Contains(lower, "until")):
		hint = "See: tkt man stats"
	}
	if hint == "" {
		return msg
	}
	return msg + "\n" + hint
}
