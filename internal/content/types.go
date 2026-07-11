// Package content defines the shared data types and content-index loading used
// across the render pipeline. Every other internal package depends on these
// types, so they are the stable contract between subsystems.
package content

import "time"

// Status distinguishes finished essays from drafts. Drafts are rendered to
// their own page but excluded from the index and from "related" listings.
type Status string

const (
	StatusFinished Status = "finished"
	StatusDraft    Status = "draft"
)

// Frontmatter is the parsed YAML header of an essay file.
type Frontmatter struct {
	Title               string   `yaml:"title"`
	Subtitle            string   `yaml:"subtitle"`
	Slug                string   `yaml:"slug"`
	Date                string   `yaml:"date"` // raw YYYY-MM-DD string as authored
	Tags                []string `yaml:"tags"`
	Status              Status   `yaml:"status"`
	ReadingTimeOverride *int     `yaml:"reading_time_override"`
}

// IndexEntry is the lightweight per-essay record held in the content index.
// It carries only what other essays need to resolve internal chips and the
// "related" block, without holding the full body in memory.
type IndexEntry struct {
	Slug     string
	Title    string
	Subtitle string
	Date     time.Time
	DateRaw  string
	Tags     []string
	Status   Status
	// SourcePath is the path to the source .md file (for error messages).
	SourcePath string
}

// Index maps slug -> IndexEntry for every essay in content/essays.
type Index struct {
	BySlug  map[string]*IndexEntry
	Ordered []*IndexEntry // sorted newest-first
}

// Get returns the entry for a slug, or nil if absent.
func (ix *Index) Get(slug string) *IndexEntry {
	if ix == nil || ix.BySlug == nil {
		return nil
	}
	return ix.BySlug[slug]
}

// Citation is one entry from content/citations.yaml, keyed by its cite key.
type Citation struct {
	Author string `yaml:"author"`
	Title  string `yaml:"title"`
	Source string `yaml:"source"`
	Year   string `yaml:"year"`
	URL    string `yaml:"url"`
}

// Essay is a fully parsed essay: frontmatter plus raw body bytes (markdown
// after the frontmatter block). BodyOffset is the line number in the original
// file where the body begins (1-based), used to report accurate line numbers
// in validation errors.
type Essay struct {
	Front      Frontmatter
	Body       []byte
	SourcePath string
	BodyOffset int
}
