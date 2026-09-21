package render

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// scaffoldKinds extends scaffold with the blog/ and lists/ content dirs.
func scaffoldKinds(t *testing.T) (Options, string) {
	t.Helper()
	opts, tmp := scaffold(t)
	for _, d := range []string{"blog", "lists"} {
		if err := os.MkdirAll(filepath.Join(opts.ContentDir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return opts, tmp
}

const listBody = `---
title: "A List"
slug: a-list
date: 2026-09-21
tags: [lists]
status: finished
---

An intro paragraph.

## First Thing

Body of the first item.

## Second Thing

Body of the second item.
`

const textBody = `---
title: "A Text Post"
slug: a-text-post
date: 2026-09-20
tags: [blog]
status: finished
---

Just a paragraph.
`

// ---- Output paths ---------------------------------------------------------

func TestTextPostWritesToBlogPath(t *testing.T) {
	opts, tmp := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "blog", "post.md"), textBody)
	if err := Build(opts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "static", "blog", "a-text-post", "index.html")); err != nil {
		t.Fatalf("expected /blog/a-text-post/index.html: %v", err)
	}
}

func TestListPostWritesToListsPath(t *testing.T) {
	opts, tmp := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "lists", "l.md"), listBody)
	if err := Build(opts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "static", "lists", "a-list", "index.html")); err != nil {
		t.Fatalf("expected /lists/a-list/index.html: %v", err)
	}
}

// A missing content directory is normal (the site may have no lists yet) and
// must not fail the build.
func TestBuildSucceedsWithoutNewContentDirs(t *testing.T) {
	opts, _ := scaffold(t) // only essays/ exists
	writeFileT(t, filepath.Join(opts.ContentDir, "essays", "e.md"), `---
title: "E"
slug: e
date: 2026-09-01
status: finished
---

Body.
`)
	if err := Build(opts); err != nil {
		t.Fatalf("build should tolerate missing blog/ and lists/: %v", err)
	}
}

// ---- Table of contents ----------------------------------------------------

func TestListPostGeneratesTOCWithMatchingAnchors(t *testing.T) {
	opts, tmp := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "lists", "l.md"), listBody)
	if err := Build(opts); err != nil {
		t.Fatal(err)
	}
	html := readOut(t, tmp, filepath.Join("lists", "a-list", "index.html"))

	for _, want := range []string{
		`class="list-toc"`,
		`<a href="#first-thing">First Thing</a>`,
		`<a href="#second-thing">Second Thing</a>`,
		`<h2 id="first-thing"`,
		`<h2 id="second-thing"`,
		`<span class="list-item-number">1.</span>`,
		`<span class="list-item-number">2.</span>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("list page missing %q", want)
		}
	}
}

// The TOC must never introduce scroll-moving behaviour.
func TestListTOCHasNoScriptOrScrollHooks(t *testing.T) {
	opts, tmp := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "lists", "l.md"), listBody)
	if err := Build(opts); err != nil {
		t.Fatal(err)
	}
	html := strings.ToLower(readOut(t, tmp, filepath.Join("lists", "a-list", "index.html")))
	for _, forbidden := range []string{"<script", "onclick=", "scrollto", "scrollintoview"} {
		if strings.Contains(html, forbidden) {
			t.Errorf("list page should contain no %q", forbidden)
		}
	}
}

// A text post is not a list, so it gets no table of contents.
func TestTextPostHasNoTOC(t *testing.T) {
	opts, tmp := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "blog", "p.md"), textBody)
	if err := Build(opts); err != nil {
		t.Fatal(err)
	}
	html := readOut(t, tmp, filepath.Join("blog", "a-text-post", "index.html"))
	if strings.Contains(html, `class="list-toc"`) {
		t.Error("a text post should not have a table of contents")
	}
}

// ---- img blocks -----------------------------------------------------------

func imgPost(block string) string {
	return `---
title: "Img"
slug: img-post
date: 2026-09-21
status: finished
---

` + block + `
`
}

func TestImgBlockAlignments(t *testing.T) {
	for _, align := range []string{"left", "right", "center"} {
		opts, tmp := scaffoldKinds(t)
		writeFileT(t, filepath.Join(opts.ContentDir, "blog", "i.md"), imgPost(
			"```img\nsrc=\"/img/a.png\"\nalign=\""+align+"\"\nalt=\"Alt text\"\n```"))
		if err := Build(opts); err != nil {
			t.Fatalf("align=%s: %v", align, err)
		}
		html := readOut(t, tmp, filepath.Join("blog", "img-post", "index.html"))
		if !strings.Contains(html, `class="post-img align-`+align+`"`) {
			t.Errorf("align=%s: missing alignment class", align)
		}
		if !strings.Contains(html, `alt="Alt text"`) {
			t.Errorf("align=%s: missing alt text", align)
		}
	}
}

