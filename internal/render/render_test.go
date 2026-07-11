package render

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gordyclark/gordyclark.com/internal/content"
	"github.com/gordyclark/gordyclark.com/internal/diagrams"
)

// repoRoot returns the repository root derived from this test file's location
// (internal/render/), so tests can point at the REAL templates/ and assets/.
func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// wd is <root>/internal/render
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

func writeFileT(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// scaffold creates a temp content tree and returns Options wired to the real
// templates and assets dirs.
func scaffold(t *testing.T) (Options, string) {
	t.Helper()
	root := repoRoot(t)
	tmp := t.TempDir()
	opts := Options{
		ContentDir:   filepath.Join(tmp, "content"),
		OutDir:       filepath.Join(tmp, "static"),
		CacheDir:     filepath.Join(tmp, ".cache"),
		AssetsDir:    filepath.Join(root, "assets"),
		TemplatesDir: filepath.Join(root, "templates"),
	}
	if err := os.MkdirAll(filepath.Join(opts.ContentDir, "essays"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Empty citations by default.
	writeFileT(t, filepath.Join(opts.ContentDir, "citations.yaml"), "")
	return opts, tmp
}

// ---- css.go: deterministic hash ------------------------------------------

func TestBuildCSSDeterministic(t *testing.T) {
	root := repoRoot(t)
	assets := filepath.Join(root, "assets")

	out1 := t.TempDir()
	out2 := t.TempDir()
	name1, err := buildCSS(assets, out1)
	if err != nil {
		t.Fatal(err)
	}
	name2, err := buildCSS(assets, out2)
	if err != nil {
		t.Fatal(err)
	}
	if name1 != name2 {
		t.Fatalf("non-deterministic css filename: %q vs %q", name1, name2)
	}
	if !strings.HasPrefix(name1, "style.") || !strings.HasSuffix(name1, ".css") {
		t.Fatalf("unexpected css filename %q", name1)
	}
	if _, err := os.Stat(filepath.Join(out1, name1)); err != nil {
		t.Fatalf("css bundle not written: %v", err)
	}
}

func TestBuildCSSHashChangesWithContent(t *testing.T) {
	// Build a minimal fake assets tree so we can mutate a css file.
	tmp := t.TempDir()
	cssDir := filepath.Join(tmp, "css")
	writeFileT(t, filepath.Join(cssDir, "manifest.txt"), "a.css\nb.css\n")
	writeFileT(t, filepath.Join(cssDir, "a.css"), "body{color:red}\n")
	writeFileT(t, filepath.Join(cssDir, "b.css"), ".x{margin:0}\n")

	out := t.TempDir()
	before, err := buildCSS(tmp, out)
	if err != nil {
		t.Fatal(err)
	}
	// Change a css file.
	writeFileT(t, filepath.Join(cssDir, "a.css"), "body{color:blue}\n")
	after, err := buildCSS(tmp, out)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatalf("hash did not change after editing a css file: %q", before)
	}
}

// ---- reading time --------------------------------------------------------

func TestReadingTime(t *testing.T) {
	i5 := 5
	cases := []struct {
		name     string
		body     string
		override *int
		want     int
	}{
		{"override wins", strings.Repeat("word ", 1000), &i5, 5},
		{"short floors to 1", "just a few words here", nil, 1},
		{"computed nearest", strings.Repeat("word ", 460), nil, 2},
		{"empty floors to 1", "", nil, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := &content.Essay{
				Body:  []byte(tc.body),
				Front: content.Frontmatter{ReadingTimeOverride: tc.override},
			}
			if got := readingTime(e); got != tc.want {
				t.Fatalf("readingTime = %d, want %d", got, tc.want)
			}
		})
	}
}

// ---- related essays ------------------------------------------------------

func TestRelatedEssays(t *testing.T) {
	ix := &content.Index{BySlug: map[string]*content.IndexEntry{}}
	add := func(slug string, tags []string, status content.Status) {
		e := &content.IndexEntry{Slug: slug, Title: "T-" + slug, Subtitle: "S-" + slug, Tags: tags, Status: status}
		ix.BySlug[slug] = e
		ix.Ordered = append(ix.Ordered, e) // insertion order = newest-first for test
	}
	add("self", []string{"go", "web"}, content.StatusFinished)
	add("a", []string{"go"}, content.StatusFinished)
	add("b", []string{"web"}, content.StatusFinished)
	add("c", []string{"go", "web"}, content.StatusFinished)
	add("draft", []string{"go"}, content.StatusDraft)
	add("d", []string{"go"}, content.StatusFinished)
	add("unrelated", []string{"cooking"}, content.StatusFinished)

	self := &content.Essay{Front: content.Frontmatter{Slug: "self", Tags: []string{"go", "web"}}}
	rel := relatedEssays(self, ix)

	if len(rel) != 3 {
		t.Fatalf("expected cap of 3 related, got %d: %+v", len(rel), rel)
	}
	for _, r := range rel {
		if r.Slug == "self" {
			t.Fatalf("related must exclude self")
		}
		if r.Slug == "draft" {
			t.Fatalf("related must exclude drafts")
		}
		if r.Slug == "unrelated" {
			t.Fatalf("related must share a tag")
		}
	}
	// Order should follow ix.Ordered: a, b, c (cap hits before d).
	want := []string{"a", "b", "c"}
	for i, r := range rel {
		if r.Slug != want[i] {
			t.Fatalf("related[%d] = %q, want %q (order %+v)", i, r.Slug, want[i], rel)
		}
	}
}

// ---- §2.3 unhydrated external margin link --------------------------------

func TestUnhydratedExternalLinkError(t *testing.T) {
	opts, _ := scaffold(t)
	essay := `---
title: Test
slug: test
date: 2026-01-01
---

See [x](https://y.com){.margin} for more.
`
	writeFileT(t, filepath.Join(opts.ContentDir, "essays", "test.md"), essay)

	err := Build(opts)
	if err == nil {
		t.Fatal("expected error for unhydrated external margin link")
	}
	msg := err.Error()
	for _, want := range []string{
		"ERROR: unhydrated margin link in",
		"[x](https://y.com){.margin}",
		"run: cmd/hydrate",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error message missing %q; got:\n%s", want, msg)
		}
	}
}

