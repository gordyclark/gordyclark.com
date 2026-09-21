package render

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gordyclark/gordyclark.com/internal/content"
)

// scaffoldCollection extends the kinds scaffold with collections/ and data/.
func scaffoldCollection(t *testing.T) (Options, string) {
	t.Helper()
	opts, tmp := scaffoldKinds(t)
	for _, d := range []string{"collections", "data"} {
		if err := os.MkdirAll(filepath.Join(opts.ContentDir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return opts, tmp
}

const collectionBody = `---
title: "Books"
slug: books
date: 2026-09-21
tags: [books]
status: finished
data: books.csv
---

A prologue paragraph.
`

const collectionCSV = `Title,Author,Genre,Notes
The Windup Girl,"Bacigalupi, Paolo",Science Fiction,Calorie exec in a starved Thailand
Bunny,"Awad, Mona",Literature,Mean Girls with rabbits
Unsong,"Alexander, Scott",Fantasy,Angels are real
`

// buildCollection writes the standard fixture pair and builds the site.
func buildCollection(t *testing.T) (Options, string) {
	t.Helper()
	opts, tmp := scaffoldCollection(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "collections", "books.md"), collectionBody)
	writeFileT(t, filepath.Join(opts.ContentDir, "data", "books.csv"), collectionCSV)
	if err := Build(opts); err != nil {
		t.Fatalf("build: %v", err)
	}
	return opts, tmp
}

func collectionHTML(t *testing.T, tmp string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(tmp, "static", "collections", "books", "index.html"))
	if err != nil {
		t.Fatalf("reading collection page: %v", err)
	}
	return string(b)
}

func TestCollectionWritesToCollectionsPath(t *testing.T) {
	_, tmp := buildCollection(t)
	if _, err := os.Stat(filepath.Join(tmp, "static", "collections", "books", "index.html")); err != nil {
		t.Fatalf("expected /collections/books/index.html: %v", err)
	}
}

func TestCollectionRendersEveryViewIntoOnePage(t *testing.T) {
	_, tmp := buildCollection(t)
	html := collectionHTML(t, tmp)

	for _, view := range []string{"author", "title", "genre"} {
		if !strings.Contains(html, `data-view="`+view+`"`) {
			t.Errorf("page is missing the %q view", view)
		}
	}
	// Three books in three views: every book is emitted once per view, which
	// is what lets the reader switch without a request.
	if got := strings.Count(html, `class="book"`); got != 9 {
		t.Errorf("rendered %d book entries, want 9 (3 books x 3 views)", got)
	}
}

func TestCollectionDefaultsToTheAuthorView(t *testing.T) {
	_, tmp := buildCollection(t)
	html := collectionHTML(t, tmp)

	if !strings.Contains(html, `id="sort-author" class="collection-sort-input" checked`) {
		t.Error("the author radio must be checked so the page renders a view before any interaction")
	}
	if got := strings.Count(html, " checked"); got != 1 {
		t.Errorf("%d radios are checked, want exactly 1", got)
	}
}

// The site's hard rule: nothing may move the reader's scroll position. The
// view switcher must therefore be pure CSS.
func TestCollectionHasNoScriptOrScrollHooks(t *testing.T) {
	_, tmp := buildCollection(t)
	html := collectionHTML(t, tmp)

	// Isolate the collection block so site-wide markup is not searched.
	start := strings.Index(html, `<div class="collection">`)
	if start < 0 {
		t.Fatal("no collection block on the page")
	}
	block := html[start:]

	for _, banned := range []string{"<script", "onclick", "onchange", "javascript:", ":target"} {
		if strings.Contains(strings.ToLower(block), banned) {
			t.Errorf("collection markup contains %q; view switching must be CSS-only", banned)
		}
	}
}

// Every jump link must point at a section that exists on the page, or the bar
// silently sends readers nowhere.
func TestCollectionJumpAnchorsResolve(t *testing.T) {
	_, tmp := buildCollection(t)
	html := collectionHTML(t, tmp)

	ids := map[string]bool{}
	for _, m := range regexp.MustCompile(`class="collection-section-heading" id="([^"]+)"`).FindAllStringSubmatch(html, -1) {
		ids[m[1]] = true
	}
	links := regexp.MustCompile(`class="collection-jump-item" href="#([^"]+)"`).FindAllStringSubmatch(html, -1)
	if len(links) == 0 {
		t.Fatal("no jump links rendered")
	}
	for _, m := range links {
		if !ids[m[1]] {
			t.Errorf("jump link points at #%s, which is not a section on the page", m[1])
		}
	}
}

func TestCollectionEmptyJumpSlotsAreNotLinks(t *testing.T) {
	_, tmp := buildCollection(t)
	html := collectionHTML(t, tmp)

	// No author surname here starts with Q, so its slot must be inert.
	empty := regexp.MustCompile(`<span class="collection-jump-item is-empty"[^>]*>Q</span>`)
	if !empty.MatchString(html) {
		t.Error("expected Q to render as an empty, unlinked jump slot")
	}
}

func TestCollectionGenrePillsUseTheTagPalette(t *testing.T) {
	_, tmp := buildCollection(t)
	html := collectionHTML(t, tmp)

	if !regexp.MustCompile(`class="book-genre tag tag-c\d"`).MatchString(html) {
		t.Error("genre pills should carry a tag palette class from tagColorClass")
	}
	// The same genre must take the same color everywhere it appears.
	want := tagColorClass("Literature")
	if !strings.Contains(html, `tag `+want+`">Literature<`) {
		t.Errorf("Literature pill should use %s", want)
	}
}

func TestCollectionEscapesDataAsText(t *testing.T) {
	opts, tmp := scaffoldCollection(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "collections", "books.md"), collectionBody)
	writeFileT(t, filepath.Join(opts.ContentDir, "data", "books.csv"),
		"Title,Author,Genre,Notes\n<script>alert(1)</script>,\"Awad, Mona\",Literature,Note\n")
	if err := Build(opts); err != nil {
		t.Fatalf("build: %v", err)
	}
	html := collectionHTML(t, tmp)
	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Error("CSV data must be escaped, not rendered as HTML")
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Error("expected the title to appear escaped")
	}
}

