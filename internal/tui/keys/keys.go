package keys

// Binding is a single key hint: the key to press and a short description.
type Binding struct {
	Key  string
	Desc string
}

// Scope identifies which UI context a set of bindings belongs to.
type Scope int

const (
	ScopeGlobal Scope = iota
	ScopeList
	ScopeDetail
	ScopeSearch
)

// Section groups bindings under a named context.
type Section struct {
	Title    string
	Scope    Scope
	Bindings []Binding
}

var sections = []Section{
	{
		Title: "Global",
		Scope: ScopeGlobal,
		Bindings: []Binding{
			{"q", "quit"},
			{"?", "help"},
		},
	},
	{
		Title: "Board",
		Scope: ScopeList,
		Bindings: []Binding{
			{"←/→ h/l", "switch column"},
			{"↑/↓ j/k", "navigate"},
			{"enter", "open"},
			{"/", "search"},
			{"q", "quit"},
		},
	},
	{
		Title: "Detail",
		Scope: ScopeDetail,
		Bindings: []Binding{
			{"↑/↓ j/k", "scroll"},
			{"esc", "close"},
		},
	},
	{
		Title: "Search",
		Scope: ScopeSearch,
		Bindings: []Binding{
			{"esc", "cancel"},
			{"enter", "select"},
		},
	},
}

// All returns all registered sections in display order.
func All() []Section {
	return sections
}

// For returns the bindings for the given scope. Returns nil if the scope has no entry.
func For(s Scope) []Binding {
	for _, sec := range sections {
		if sec.Scope == s {
			return sec.Bindings
		}
	}
	return nil
}