// ---- unknown citation key ------------------------------------------------

func TestUnknownCitationKey(t *testing.T) {
	opts, _ := scaffold(t)
	essay := `---
title: Test
slug: test
date: 2026-01-01
---

A claim.[^cite:missing]

[^cite:missing]: ignored body
`
	writeFileT(t, filepath.Join(opts.ContentDir, "essays", "test.md"), essay)

	err := Build(opts)
	if err == nil {
		t.Fatal("expected error for unknown citation key")
	}
	if !strings.Contains(err.Error(), `unknown citation key "missing"`) {
		t.Fatalf("error missing key name; got: %s", err.Error())
	}
}

// ---- internal chip resolution --------------------------------------------

func TestInternalChipResolves(t *testing.T) {
	opts, tmp := scaffold(t)
	other := `---
title: Other Essay
subtitle: The other one
slug: other
date: 2026-01-01
tags: [go]
---

Body of other.
`
	main := `---
title: Main Essay
slug: main
date: 2026-02-01
tags: [go]
---

Read [the other](/essays/other){.margin} essay.
`
	writeFileT(t, filepath.Join(opts.ContentDir, "essays", "other.md"), other)
	writeFileT(t, filepath.Join(opts.ContentDir, "essays", "main.md"), main)

	if err := Build(opts); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	html := readOut(t, tmp, "essays/main/index.html")
	if !strings.Contains(html, `class="margin-chip internal"`) {
		t.Fatalf("internal chip markup missing:\n%s", html)
	}
	if !strings.Contains(html, `href="/essays/other/"`) {
		t.Fatalf("internal chip href missing")
	}
	if !strings.Contains(html, "Other Essay") {
		t.Fatalf("internal chip should carry target title")
	}
}

func TestInternalChipUnknownSlug(t *testing.T) {
	opts, _ := scaffold(t)
	main := `---
title: Main
slug: main
date: 2026-02-01
---

Read [nope](/essays/ghost){.margin}.
`
	writeFileT(t, filepath.Join(opts.ContentDir, "essays", "main.md"), main)
	err := Build(opts)
	if err == nil || !strings.Contains(err.Error(), "unknown essay slug") {
		t.Fatalf("expected unknown-slug error, got: %v", err)
	}
}

// ---- content/margin cell pairing ----------------------------------------

func TestCellPairing(t *testing.T) {
	opts, tmp := scaffold(t)
	essay := `---
title: Multi Block
slug: multi
date: 2026-01-01
---

First paragraph.

## A heading

Second paragraph with a note.[^1]

- a list
- of items

> a blockquote

[^1]: the note body
`
	writeFileT(t, filepath.Join(opts.ContentDir, "essays", "multi.md"), essay)
	if err := Build(opts); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	html := readOut(t, tmp, "essays/multi/index.html")
	nc := strings.Count(html, `class="content-cell"`)
	nm := strings.Count(html, `class="margin-cell"`)
	if nc == 0 {
		t.Fatal("no content cells emitted")
	}
	if nc != nm {
		t.Fatalf("content-cell count %d != margin-cell count %d", nc, nm)
	}
	// The note body must appear in the margin, not as a trailing footnote list.
	if !strings.Contains(html, `class="margin-note"`) {
		t.Fatalf("footnote body not relocated to margin")
	}
	if strings.Contains(html, `class="footnotes"`) {
		t.Fatalf("goldmark footnote list leaked into output")
	}
}

// ---- citation card content ----------------------------------------------