// A collection page's own prose must still render; the data is appended to it.
func TestCollectionKeepsItsMarkdownBody(t *testing.T) {
	_, tmp := buildCollection(t)
	html := collectionHTML(t, tmp)
	if !strings.Contains(html, "A prologue paragraph.") {
		t.Error("the page's markdown body should render above the generated views")
	}
}

// A collection is data-driven, not a list of level-2 headings, so the list
// post's numbered TOC must not appear.
func TestCollectionHasNoListTOC(t *testing.T) {
	_, tmp := buildCollection(t)
	html := collectionHTML(t, tmp)
	if strings.Contains(html, `class="list-toc"`) {
		t.Error("a collection page should not render the list-post TOC")
	}
}

// ---- Build-time validation ------------------------------------------------

func TestCollectionWithoutDataFieldFailsBuild(t *testing.T) {
	opts, _ := scaffoldCollection(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "collections", "books.md"), `---
title: "Books"
slug: books
date: 2026-09-21
status: finished
---

No data field.
`)
	err := Build(opts)
	if err == nil {
		t.Fatal("expected a build failure when a collection names no data file")
	}
	if !strings.Contains(err.Error(), "data") {
		t.Errorf("error should name the missing field, got: %v", err)
	}
}

func TestDataFieldOnNonCollectionFailsBuild(t *testing.T) {
	opts, _ := scaffoldCollection(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "lists", "l.md"), `---
title: "A List"
slug: a-list
date: 2026-09-21
status: finished
data: books.csv
---

## Item
`)
	err := Build(opts)
	if err == nil {
		t.Fatal("expected a build failure: only a collection renders a data file")
	}
	if !strings.Contains(err.Error(), "collection") {
		t.Errorf("error should explain that data belongs to a collection, got: %v", err)
	}
}