func TestImgBlockCaption(t *testing.T) {
	opts, tmp := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "blog", "i.md"), imgPost(
		"```img\nsrc=\"/img/a.png\"\nalt=\"Alt\"\ncaption=\"Hello\"\n```"))
	if err := Build(opts); err != nil {
		t.Fatal(err)
	}
	html := readOut(t, tmp, filepath.Join("blog", "img-post", "index.html"))
	if !strings.Contains(html, "<figcaption>Hello</figcaption>") {
		t.Error("caption missing from rendered img block")
	}
}

// An image with no alt text must fail the build, not ship unlabelled.
func TestImgBlockMissingAltFailsBuild(t *testing.T) {
	opts, _ := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "blog", "i.md"), imgPost(
		"```img\nsrc=\"/img/a.png\"\n```"))
	err := Build(opts)
	if err == nil {
		t.Fatal("expected the build to fail when alt is missing")
	}
	if !strings.Contains(err.Error(), "alt") {
		t.Errorf("error should name the missing attribute, got: %v", err)
	}
}

func TestImgBlockBadAlignFailsBuild(t *testing.T) {
	opts, _ := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "blog", "i.md"), imgPost(
		"```img\nsrc=\"/img/a.png\"\nalt=\"Alt\"\nalign=\"middle\"\n```"))
	err := Build(opts)
	if err == nil {
		t.Fatal("expected the build to fail for an invalid align value")
	}
	if !strings.Contains(err.Error(), "middle") {
		t.Errorf("error should quote the bad value, got: %v", err)
	}
}

// img blocks must work inside list posts too, not only text posts.
func TestImgBlockInsideListPost(t *testing.T) {
	opts, tmp := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "lists", "l.md"), `---
title: "A List"
slug: a-list
date: 2026-09-21
status: finished
---

## Item One

`+"```img\nsrc=\"/img/a.png\"\nalign=\"left\"\nalt=\"Alt\"\n```"+`
`)
	if err := Build(opts); err != nil {
		t.Fatal(err)
	}
	html := readOut(t, tmp, filepath.Join("lists", "a-list", "index.html"))
	if !strings.Contains(html, `class="post-img align-left"`) {
		t.Error("img block did not render inside a list post")
	}
}

// ---- Frontmatter ----------------------------------------------------------

func TestHeroRequiresAltText(t *testing.T) {
	opts, _ := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "blog", "p.md"), `---
title: "Hero"
slug: hero-post
date: 2026-09-21
hero: /img/a.png
status: finished
---

Body.
`)
	err := Build(opts)
	if err == nil {
		t.Fatal("expected the build to fail when hero has no hero_alt")
	}
	if !strings.Contains(err.Error(), "hero_alt") {
		t.Errorf("error should name hero_alt, got: %v", err)
	}
}

func TestBylineUsesDefaultAuthorWhenOmitted(t *testing.T) {
	opts, tmp := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "blog", "p.md"), textBody)
	if err := Build(opts); err != nil {
		t.Fatal(err)
	}
	html := readOut(t, tmp, filepath.Join("blog", "a-text-post", "index.html"))
	if !strings.Contains(html, "Gordy Clark") {
		t.Error("byline should fall back to the default author")
	}
}

func TestExplicitAuthorIsUsed(t *testing.T) {
	opts, tmp := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "blog", "p.md"), `---
title: "By Someone"
slug: by-someone
date: 2026-09-21
author: A Guest Writer
status: finished
---

Body.
`)
	if err := Build(opts); err != nil {
		t.Fatal(err)
	}
	html := readOut(t, tmp, filepath.Join("blog", "by-someone", "index.html"))
	if !strings.Contains(html, "A Guest Writer") {
		t.Error("explicit author should appear in the byline")
	}
}

// ---- Cross-type behaviour -------------------------------------------------

