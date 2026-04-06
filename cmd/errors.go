package cmd

// msgNoSession is the standard error message for commands that require an active session.
// Use this constant in every session.ErrNoSession branch — do not write bespoke messages.
const msgNoSession = "no active session. Run: tkt session --role <architect|implementer>"
