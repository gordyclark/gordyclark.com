package content

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// dateLayout is the raw date format authored in frontmatter.
const dateLayout = "2006-01-02"

// LoadIndex walks the *.md files in essaysDir (non-recursively), parses each
// file's frontmatter, and builds an Index keyed by slug and ordered
// newest-date-first (ties broken by slug ascending).
func LoadIndex(essaysDir string) (*Index, error) {
	entries, err := os.ReadDir(essaysDir)
	if err != nil {
		return nil, err
	}

	ix := &Index{
		BySlug:  make(map[string]*IndexEntry),
		Ordered: nil,
	}
	// slugSource records which file first claimed a slug, for duplicate errors.
	slugSource := make(map[string]string)

	for _, de := range entries {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		if filepath.Ext(name) != ".md" {
			continue
		}
		path := filepath.Join(essaysDir, name)

		fm, err := ParseFrontmatter(path)
		if err != nil {
			return nil, err
		}

		date, err := time.Parse(dateLayout, fm.Date)
		if err != nil {
			return nil, fmt.Errorf("%s: invalid date %q (want YYYY-MM-DD): %w", path, fm.Date, err)
		}

		if prev, ok := slugSource[fm.Slug]; ok {
			return nil, fmt.Errorf("duplicate slug %q in %s and %s", fm.Slug, prev, path)
		}
		slugSource[fm.Slug] = path

		entry := &IndexEntry{
			Slug:       fm.Slug,
			Title:      fm.Title,
			Subtitle:   fm.Subtitle,
			Date:       date,
			DateRaw:    fm.Date,
			Tags:       fm.Tags,
			Status:     fm.Status,
			SourcePath: path,
		}
		ix.BySlug[fm.Slug] = entry
		ix.Ordered = append(ix.Ordered, entry)
	}

	sort.SliceStable(ix.Ordered, func(i, j int) bool {
		a, b := ix.Ordered[i], ix.Ordered[j]
		if !a.Date.Equal(b.Date) {
			return a.Date.After(b.Date) // newest first
		}
		return a.Slug < b.Slug // tie-break by slug ascending
	})

	return ix, nil
}
