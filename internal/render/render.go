// Package render is the integration layer of the static essay-site generator.
// It ties together the content, margin, highlight and diagrams packages: it
// loads the content index and citations, parses the essay bodies through
// goldmark with the site's custom extensions, relocates footnotes and margin
// links into a right-hand margin column, and writes the final static HTML tree
// (essays, index, per-tag pages) plus a content-hashed CSS bundle and copied
// assets.
package render

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gordyclark/gordyclark.com/internal/charts"
	"github.com/gordyclark/gordyclark.com/internal/content"
	"github.com/gordyclark/gordyclark.com/internal/diagrams"
)

// Options configures a build.
type Options struct {
	// ContentDir is the root of the content tree (holds essays/ and
	// citations.yaml).
	ContentDir string
	// OutDir is where the static site is written (e.g. ./static).
	OutDir string
	// CacheDir is where rendered diagrams are cached (e.g. ./.cache).
	CacheDir string
	// AssetsDir holds css/ and fonts/. Defaults to "assets" (relative to CWD)
	// when empty; tests set it explicitly.
	AssetsDir string
	// TemplatesDir holds the *.html.tmpl files. Defaults to "templates"
	// when empty.
	TemplatesDir string
	// BooksCSV is the reading-log CSV rendered into the /books/ page. Defaults
	// to "books.csv" (repo root). If the file is absent, the books page is
	// skipped (not an error) so the site still builds without it.
	BooksCSV string
	// BooksSeed seeds the random "featured book" pick on the /books/ page.
	// cmd/render passes a wall-clock value so the pick changes each build; a
	// fixed value makes it deterministic (used in tests).
	BooksSeed int64
}

// imageExts are the content image extensions mirrored into the output tree.
var imageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".svg": true,
	".gif": true, ".webp": true,
}

// Build runs the full render pipeline described in SPEC §3.2. It returns a
// non-nil, actionable error on the first failure.
func Build(opts Options) error {
	if opts.AssetsDir == "" {
		opts.AssetsDir = "assets"
	}
	if opts.TemplatesDir == "" {
		opts.TemplatesDir = "templates"
	}
	if opts.BooksCSV == "" {
		opts.BooksCSV = "books.csv"
	}
	templatesDir = opts.TemplatesDir

	// (a) Load one merged content index spanning essays, blog posts and lists.
	// A single index means internal links and "related" resolve across every
	// content type, and slug collisions between types are caught here.
	ix, err := content.LoadSiteIndex(opts.ContentDir)
	if err != nil {
		return fmt.Errorf("loading content index: %w", err)
	}

	// (b) Load citations.
	cites, err := content.LoadCitations(filepath.Join(opts.ContentDir, "citations.yaml"))
	if err != nil {
		return fmt.Errorf("loading citations: %w", err)
	}

	// (c) Parse the template set once.
	tmpl, err := template.ParseFiles(
		filepath.Join(opts.TemplatesDir, "base.html.tmpl"),
		filepath.Join(opts.TemplatesDir, "essay.html.tmpl"),
		filepath.Join(opts.TemplatesDir, "index.html.tmpl"),
	)
	if err != nil {
		return fmt.Errorf("parsing templates: %w", err)
	}

	// (d) Build the content-hashed CSS bundle.
	stylesheet, err := buildCSS(opts.AssetsDir, opts.OutDir)
	if err != nil {
		return fmt.Errorf("building css: %w", err)
	}

	dr := diagrams.New(opts.CacheDir)
	cr := charts.New(opts.CacheDir)

	// (e) Render every document of every kind. Each kind renders through the
	// same pipeline and differs only in its page template and URL prefix.
	for _, kind := range []content.Kind{content.KindEssay, content.KindText, content.KindList} {
		dir := filepath.Join(opts.ContentDir, content.DirForKind(kind))
		files, err := listContentFiles(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue // no content of this kind yet
			}
			return err
		}
		for _, path := range files {
			doc, err := content.ParseDoc(path, kind)
			if err != nil {
				return err // missing/invalid frontmatter, already actionable
			}
			articleHTML, meta, err := renderEssay(doc, ix, cites, dr, cr)
			if err != nil {
				return err
			}
			if err := writeDocPage(tmpl, opts.OutDir, stylesheet, kind, meta, articleHTML); err != nil {
				return err
			}
		}
	}

	// (f) Homepage (all finished content, newest first) + per-tag pages.
	// Preserve the index's newest-first ordering by filtering ix.Ordered.
	var indexEntries []*content.IndexEntry
	for _, e := range ix.Ordered {
		if e.Status == content.StatusFinished {
			indexEntries = append(indexEntries, e)
		}
	}
	if err := writeIndexPage(tmpl, opts.OutDir, stylesheet, "Writing", indexEntries, "", homeIntro); err != nil {
		return err
	}
	// Per-kind section index pages, e.g. /blog/ and /lists/.
	for _, kind := range []content.Kind{content.KindEssay, content.KindText, content.KindList} {
		var forKind []*content.IndexEntry
		for _, e := range indexEntries {
			if e.Kind == kind {
				forKind = append(forKind, e)
			}
		}
		if len(forKind) == 0 {
			continue // do not publish an empty section page
		}
		if err := writeIndexPage(tmpl, opts.OutDir, stylesheet, kind.Label(), forKind, kind.URLPrefix(), ""); err != nil {
			return err
		}
	}
	if err := writeTagPages(tmpl, opts.OutDir, stylesheet, indexEntries); err != nil {
		return err
	}

	// (f2) Books page — re-reads Books.csv every build so new entries flow into
	// the charts and table automatically. Skipped (not fatal) if the CSV is
	// absent, so the site still builds without a reading log.
	if _, statErr := os.Stat(opts.BooksCSV); statErr == nil {
		if err := writeBooksPage(tmpl, opts.OutDir, stylesheet, opts.BooksCSV, cr, opts.BooksSeed); err != nil {
			return err
		}
	}

	// (g) Copy assets: fonts and any content images.
	if err := copyFonts(opts.AssetsDir, opts.OutDir); err != nil {
		return err
	}
	if err := copyContentImages(opts.ContentDir, opts.OutDir); err != nil {
		return err
	}

	return nil
}