// One flat slug namespace: a collision between two kinds is a build error, not
// two pages that internal links cannot tell apart.
func TestDuplicateSlugAcrossKindsFailsBuild(t *testing.T) {
	opts, _ := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "essays", "e.md"), `---
title: "Essay"
slug: same-slug
date: 2026-09-21
status: finished
---

Body.
`)
	writeFileT(t, filepath.Join(opts.ContentDir, "blog", "b.md"), `---
title: "Blog"
slug: same-slug
date: 2026-09-21
status: finished
---

Body.
`)
	err := Build(opts)
	if err == nil {
		t.Fatal("expected a duplicate-slug error across content kinds")
	}
	if !strings.Contains(err.Error(), "same-slug") {
		t.Errorf("error should name the duplicate slug, got: %v", err)
	}
}

// The homepage is a unified feed, so each entry must link to its own section.
func TestHomepageLinksEachKindToItsOwnURL(t *testing.T) {
	opts, tmp := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "essays", "e.md"), `---
title: "An Essay"
slug: an-essay
date: 2026-09-19
status: finished
---

Body.
`)
	writeFileT(t, filepath.Join(opts.ContentDir, "blog", "b.md"), textBody)
	writeFileT(t, filepath.Join(opts.ContentDir, "lists", "l.md"), listBody)
	if err := Build(opts); err != nil {
		t.Fatal(err)
	}
	home := readOut(t, tmp, "index.html")
	for _, want := range []string{
		`href="/essays/an-essay/"`,
		`href="/blog/a-text-post/"`,
		`href="/lists/a-list/"`,
	} {
		if !strings.Contains(home, want) {
			t.Errorf("homepage missing %q", want)
		}
	}
}

// Section index pages are published per kind.
func TestSectionIndexPages(t *testing.T) {
	opts, tmp := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "blog", "b.md"), textBody)
	writeFileT(t, filepath.Join(opts.ContentDir, "lists", "l.md"), listBody)
	if err := Build(opts); err != nil {
		t.Fatal(err)
	}
	blog := readOut(t, tmp, filepath.Join("blog", "index.html"))
	if !strings.Contains(blog, `href="/blog/a-text-post/"`) {
		t.Error("/blog/ index should list the text post")
	}
	if strings.Contains(blog, `href="/lists/a-list/"`) {
		t.Error("/blog/ index should not list content from another section")
	}
	lists := readOut(t, tmp, filepath.Join("lists", "index.html"))
	if !strings.Contains(lists, `href="/lists/a-list/"`) {
		t.Error("/lists/ index should list the list post")
	}
}

// A tag shared across kinds lists all of them, each with a correct URL.
func TestTagPageSpansContentKinds(t *testing.T) {
	opts, tmp := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "essays", "e.md"), `---
title: "Essay"
slug: tagged-essay
date: 2026-09-19
tags: [shared]
status: finished
---

Body.
`)
	writeFileT(t, filepath.Join(opts.ContentDir, "lists", "l.md"), `---
title: "List"
slug: tagged-list
date: 2026-09-21
tags: [shared]
status: finished
---

## Item
`)
	if err := Build(opts); err != nil {
		t.Fatal(err)
	}
	page := readOut(t, tmp, filepath.Join("tags", "shared", "index.html"))
	for _, want := range []string{`href="/essays/tagged-essay/"`, `href="/lists/tagged-list/"`} {
		if !strings.Contains(page, want) {
			t.Errorf("shared tag page missing %q", want)
		}
	}
}

// An internal .margin chip must resolve to a non-essay target's real URL.
func TestInternalChipResolvesAcrossKinds(t *testing.T) {
	opts, tmp := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "lists", "l.md"), listBody)
	writeFileT(t, filepath.Join(opts.ContentDir, "essays", "e.md"), `---
title: "Essay"
slug: linking-essay
date: 2026-09-21
status: finished
---

See [the list](/lists/a-list){.margin}.
`)
	if err := Build(opts); err != nil {
		t.Fatal(err)
	}
	html := readOut(t, tmp, filepath.Join("essays", "linking-essay", "index.html"))
	if !strings.Contains(html, `href="/lists/a-list/"`) {
		t.Errorf("internal chip should link to the list's own URL:\n%s", html)
	}
}

// The breadcrumb names the document's own section.
func TestBreadcrumbFollowsContentKind(t *testing.T) {
	opts, tmp := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "lists", "l.md"), listBody)
	if err := Build(opts); err != nil {
		t.Fatal(err)
	}
	html := readOut(t, tmp, filepath.Join("lists", "a-list", "index.html"))
	if !strings.Contains(html, `<a href="/lists/">Lists</a>`) {
		t.Error("breadcrumb should name the Lists section")
	}
}