func TestCollectionWithMissingCSVFailsBuild(t *testing.T) {
	opts, _ := scaffoldCollection(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "collections", "books.md"), collectionBody)
	// No CSV written.
	if err := Build(opts); err == nil {
		t.Fatal("expected a build failure when the named CSV is absent")
	}
}

func TestCollectionWithEmptyCSVFailsBuild(t *testing.T) {
	opts, _ := scaffoldCollection(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "collections", "books.md"), collectionBody)
	writeFileT(t, filepath.Join(opts.ContentDir, "data", "books.csv"), "Title,Author,Genre,Notes\n")
	if err := Build(opts); err == nil {
		t.Fatal("expected a build failure when the CSV holds no books")
	}
}

// ---- Site integration -----------------------------------------------------

func TestCollectionAppearsOnHomepageWithCollectionURL(t *testing.T) {
	_, tmp := buildCollection(t)
	b, err := os.ReadFile(filepath.Join(tmp, "static", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `href="/collections/books/"`) {
		t.Error("the homepage should link the collection at its own URL")
	}
}

func TestCollectionSectionIndexIsPublished(t *testing.T) {
	_, tmp := buildCollection(t)
	if _, err := os.Stat(filepath.Join(tmp, "static", "collections", "index.html")); err != nil {
		t.Fatalf("expected a /collections/ section index: %v", err)
	}
}

// The books page moved from /lists/books/ to /collections/books/, so the old
// URL must keep working for existing links and bookmarks.
func TestOldBooksURLRedirectsToTheCollection(t *testing.T) {
	_, tmp := buildCollection(t)
	b, err := os.ReadFile(filepath.Join(tmp, "static", "_redirects"))
	if err != nil {
		t.Fatal(err)
	}
	redirects := string(b)
	for _, want := range []string{"/lists/books/ /collections/books/ 301", "/books/ /collections/books/ 301"} {
		if !strings.Contains(redirects, want) {
			t.Errorf("_redirects is missing %q", want)
		}
	}
}

// The CSS switches views with a sibling combinator (`#sort-x:checked ~ ...`),
// which only reaches elements that come *after* the radios and share their
// parent. If the radios were ever emitted below the views, every view would
// stay hidden and the page would render empty. This guards that ordering.
func TestCollectionRadiosPrecedeControlAndViews(t *testing.T) {
	_, tmp := buildCollection(t)
	html := collectionHTML(t, tmp)

	start := strings.Index(html, `<div class="collection">`)
	if start < 0 {
		t.Fatal("no collection block on the page")
	}
	block := html[start:]

	lastRadio := strings.LastIndex(block, `class="collection-sort-input"`)
	if lastRadio < 0 {
		t.Fatal("no sort radios rendered")
	}
	control := strings.Index(block, `class="collection-sort"`)
	firstView := strings.Index(block, `class="collection-view"`)

	if control < lastRadio {
		t.Error("the sort control must follow every radio, or :checked ~ cannot reach its labels")
	}
	if firstView < lastRadio {
		t.Error("the views must follow every radio, or :checked ~ cannot reveal them")
	}
}

// Each view carries its own jump bar so the links on screen always match the
// sections on screen.
func TestEachViewHasItsOwnJumpBar(t *testing.T) {
	_, tmp := buildCollection(t)
	html := collectionHTML(t, tmp)

	if got := strings.Count(html, `class="collection-jump"`); got != 3 {
		t.Errorf("rendered %d jump bars, want one per view (3)", got)
	}
}

// ---- Site header ----------------------------------------------------------

// The nav links the books collection directly, and the URL is derived from the
// content kind rather than hardcoded in the template.
func TestNavLinksBooksCollection(t *testing.T) {
	_, tmp := buildCollection(t)
	html := collectionHTML(t, tmp)

	want := content.KindCollection.URL(booksSlug)
	if !strings.Contains(html, `<a href="`+want+`">Books</a>`) {
		t.Errorf("nav should link Books at %s", want)
	}
}

// Back to top lives in the collection's own jump bar, beside the A-Z letters,
// not in the site nav. It is a plain anchor to an id that exists, so following
// it is the reader's own click and lands somewhere real.
func TestBackToTopIsAPlainAnchorWithATarget(t *testing.T) {
	_, tmp := buildCollection(t)
	html := collectionHTML(t, tmp)

	if !strings.Contains(html, `class="collection-jump-item collection-jump-top" href="#top"`) {
		t.Error("expected a back-to-top anchor in the jump bar pointing at #top")
	}
	// It belongs to the jump bar; the site-wide nav must not carry one.
	if strings.Contains(html, `class="to-top"`) {
		t.Error("back-to-top should not be in the site header")
	}
	if !strings.Contains(html, `<body id="top">`) {
		t.Error("#top must exist, or the back-to-top link goes nowhere")
	}
	// Anything scripted here would move the reader's scroll position.
	for _, banned := range []string{"onclick", "scrollTo", "javascript:"} {
		if strings.Contains(html, banned) {
			t.Errorf("back-to-top must be CSS/anchor only, found %q", banned)
		}
	}
}

// Every page carries the header, not just collections.
func TestNavAppearsOnEveryContentKind(t *testing.T) {
	opts, tmp := scaffoldCollection(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "blog", "p.md"), textBody)
	writeFileT(t, filepath.Join(opts.ContentDir, "lists", "l.md"), listBody)
	writeFileT(t, filepath.Join(opts.ContentDir, "collections", "books.md"), collectionBody)
	writeFileT(t, filepath.Join(opts.ContentDir, "data", "books.csv"), collectionCSV)
	if err := Build(opts); err != nil {
		t.Fatal(err)
	}

	pages := []string{
		filepath.Join("blog", "a-text-post", "index.html"),
		filepath.Join("lists", "a-list", "index.html"),
		filepath.Join("collections", "books", "index.html"),
		"index.html",
	}
	for _, page := range pages {
		b, err := os.ReadFile(filepath.Join(tmp, "static", page))
		if err != nil {
			t.Fatalf("reading %s: %v", page, err)
		}
		if strings.Contains(string(b), `class="to-top"`) {
			t.Errorf("%s has a back-to-top link in the site header; it belongs to the collection jump bar", page)
		}
		if !strings.Contains(string(b), `>Books</a>`) {
			t.Errorf("%s is missing the Books nav link", page)
		}
	}
}

// Each view has its own jump bar, so each needs its own Top link — otherwise
// switching to the genre view would lose the way back.
func TestEveryJumpBarEndsWithBackToTop(t *testing.T) {
	_, tmp := buildCollection(t)
	html := collectionHTML(t, tmp)

	if got := strings.Count(html, `collection-jump-top`); got != 3 {
		t.Errorf("found %d back-to-top links, want one per view (3)", got)
	}

	// It must come last in each bar, after the letters, or it reads as a
	// section jump rather than the way out.
	bars := regexp.MustCompile(`(?s)<nav class="collection-jump".*?</nav>`).FindAllString(html, -1)
	if len(bars) != 3 {
		t.Fatalf("found %d jump bars, want 3", len(bars))
	}
	for i, bar := range bars {
		top := strings.Index(bar, "collection-jump-top")
		if top < 0 {
			t.Errorf("bar %d has no back-to-top link", i)
			continue
		}
		// Nothing may follow it but its own markup, so the last anchor opening
		// in the bar must be the Top link's own.
		if last := strings.LastIndex(bar, `<a class="collection-jump-item`); last != top-len(`<a class="collection-jump-item `) {
			t.Errorf("bar %d has a link after back-to-top; Top should close the bar", i)
		}
	}
}