// pageData is the data passed to the "base" entrypoint template. The embedded
// per-page struct supplies the fields the "main" block reads.
type pageData struct {
	Title          string
	Subtitle       string
	StylesheetPath string
	// article page fields
	ArticleHTML template.HTML
	Related     []relatedEssay
	// Byline fields, shown on text and list posts.
	Author  string
	DateRaw string
	Tags    []string
	// Hero is the optional full-width image above the article.
	Hero    string
	HeroAlt string
	// TOC is the auto-generated table of contents on list posts.
	TOC template.HTML
	// index page fields
	Heading string
	Essays  []*content.IndexEntry
	Intro   template.HTML // homepage "about me" blurb; empty on tag pages
}

// pageTemplateFor maps a content kind to the template file defining its "main"
// block.
func pageTemplateFor(kind content.Kind) string {
	switch kind {
	case content.KindText:
		return "text.html.tmpl"
	case content.KindList:
		return "list.html.tmpl"
	default:
		return "essay.html.tmpl"
	}
}

// writeDocPage renders one document to <outDir>/<prefix>/<slug>/index.html,
// where the prefix and the page template both derive from the content kind.
func writeDocPage(tmpl *template.Template, outDir, stylesheet string, kind content.Kind, meta essayMeta, article template.HTML) error {
	data := pageData{
		Title:          meta.Title,
		Subtitle:       meta.Subtitle,
		StylesheetPath: stylesheet,
		ArticleHTML:    article,
		Related:        meta.Related,
		Author:         meta.Author,
		DateRaw:        meta.DateRaw,
		Tags:           meta.Tags,
		Hero:           meta.Hero,
		HeroAlt:        meta.HeroAlt,
		TOC:            meta.TOC,
	}
	set, err := templateSetFor(tmpl, pageTemplateFor(kind))
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := set.ExecuteTemplate(&buf, "base", data); err != nil {
		return fmt.Errorf("executing %s template for %q: %w", kind, meta.Slug, err)
	}
	dir := filepath.Join(outDir, kind.URLPrefix(), meta.Slug)
	return writeFile(filepath.Join(dir, "index.html"), buf.Bytes())
}

// homeIntro is the short "about me" blurb shown at the top of the homepage
// (and only the homepage — tag pages pass an empty intro). It is authored here
// as trusted HTML rather than as a content file so the site has no standalone
// About page. Edit the copy freely.
const homeIntro template.HTML = `<p>I'm Gordy Clark. I build software and, occasionally, write about how I ` +
	`build it — the boring, durable kind that keeps working after you stop paying ` +
	`attention to it. These essays are mostly notes to myself about tools, ` +
	`tradeoffs, and the value of keeping systems small enough to hold in your head.</p>`