// Existing essays must keep their URLs exactly.
func TestEssayURLsUnchanged(t *testing.T) {
	opts, tmp := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "essays", "e.md"), `---
title: "Essay"
slug: still-here
date: 2026-09-21
status: finished
---

Body.
`)
	if err := Build(opts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "static", "essays", "still-here", "index.html")); err != nil {
		t.Fatalf("essay URLs must not change: %v", err)
	}
}

// ---- Cloudflare Pages output ---------------------------------------------

// Without a top-level 404.html, Pages treats the site as a single-page app and
// serves "/" for every unmatched path.
func TestBuildWrites404Page(t *testing.T) {
	opts, tmp := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "blog", "p.md"), textBody)
	if err := Build(opts); err != nil {
		t.Fatal(err)
	}
	html := readOut(t, tmp, "404.html")
	if !strings.Contains(html, "Not found") {
		t.Error("404.html should say the page was not found")
	}
	if !strings.Contains(html, `href="/"`) {
		t.Error("404.html should link back to the homepage")
	}
	// It must carry the real stylesheet, not a broken link.
	if !strings.Contains(html, "style.") {
		t.Error("404.html should reference the built stylesheet")
	}
}

func TestBuildWritesHeadersFile(t *testing.T) {
	opts, tmp := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "blog", "p.md"), textBody)
	if err := Build(opts); err != nil {
		t.Fatal(err)
	}
	h := readOut(t, tmp, "_headers")
	for _, want := range []string{
		"/style.*.css",
		"immutable",
		"/fonts/*",
		"max-age=60",
	} {
		if !strings.Contains(h, want) {
			t.Errorf("_headers missing %q", want)
		}
	}
}

