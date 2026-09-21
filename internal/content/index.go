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

// LoadIndex walks the *.md files in essaysDir (non-recursively) and builds an
// Index of KindEssay documents. It is shorthand for LoadIndexKind with
// KindEssay.
func LoadIndex(essaysDir string) (*Index, error) {
	return LoadIndexKind(essaysDir, KindEssay)
}

// LoadSiteIndex builds one merged index over every content kind under
// contentDir. A missing directory for a kind is not an error (the site may
// simply have no list posts yet); any other read error is.
//
// The index is deliberately a single flat slug namespace across all kinds, so
// a slug collision between, say, an essay and a blog post is reported at build
// time rather than silently producing two pages that internal links cannot
// tell apart.
func LoadSiteIndex(contentDir string) (*Index, error) {
	merged := &Index{BySlug: make(map[string]*IndexEntry)}
	// slugSource records which file first claimed a slug, for duplicate errors.
	slugSource := make(map[string]string)

	for _, kind := range AllKinds {
		dir := filepath.Join(contentDir, DirForKind(kind))
		ix, err := LoadIndexKind(dir, kind)
		if err != nil {
			if os.IsNotExist(err) {
				continue // no content of this kind yet
			}
			return nil, err
		}
		for _, e := range ix.Ordered {
			if prev, ok := slugSource[e.Slug]; ok {
				return nil, fmt.Errorf("duplicate slug %q in %s and %s", e.Slug, prev, e.SourcePath)
			}
			slugSource[e.Slug] = e.SourcePath
			merged.BySlug[e.Slug] = e
			merged.Ordered = append(merged.Ordered, e)
		}
	}

	sortEntries(merged.Ordered)
	return merged, nil
}

// LoadIndexKind walks the *.md files in dir (non-recursively), parses each
// file's frontmatter, and builds an Index keyed by slug and ordered
// newest-date-first (ties broken by slug ascending). Every entry is tagged
// with the given kind, which is what lets callers build correct URLs.
func LoadIndexKind(dir string, kind Kind) (*Index, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	essaysDir := dir

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

		fm, err := ParseFrontmatterOfKind(path, kind)
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
			Kind:       kind,
			SourcePath: path,
		}
		ix.BySlug[fm.Slug] = entry
		ix.Ordered = append(ix.Ordered, entry)
	}

	sortEntries(ix.Ordered)
	return ix, nil
}

// sortEntries orders entries newest-date-first, ties broken by slug ascending.
func sortEntries(entries []*IndexEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if !a.Date.Equal(b.Date) {
			return a.Date.After(b.Date) // newest first
		}
		return a.Slug < b.Slug // tie-break by slug ascending
	})
}
