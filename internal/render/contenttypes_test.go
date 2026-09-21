package render

import (
	"os"
	"path/filepath"
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
