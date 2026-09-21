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

// Kind is the content type of a document. It is determined by which directory
// under content/ the source file lives in, never guessed from frontmatter, so
// there is exactly one way to classify a document: move the file.
type Kind string

const (
	KindEssay Kind = "essay" // content/essays -> /essays/<slug>/
	KindText  Kind = "text"  // content/blog   -> /blog/<slug>/
	KindList  Kind = "list"  // content/lists  -> /lists/<slug>/
)

// DirForKind maps a Kind to its directory name under the content root.
func DirForKind(k Kind) string {
	switch k {
	case KindText:
		return "blog"
	case KindList:
		return "lists"
	default:
		return "essays"
	}
}

// URLPrefix returns the leading path segment for a kind's published URLs, with
// no surrounding slashes ("essays", "blog", "lists"). Every URL the site emits
// for a document derives from this, so adding a content type means adding a
// case here and nowhere else.
func (k Kind) URLPrefix() string { return DirForKind(k) }

// URL returns the absolute site path for a document of this kind, e.g.
// "/lists/seven-things/".
func (k Kind) URL(slug string) string { return "/" + k.URLPrefix() + "/" + slug + "/" }

// Label returns the human-readable section name used in nav and breadcrumbs.
func (k Kind) Label() string {
	switch k {
	case KindText:
		return "Blog"
	case KindList:
		return "Lists"
	default:
		return "Essays"
	}
}

// Frontmatter is the parsed YAML header of an essay file.
type Frontmatter struct {
	Title               string   `yaml:"title"`
	Subtitle            string   `yaml:"subtitle"`
	Slug                string   `yaml:"slug"`
	Date                string   `yaml:"date"` // raw YYYY-MM-DD string as authored
	Tags                []string `yaml:"tags"`
	Status              Status   `yaml:"status"`
	ReadingTimeOverride *int     `yaml:"reading_time_override"`
	// Author defaults to DefaultAuthor when unset.
	Author string `yaml:"author"`
	// Hero is an optional full-width image shown above the article body.
	// HeroAlt is required whenever Hero is set, so a hero image can never ship
	// without alt text.
	Hero    string `yaml:"hero"`
	HeroAlt string `yaml:"hero_alt"`
}

// DefaultAuthor is used when a document omits the author field. This is a
// single-author site, so an omitted author is the common case, not an error.
const DefaultAuthor = "Gordy Clark"

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
	// Kind is the content type, derived from the source directory. It is what
	// makes a merged cross-type index able to emit correct URLs.
	Kind Kind
	// SourcePath is the path to the source .md file (for error messages).
	SourcePath string
}

// URL returns the absolute site path for this entry.
func (e *IndexEntry) URL() string { return e.Kind.URL(e.Slug) }

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
	// Kind is the content type, set by the caller from the source directory.
	Kind Kind
}
