package docs

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// DocsDir returns the path to the active docs directory.
func DocsDir(root string) string { return filepath.Join(root, ".tkt", "docs") }

// DocsArchivedDir returns the path to the archived docs directory.
func DocsArchivedDir(root string) string { return filepath.Join(root, ".tkt", "docs", "archived") }

var numPrefixRe = regexp.MustCompile(`^(\d+)-`)
var metaLineRe = regexp.MustCompile(`^\*\*(\w[\w ]+):\*\*\s*(.+)$`)
var titleLineRe = regexp.MustCompile(`^#\s+\d+\s+—\s+(.+)$`)
var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// NextDocID scans DocsDir(root) for *.md files (non-recursive) and returns the
// next available NNN string, e.g. "001", "004". Empty or no matching files
// returns "001".
func NextDocID(root string) (string, error) {
	entries, err := os.ReadDir(DocsDir(root))
	if err != nil {
		if os.IsNotExist(err) {
			return "001", nil
		}
		return "", fmt.Errorf("NextDocID: read dir: %w", err)
	}

	max := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		m := numPrefixRe.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return fmt.Sprintf("%03d", max+1), nil
}

// DocMeta holds the parsed metadata of a doc file.
type DocMeta struct {
	ID    string
	Slug  string
	Title string
	Type  string
	Date  string
	By    string
	Path  string
}

// ParseDocMeta reads the first 20 lines of the file at path and returns a
// DocMeta. Malformed files return a partial DocMeta (empty fields) with no
// error; Title is set to "(malformed)" if line 1 doesn't match the expected
// format.
func ParseDocMeta(path string) (DocMeta, error) {
	base := filepath.Base(path)
	m := numPrefixRe.FindStringSubmatch(base)

	meta := DocMeta{Path: path}

	if m != nil {
		meta.ID = strings.TrimLeft(m[1], "0")
		if meta.ID == "" {
			meta.ID = "0"
		}
		// Zero-pad to at least 3 digits for consistent display
		n, _ := strconv.Atoi(m[1])
		meta.ID = fmt.Sprintf("%03d", n)

		// Slug: part after NNN- prefix, before .md
		after := strings.TrimPrefix(base, m[0])
		meta.Slug = strings.TrimSuffix(after, ".md")
	}

	f, err := os.Open(path)
	if err != nil {
		return meta, fmt.Errorf("ParseDocMeta: open: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() && lineNum < 20 {
		line := scanner.Text()
		lineNum++

		if lineNum == 1 {
			tm := titleLineRe.FindStringSubmatch(line)
			if tm != nil {
				meta.Title = strings.TrimSpace(tm[1])
			} else {
				meta.Title = "(malformed)"
			}
			continue
		}

		mm := metaLineRe.FindStringSubmatch(line)
		if mm == nil {
			continue
		}
		key := strings.TrimSpace(mm[1])
		val := strings.TrimSpace(mm[2])
		switch key {
		case "Type":
			meta.Type = val
		case "Date":
			meta.Date = val
		case "By":
			meta.By = val
		}
	}

	return meta, nil
}

// ValidateSlug returns an error if slug is not a valid doc slug.
// Valid: lowercase alphanumeric and hyphens, ^[a-z0-9]+(-[a-z0-9]+)*$, max 60 chars.
func ValidateSlug(slug string) error {
	if slug == "" {
		return fmt.Errorf("slug must not be empty")
	}
	if len(slug) > 60 {
		return fmt.Errorf("slug must be 60 characters or fewer (got %d)", len(slug))
	}
	if !slugRe.MatchString(slug) {
		return fmt.Errorf("slug %q is invalid: use lowercase alphanumeric and hyphens only (e.g. my-doc)", slug)
	}
	return nil
}

// ResolveDoc finds a doc file in DocsDir(root) matching query. If query is all
// digits, match by numeric prefix. Otherwise match by substring of the slug
// portion. Returns the full file path. Errors on zero or multiple matches.
func ResolveDoc(root, query string) (string, error) {
	entries, err := os.ReadDir(DocsDir(root))
	if err != nil {
		return "", fmt.Errorf("ResolveDoc: read dir: %w", err)
	}

	isID := regexp.MustCompile(`^\d+$`).MatchString(query)
	queryN := 0
	if isID {
		queryN, _ = strconv.Atoi(query)
	}

	var matches []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		m := numPrefixRe.FindStringSubmatch(name)
		if m == nil {
			continue
		}

		if isID {
			n, err := strconv.Atoi(m[1])
			if err != nil {
				continue
			}
			if n == queryN {
				matches = append(matches, filepath.Join(DocsDir(root), name))
			}
		} else {
			// Match against slug portion only (after NNN- prefix, before .md)
			after := strings.TrimPrefix(name, m[0])
			slug := strings.TrimSuffix(after, ".md")
			if strings.Contains(slug, query) {
				matches = append(matches, filepath.Join(DocsDir(root), name))
			}
		}
	}

	sort.Strings(matches)

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no doc found matching %q", query)
	case 1:
		return matches[0], nil
	default:
		names := make([]string, len(matches))
		for i, p := range matches {
			names[i] = filepath.Base(p)
		}
		return "", fmt.Errorf("ambiguous: %q matches %s", query, strings.Join(names, ", "))
	}
}