func TestCitationCard(t *testing.T) {
	opts, tmp := scaffold(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "citations.yaml"), `mykey:
  author: Jane Doe
  title: A Great Paper
  source: Journal of Things
  year: "2020"
  url: https://example.com/paper
`)
	essay := `---
title: Cited
slug: cited
date: 2026-01-01
---

A claim.[^cite:mykey]

[^cite:mykey]: unused
`
	writeFileT(t, filepath.Join(opts.ContentDir, "essays", "cited.md"), essay)
	if err := Build(opts); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	html := readOut(t, tmp, "essays/cited/index.html")
	if !strings.Contains(html, `class="margin-citation"`) {
		t.Fatalf("citation card markup missing:\n%s", html)
	}
	if !strings.Contains(html, "A Great Paper") || !strings.Contains(html, "Jane Doe") {
		t.Fatalf("citation card missing resolved fields")
	}
}

// ---- external chip (hydrated) --------------------------------------------

func TestExternalChipHydrated(t *testing.T) {
	opts, tmp := scaffold(t)
	essay := `---
title: Ext
slug: ext
date: 2026-01-01
---

See [the source](https://example.com){.margin domain="example.com" title="Example" desc="An example site"}.
`
	writeFileT(t, filepath.Join(opts.ContentDir, "essays", "ext.md"), essay)
	if err := Build(opts); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	html := readOut(t, tmp, "essays/ext/index.html")
	if !strings.Contains(html, `class="margin-chip"`) {
		t.Fatalf("external chip markup missing:\n%s", html)
	}
	if !strings.Contains(html, "example.com") || !strings.Contains(html, "An example site") {
		t.Fatalf("external chip missing hydrated fields")
	}
}

// ---- d2 diagram (cache-seeded, no d2 binary needed) ----------------------

func TestD2DiagramFromCache(t *testing.T) {
	opts, tmp := scaffold(t)
	d2src := "a -> b\n"
	// Pre-seed the diagram cache at the exact sha256 path diagrams.Render uses.
	sum := sha256.Sum256([]byte(d2src))
	hash := hex.EncodeToString(sum[:])
	svg := `<svg id="seeded"><rect/></svg>`
	writeFileT(t, filepath.Join(opts.CacheDir, "diagrams", hash+".svg"), svg)

	essay := "---\ntitle: Diag\nslug: diag\ndate: 2026-01-01\n---\n\n```d2\n" + d2src + "```\n"
	writeFileT(t, filepath.Join(opts.ContentDir, "essays", "diag.md"), essay)

	if err := Build(opts); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	html := readOut(t, tmp, "essays/diag/index.html")
	if !strings.Contains(html, "<figure>"+svg+"</figure>") {
		t.Fatalf("seeded SVG not inlined inside <figure>:\n%s", html)
	}
}

// verify the cache-hit path directly against the diagrams package too.
func TestD2CacheContract(t *testing.T) {
	tmp := t.TempDir()
	dr := diagrams.New(tmp)
	src := "x -> y\n"
	sum := sha256.Sum256([]byte(src))
	hash := hex.EncodeToString(sum[:])
	svg := "<svg>hit</svg>"
	writeFileT(t, filepath.Join(tmp, "diagrams", hash+".svg"), svg)
	got, err := dr.Render(src)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if got != svg {
		t.Fatalf("cache hit returned %q, want %q", got, svg)
	}
}

// ---- full happy-path build produces index + tag pages --------------------

func TestBuildIndexAndTagPages(t *testing.T) {
	opts, tmp := scaffold(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "essays", "one.md"), `---
title: One
slug: one
date: 2026-03-01
tags: [go, web]
---
Body one.
`)
	writeFileT(t, filepath.Join(opts.ContentDir, "essays", "two.md"), `---
title: Two
slug: two
date: 2026-01-01
status: draft
tags: [go]
---
Body two.
`)
	if err := Build(opts); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	idx := readOut(t, tmp, "index.html")
	if !strings.Contains(idx, "One") {
		t.Fatalf("index missing finished essay")
	}
	if strings.Contains(idx, `href="/essays/two/"`) {
		t.Fatalf("index should exclude drafts")
	}
	// Draft still rendered to its own page.
	if _, err := os.Stat(filepath.Join(tmp, "static", "essays", "two", "index.html")); err != nil {
		t.Fatalf("draft essay page not rendered: %v", err)
	}
	// Tag page for a finished essay's tag exists; draft-only tags do not leak.
	if _, err := os.Stat(filepath.Join(tmp, "static", "tags", "web", "index.html")); err != nil {
		t.Fatalf("tag page /tags/web not written: %v", err)
	}
	tagHTML := readOut(t, tmp, "tags/web/index.html")
	if !strings.Contains(tagHTML, "Tagged: web") {
		t.Fatalf("tag page heading wrong:\n%s", tagHTML)
	}
	// Fonts copied.
	if _, err := os.Stat(filepath.Join(tmp, "static", "fonts", "public-sans-latin-wght-normal.woff2")); err != nil {
		t.Fatalf("font not copied: %v", err)
	}
}

func readOut(t *testing.T, tmp, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(tmp, "static", rel))
	if err != nil {
		t.Fatalf("reading output %q: %v", rel, err)
	}
	return string(data)
}