// Markdown tables need the GFM table extension. Without it the pipes render as
// literal text, and the typographer rewrites the "---" separator row into an
// em-dash before anything can parse it as a table.
func TestMarkdownTableRenders(t *testing.T) {
	opts, tmp := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "lists", "l.md"), `---
title: "A List"
slug: a-list
date: 2026-09-21
status: finished
---

## Items

| Title | Author |
|---|---|
| Dune | Herbert |
| Neuromancer | Gibson |
`)
	if err := Build(opts); err != nil {
		t.Fatal(err)
	}
	html := readOut(t, tmp, filepath.Join("lists", "a-list", "index.html"))
	for _, want := range []string{
		"<table>", "<th>Title</th>",
		// Body cells carry their column's header as a data-label; the narrow
		// screen layout prints it in front of the value.
		`<td data-label="Title">Dune</td>`,
		`<td data-label="Author">Gibson</td>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("table output missing %q", want)
		}
	}
	if strings.Contains(html, "| Dune |") {
		t.Error("table rendered as literal pipes; the GFM table extension is not active")
	}
	if strings.Contains(html, "&mdash;|") {
		t.Error("the typographer mangled the table separator row")
	}
}

// A table is the one block whose minimum width can exceed the column it sits
// in, so it ships inside its own scrolling wrapper. Without it the table widens
// the grid track, the track widens the page, and a phone renders the whole
// article scaled down to whatever the table needed.
func TestMarkdownTableIsWrappedForScrolling(t *testing.T) {
	opts, tmp := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "lists", "l.md"), `---
title: "A List"
slug: a-list
date: 2026-09-21
status: finished
---

## Items

| Title | Author |
|---|---|
| Dune | Herbert |
`)
	if err := Build(opts); err != nil {
		t.Fatal(err)
	}
	html := readOut(t, tmp, filepath.Join("lists", "a-list", "index.html"))
	if !strings.Contains(html, `<div class="table-scroll" tabindex="0"><table>`) {
		t.Error("table is not wrapped in a .table-scroll container")
	}
	if !strings.Contains(html, "</table>\n</div>") {
		t.Error(".table-scroll wrapper is not closed around the table")
	}
	// The scrolling belongs to the wrapper. Putting it on the <table> needs
	// `display: block` there, which stops the table being a table box, so a
	// narrow table shrink-wraps its content instead of filling the column.
	// (The narrow-screen rules further down do set `display: block`, to stack
	// the rows as cards; cssRule reads the base rule, which is the one under
	// test here.)
	css := readAssetCSS(t)
	scroll := cssRule(t, css, ".content-cell .table-scroll")
	if declValue(scroll, "overflow-x") != "auto" {
		t.Errorf(".table-scroll must scroll horizontally; got %q", declValue(scroll, "overflow-x"))
	}
	table := cssRule(t, css, ".content-cell table")
	if d := declValue(table, "display"); d == "block" {
		t.Error("the <table> itself must not be display:block; the wrapper does the scrolling")
	}
}

// Every grid track holding article content is `minmax(0, 1fr)`, never a bare
// `1fr`. `1fr` means `minmax(auto, 1fr)`, and that `auto` minimum is the
// item's min-content size — so one wide child (a table) stretches the track,
// the grid and the page, and the reader is left zoomed out to fit it. The
// mobile overrides are the easy place to get this wrong, since that is where
// the two-column grid collapses to one.
func TestArticleGridTracksNeverUseBareFr(t *testing.T) {
	css := readAssetCSS(t)
	for _, decl := range []string{
		"grid-template-columns: 1fr;",
		"grid-template-columns:1fr;",
	} {
		if strings.Contains(css, decl) {
			t.Errorf("layout.css declares %q; use minmax(0, 1fr) so a wide table "+
				"cannot widen the track and with it the page", decl)
		}
	}
	// The cells themselves must also be allowed to be narrower than their
	// content, for the same reason.
	if declValue(cssRule(t, css, ".content-cell"), "min-width") != "0" {
		t.Error(".content-cell needs min-width: 0 or its min-content size widens the grid")
	}
}

// readAssetCSS returns layout.css and the table component concatenated, which is
// what the CSS assertions above read.
func readAssetCSS(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	for _, name := range []string{
		filepath.Join("..", "..", "assets", "css", "layout.css"),
		filepath.Join("..", "..", "assets", "css", "components", "table.css"),
	} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(data)
		b.WriteByte('\n')
	}
	return b.String()
}

// cssRule returns the body of the first rule with the given selector.
func cssRule(t *testing.T, css, sel string) string {
	t.Helper()
	i := strings.Index(css, sel+" {")
	if i < 0 {
		t.Fatalf("no %s rule found", sel)
	}
	body := css[i:]
	return body[:strings.Index(body, "}")]
}

// /books/ was a generated page before it became a list article; the redirect
// keeps existing links and bookmarks working.
func TestBuildWritesRedirectsFile(t *testing.T) {
	opts, tmp := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "blog", "p.md"), textBody)
	if err := Build(opts); err != nil {
		t.Fatal(err)
	}
	r := readOut(t, tmp, "_redirects")
	if !strings.Contains(r, "/books/ /lists/books/ 301") {
		t.Errorf("_redirects missing the /books/ redirect:\n%s", r)
	}
}

// ---- Post metadata box ----------------------------------------------------

// The metadata box sits in the header's right-hand rail column, level with the
// title, and outside .article-grid.
func TestPostMetaSitsInHeaderRail(t *testing.T) {
	opts, tmp := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "lists", "l.md"), listBody)
	if err := Build(opts); err != nil {
		t.Fatal(err)
	}
	html := readOut(t, tmp, filepath.Join("lists", "a-list", "index.html"))

	header := strings.Index(html, `<header class="article-header">`)
	heading := strings.Index(html, `<div class="article-heading">`)
	meta := strings.Index(html, `<div class="post-meta">`)
	grid := strings.Index(html, `<div class="article-grid">`)
	if header < 0 || heading < 0 || meta < 0 || grid < 0 {
		t.Fatal("page is missing the header, heading block, metadata box or grid")
	}
	// Ordered: header opens, title block, then the box, all before the grid.
	if !(header < heading && heading < meta && meta < grid) {
		t.Errorf("unexpected order: header=%d heading=%d meta=%d grid=%d",
			header, heading, meta, grid)
	}
}

// The box must stay out of .article-grid. Every top-level block is its own grid
// row and rows size to their tallest cell, so a box in the first row's margin
// cell stretches that row and opens a gap under a leading heading.
func TestPostMetaIsNotInsideTheArticleGrid(t *testing.T) {
	opts, tmp := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "lists", "l.md"), listBody)
	if err := Build(opts); err != nil {
		t.Fatal(err)
	}
	html := readOut(t, tmp, filepath.Join("lists", "a-list", "index.html"))

	if strings.Contains(html, `<div class="margin-cell"><div class="post-meta">`) {
		t.Error("metadata box leaked back into a margin cell")
	}
	if grid := strings.Index(html, `<div class="article-grid">`); grid >= 0 {
		if strings.Contains(html[grid:], `<div class="post-meta">`) {
			t.Error("metadata box must not render inside .article-grid")
		}
	}
}

// The header's columns must match .article-grid's, or the box will not line up
// with the marginalia rail beneath it.
func TestHeaderRailMatchesArticleGridColumns(t *testing.T) {
	css, err := os.ReadFile(filepath.Join("..", "..", "assets", "css", "layout.css"))
	if err != nil {
		t.Fatal(err)
	}
	rule := func(sel string) string {
		i := strings.Index(string(css), sel+" {")
		if i < 0 {
			t.Fatalf("no %s rule in layout.css", sel)
		}
		body := string(css)[i:]
		return body[:strings.Index(body, "}")]
	}
	header, grid := rule(".article-header"), rule(".article-grid")
	for _, prop := range []string{"grid-template-columns", "column-gap"} {
		h, g := declValue(header, prop), declValue(grid, prop)
		if h == "" || h != g {
			t.Errorf("%s: header has %q, article grid has %q — they must match",
				prop, h, g)
		}
	}
}

// declValue returns the value of a single CSS declaration inside a rule body.
func declValue(rule, prop string) string {
	i := strings.Index(rule, prop+":")
	if i < 0 {
		return ""
	}
	v := rule[i+len(prop)+1:]
	if end := strings.Index(v, ";"); end >= 0 {
		v = v[:end]
	}
	return strings.TrimSpace(v)
}

// Author, date, reading time and tags all live in the one box, so a post states
// its metadata once rather than in both a byline and a card.
func TestPostMetaCarriesAllMetadata(t *testing.T) {
	opts, tmp := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "lists", "l.md"), listBody)
	if err := Build(opts); err != nil {
		t.Fatal(err)
	}
	html := readOut(t, tmp, filepath.Join("lists", "a-list", "index.html"))

	for _, want := range []string{
		`<a href="/lists/">Lists</a>`, // breadcrumb
		"Gordy Clark",                 // author
		`<time datetime="2026-09-21">`,
		"min read",
		">lists</span>", // tag
	} {
		if !strings.Contains(html, want) {
			t.Errorf("metadata box missing %q", want)
		}
	}

	// Exactly one <time> element: the separate byline was folded into the box,
	// so the date is not printed twice.
	if n := strings.Count(html, "<time "); n != 1 {
		t.Errorf("page has %d <time> elements, want 1 (byline was folded into the box)", n)
	}
}

// ---- Tag colors -----------------------------------------------------------

// A tag's color is a function of its name, so it looks the same on every page
// and survives a rebuild. This is the property that makes the "random" palette
// assignment usable; a real random draw would repaint tags every build.
func TestTagColorIsStableAcrossPages(t *testing.T) {
	opts, tmp := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "lists", "l.md"), listBody)
	if err := Build(opts); err != nil {
		t.Fatal(err)
	}

	want := tagColorClass("lists")
	for _, page := range []string{
		filepath.Join("lists", "a-list", "index.html"), // margin card
		"index.html",                        // homepage
		filepath.Join("tags", "lists", "index.html"),
	} {
		html := readOut(t, tmp, page)
		if !strings.Contains(html, `class="tag `+want+`"`) {
			t.Errorf("%s: tag \"lists\" should carry %s", page, want)
		}
	}
}

// Every class the hash can produce must exist in tokens.css and tag.css, or a
// tag silently renders unstyled.
func TestEveryTagColorClassIsDefined(t *testing.T) {
	tag, err := os.ReadFile(filepath.Join("..", "..", "assets", "css", "components", "tag.css"))
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := os.ReadFile(filepath.Join("..", "..", "assets", "css", "tokens.css"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= tagPaletteSize; i++ {
		class := ".tag-c" + strconv.Itoa(i)
		if !strings.Contains(string(tag), class+" {") {
			t.Errorf("tag.css defines no %s", class)
		}
		token := "--tag-" + strconv.Itoa(i) + ":"
		if !strings.Contains(string(tokens), token) {
			t.Errorf("tokens.css defines no %s", token)
		}
	}
}

func TestTagColorIsDeterministic(t *testing.T) {
	for _, tag := range []string{"lists", "travel", "platform engineering", ""} {
		first := tagColorClass(tag)
		for i := 0; i < 100; i++ {
			if got := tagColorClass(tag); got != first {
				t.Fatalf("tagColorClass(%q) returned %q then %q", tag, first, got)
			}
		}
		if !strings.HasPrefix(first, "tag-c") {
			t.Errorf("tagColorClass(%q) = %q, want a tag-cN class", tag, first)
		}
	}
}
