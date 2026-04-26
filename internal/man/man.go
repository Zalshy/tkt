package man

import (
	"embed"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed pages/*.md
var pagesFS embed.FS

type Page struct {
	Name  string
	Title string
	Body  string
}

func ListPages() ([]Page, error) {
	entries, err := pagesFS.ReadDir("pages")
	if err != nil {
		return nil, fmt.Errorf("man: read pages: %w", err)
	}
	pages := make([]Page, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".md")
		page, err := ReadPage(name)
		if err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].Name < pages[j].Name })
	return pages, nil
}

func ReadPage(name string) (Page, error) {
	name = NormalizeName(name)
	if name == "" {
		return Page{}, fmt.Errorf("man: page name is required")
	}
	data, err := pagesFS.ReadFile("pages/" + name + ".md")
	if err != nil {
		return Page{}, fmt.Errorf("man: page %q not found. Run: tkt man", name)
	}
	body := strings.TrimSpace(string(data)) + "\n"
	return Page{Name: name, Title: titleFromBody(name, body), Body: body}, nil
}

func NormalizeName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = strings.TrimPrefix(name, "tkt ")
	name = strings.ReplaceAll(name, "_", "-")
	if name == "llm" {
		return "minimal"
	}
	return name
}

func titleFromBody(name, body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return name
}
