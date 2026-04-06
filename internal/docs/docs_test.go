package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helpers

func makeTempDocsDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := DocsDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("makeTempDocsDir: %v", err)
	}
	return root
}

func writeDoc(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeDoc: %v", err)
	}
	return path
}

// TestNextDocID_Empty verifies that an empty docs dir returns "001".
func TestNextDocID_Empty(t *testing.T) {
	root := makeTempDocsDir(t)
	got, err := NextDocID(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "001" {
		t.Errorf("want %q, got %q", "001", got)
	}
}

// TestNextDocID_Gap verifies that gaps are ignored and max+1 is returned.
func TestNextDocID_Gap(t *testing.T) {
	root := makeTempDocsDir(t)
	dir := DocsDir(root)
	writeDoc(t, dir, "001-foo.md", "")
	writeDoc(t, dir, "003-bar.md", "")
	got, err := NextDocID(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "004" {
		t.Errorf("want %q, got %q", "004", got)
	}
}

// TestNextDocID_NonMatchingIgnored verifies that non-NNN files don't affect the counter.
func TestNextDocID_NonMatchingIgnored(t *testing.T) {
	root := makeTempDocsDir(t)
	dir := DocsDir(root)
	writeDoc(t, dir, "readme.txt", "")
	got, err := NextDocID(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "001" {
		t.Errorf("want %q, got %q", "001", got)
	}
}

// TestParseDocMeta_Valid verifies all fields parse correctly.
func TestParseDocMeta_Valid(t *testing.T) {
	root := makeTempDocsDir(t)
	dir := DocsDir(root)
	content := `# 007 — My Great Analysis

**Type:** analysis
**Date:** 2026-04-05
**By:** architect

---

Body text here.
`
	path := writeDoc(t, dir, "007-my-great-analysis.md", content)

	meta, err := ParseDocMeta(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := map[string][2]string{
		"ID":    {"007", meta.ID},
		"Slug":  {"my-great-analysis", meta.Slug},
		"Title": {"My Great Analysis", meta.Title},
		"Type":  {"analysis", meta.Type},
		"Date":  {"2026-04-05", meta.Date},
		"By":    {"architect", meta.By},
		"Path":  {path, meta.Path},
	}
	for field, pair := range checks {
		if pair[0] != pair[1] {
			t.Errorf("field %s: want %q, got %q", field, pair[0], pair[1])
		}
	}
}

// TestParseDocMeta_Malformed verifies that a malformed first line yields Title "(malformed)" and no error.
func TestParseDocMeta_Malformed(t *testing.T) {
	root := makeTempDocsDir(t)
	dir := DocsDir(root)
	content := "not a valid title line\n\n**Type:** analysis\n"
	path := writeDoc(t, dir, "002-bad.md", content)

	meta, err := ParseDocMeta(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Title != "(malformed)" {
		t.Errorf("want Title %q, got %q", "(malformed)", meta.Title)
	}
}

// TestValidateSlug_Table is a table-driven test for ValidateSlug.
func TestValidateSlug_Table(t *testing.T) {
	cases := []struct {
		slug    string
		wantErr bool
	}{
		{"my-doc", false},
		{"analysis", false},
		{"post-mortem-2026", false},
		{"a", false},
		{"abc123", false},
		{"", true},
		{"My-Doc", true},
		{"has spaces", true},
		{"trailing-", true},
		{"-leading", true},
		{"double--hyphen", true},
		{strings.Repeat("a", 61), true},
		{strings.Repeat("a", 60), false},
	}

	for _, tc := range cases {
		err := ValidateSlug(tc.slug)
		if (err != nil) != tc.wantErr {
			t.Errorf("ValidateSlug(%q): wantErr=%v, got err=%v", tc.slug, tc.wantErr, err)
		}
	}
}

// TestResolveDoc_ByID verifies resolution by numeric ID.
func TestResolveDoc_ByID(t *testing.T) {
	root := makeTempDocsDir(t)
	dir := DocsDir(root)
	writeDoc(t, dir, "001-alpha.md", "")
	writeDoc(t, dir, "002-beta.md", "")

	path, err := ResolveDoc(root, "2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Base(path) != "002-beta.md" {
		t.Errorf("want 002-beta.md, got %s", filepath.Base(path))
	}
}

// TestResolveDoc_BySlug verifies resolution by full slug.
func TestResolveDoc_BySlug(t *testing.T) {
	root := makeTempDocsDir(t)
	dir := DocsDir(root)
	writeDoc(t, dir, "001-alpha.md", "")
	writeDoc(t, dir, "002-beta.md", "")

	path, err := ResolveDoc(root, "alpha")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Base(path) != "001-alpha.md" {
		t.Errorf("want 001-alpha.md, got %s", filepath.Base(path))
	}
}

// TestResolveDoc_Ambiguous verifies an error is returned when multiple files match.
func TestResolveDoc_Ambiguous(t *testing.T) {
	root := makeTempDocsDir(t)
	dir := DocsDir(root)
	writeDoc(t, dir, "001-foo-bar.md", "")
	writeDoc(t, dir, "002-foo-baz.md", "")

	_, err := ResolveDoc(root, "foo")
	if err == nil {
		t.Fatal("expected ambiguous error, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected 'ambiguous' in error, got: %v", err)
	}
}

// TestResolveDoc_NotFound verifies an error is returned when nothing matches.
func TestResolveDoc_NotFound(t *testing.T) {
	root := makeTempDocsDir(t)
	dir := DocsDir(root)
	writeDoc(t, dir, "001-alpha.md", "")

	_, err := ResolveDoc(root, "zzz")
	if err == nil {
		t.Fatal("expected not-found error, got nil")
	}
	if !strings.Contains(err.Error(), "no doc found") {
		t.Errorf("expected 'no doc found' in error, got: %v", err)
	}
}
