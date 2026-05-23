package man

import (
	"strings"
	"testing"
)

func TestNormalizeName(t *testing.T) {
	tests := map[string]string{
		" session ":     "session",
		"TKT session":   "session",
		"ticket_log":    "ticket-log",
		"llm":           "minimal",
		"TKT ticket_log": "ticket-log",
	}

	for input, want := range tests {
		if got := NormalizeName(input); got != want {
			t.Fatalf("NormalizeName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestReadPage(t *testing.T) {
	page, err := ReadPage(" tkt session ")
	if err != nil {
		t.Fatalf("ReadPage returned error: %v", err)
	}
	if page.Name != "session" {
		t.Fatalf("Name = %q, want session", page.Name)
	}
	if page.Title != "tkt session" {
		t.Fatalf("Title = %q, want tkt session", page.Title)
	}
	if !strings.HasPrefix(page.Body, "# tkt session") {
		t.Fatalf("Body missing session heading: %q", page.Body[:min(40, len(page.Body))])
	}
	if !strings.HasSuffix(page.Body, "\n") {
		t.Fatalf("Body should end with newline")
	}
}

func TestReadPageErrors(t *testing.T) {
	if _, err := ReadPage("   "); err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("ReadPage blank error = %v, want required", err)
	}
	if _, err := ReadPage("missing-page"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("ReadPage missing error = %v, want not found", err)
	}
}

func TestListPagesSorted(t *testing.T) {
	pages, err := ListPages()
	if err != nil {
		t.Fatalf("ListPages returned error: %v", err)
	}
	if len(pages) == 0 {
		t.Fatal("ListPages returned no pages")
	}
	for i := 1; i < len(pages); i++ {
		if pages[i-1].Name > pages[i].Name {
			t.Fatalf("pages not sorted at %d: %q > %q", i, pages[i-1].Name, pages[i].Name)
		}
	}
}

func TestTitleFromBodyFallsBackToName(t *testing.T) {
	if got := titleFromBody("fallback", "not a heading\n## smaller"); got != "fallback" {
		t.Fatalf("titleFromBody fallback = %q, want fallback", got)
	}
}