func writeIndexPage(tmpl *template.Template, outDir, stylesheet, heading string, essays []*content.IndexEntry, subdir string, intro template.HTML) error {
	data := pageData{
		Title:          heading,
		StylesheetPath: stylesheet,
		Heading:        heading,
		Essays:         essays,
		Intro:          intro,
	}
	set, err := templateSetFor(tmpl, "index.html.tmpl")
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := set.ExecuteTemplate(&buf, "base", data); err != nil {
		return fmt.Errorf("executing index template (%q): %w", heading, err)
	}
	var path string
	if subdir == "" {
		path = filepath.Join(outDir, "index.html")
	} else {
		path = filepath.Join(outDir, subdir, "index.html")
	}
	return writeFile(path, buf.Bytes())
}

func writeTagPages(tmpl *template.Template, outDir, stylesheet string, finished []*content.IndexEntry) error {
	// Collect essays per tag, preserving newest-first order (finished is already
	// newest-first).
	byTag := map[string][]*content.IndexEntry{}
	var tagOrder []string
	seen := map[string]bool{}
	for _, e := range finished {
		for _, t := range e.Tags {
			if !seen[t] {
				seen[t] = true
				tagOrder = append(tagOrder, t)
			}
			byTag[t] = append(byTag[t], e)
		}
	}
	sort.Strings(tagOrder)
	for _, tag := range tagOrder {
		heading := "Tagged: " + tag
		subdir := filepath.Join("tags", tag)
		if err := writeIndexPage(tmpl, outDir, stylesheet, heading, byTag[tag], subdir, ""); err != nil {
			return err
		}
	}
	return nil
}

// templateSetFor returns a template set whose "main" block is defined by the
// given page template file. Because base.html.tmpl invokes {{block "main"}} and
// both essay.html.tmpl and index.html.tmpl define "main", we must select the
// right "main" per page. We do this by re-parsing base + the chosen page file
// into a fresh set. The templates are tiny so this is cheap.
func templateSetFor(_ *template.Template, pageFile string) (*template.Template, error) {
	// Both essay.html.tmpl and index.html.tmpl define a "main" block; when all
	// three files are parsed into one set the last-parsed "main" wins, which is
	// not what we want. So we rebuild a fresh set from just base + the chosen
	// page file, guaranteeing the correct "main" is bound. The templates are
	// tiny so re-parsing per page is cheap.
	set, err := template.ParseFiles(
		filepath.Join(templatesDir, "base.html.tmpl"),
		filepath.Join(templatesDir, pageFile),
	)
	if err != nil {
		return nil, fmt.Errorf("re-parsing templates for %s: %w", pageFile, err)
	}
	return set, nil
}

// templatesDir is set at the start of Build so templateSetFor can re-parse the
// right files. Build is not run concurrently within a process for this tool.
var templatesDir string

func listContentFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Returned unwrapped so callers can test it with os.IsNotExist: a
		// content directory that does not exist yet is not an error.
		return nil, err
	}
	var out []string
	for _, de := range entries {
		if de.IsDir() || filepath.Ext(de.Name()) != ".md" {
			continue
		}
		out = append(out, filepath.Join(dir, de.Name()))
	}
	sort.Strings(out)
	return out, nil
}

func copyFonts(assetsDir, outDir string) error {
	src := filepath.Join(assetsDir, "fonts")
	entries, err := os.ReadDir(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading fonts dir: %w", err)
	}
	for _, de := range entries {
		if de.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, de.Name()))
		if err != nil {
			return fmt.Errorf("reading font %q: %w", de.Name(), err)
		}
		if err := writeFile(filepath.Join(outDir, "fonts", de.Name()), data); err != nil {
			return err
		}
	}
	return nil
}

// copyContentImages mirrors any image files found under contentDir into outDir,
// preserving their path relative to contentDir.
func copyContentImages(contentDir, outDir string) error {
	return filepath.WalkDir(contentDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !imageExts[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		rel, err := filepath.Rel(contentDir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading image %q: %w", path, err)
		}
		return writeFile(filepath.Join(outDir, rel), data)
	})
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating dir for %q: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing %q: %w", path, err)
	}
	return nil
}
