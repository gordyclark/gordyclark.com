package margin

import (
	"bytes"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	gmtext "github.com/yuin/goldmark/text"
)

func TestInlineAttrParserAttaches(t *testing.T) {
	src := []byte(`[D2](https://d2lang.com){.margin domain="d2lang.com" title="T" desc="D"}`)
	md := goldmark.New(goldmark.WithExtensions(AttributeExtension(), extension.Footnote))
	reader := gmtext.NewReader(src)
	doc := md.Parser().Parse(reader)

	var link *ast.Link
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if l, ok := n.(*ast.Link); ok {
				link = l
			}
		}
		return ast.WalkContinue, nil
	})
	if link == nil {
		t.Fatal("no link node found")
	}
	classes, kv, ok := LinkAttrs(link)
	if !ok {
		t.Fatal("LinkAttrs returned ok=false; attributes not attached")
	}
	if !hasClass(classes, "margin") {
		t.Errorf("expected 'margin' class, got %v", classes)
	}
	for k, want := range map[string]string{"domain": "d2lang.com", "title": "T", "desc": "D"} {
		if kv[k] != want {
			t.Errorf("kv[%q]=%q, want %q", k, kv[k], want)
		}
	}
}

func TestRenderedOutputHasNoBraces(t *testing.T) {
	md := goldmark.New(goldmark.WithExtensions(AttributeExtension(), extension.Footnote))
	src := []byte(`[D2](https://x.com){.margin domain="d" title="t" desc="e"}`)
	var buf bytes.Buffer
	if err := md.Convert(src, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `href="https://x.com"`) {
		t.Errorf("expected href in output, got: %s", out)
	}
	if strings.Contains(out, "{") || strings.Contains(out, "}") {
		t.Errorf("raw braces leaked into output: %s", out)
	}
	if strings.Contains(out, "domain=") {
		t.Errorf("attribute text leaked into output: %s", out)
	}
}

func TestBracesNotFollowingLinkAreLiteral(t *testing.T) {
	md := goldmark.New(goldmark.WithExtensions(AttributeExtension()))
	var buf bytes.Buffer
	if err := md.Convert([]byte(`plain {.foo} text`), &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "{.foo}") {
		t.Errorf("expected literal braces preserved, got: %s", buf.String())
	}
}

func TestIsExternal(t *testing.T) {
	cases := map[string]bool{
		"https://d2lang.com": true,
		"http://x.com":       true,
		"/essays/foo.md":     false,
		"./foo.md":           false,
		"foo":                false,
		"foo/bar":            false,
		"mailto:x@y.com":     false,
	}
	for in, want := range cases {
		if got := IsExternal(in); got != want {
			t.Errorf("IsExternal(%q)=%v, want %v", in, got, want)
		}
	}
}

func TestSlugFromInternalHref(t *testing.T) {
	cases := map[string]string{
		"/essays/foo.md":  "foo",
		"/essays/foo":     "foo",
		"./foo.md":        "foo",
		"foo":             "foo",
		"foo/":            "foo",
		"/essays/foo/":    "foo",
		"/essays/foo#sec": "foo",
		"bar.md":          "bar",
	}
	for in, want := range cases {
		if got := SlugFromInternalHref(in); got != want {
			t.Errorf("SlugFromInternalHref(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestHostname(t *testing.T) {
	cases := map[string]string{
		"https://example.com/a/b":      "example.com",
		"https://www.example.com/a":    "example.com",
		"http://en.wikipedia.org/wiki": "en.wikipedia.org",
		"https://example.com":          "example.com",
		// A URL with no host yields empty rather than an error the caller
		// would have to decide what to do with.
		"not a url": "",
	}
	for in, want := range cases {
		if got := Hostname(in); got != want {
			t.Errorf("Hostname(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestChipDomainFallsBackToHref(t *testing.T) {
	// An authored domain always wins.
	if got := ChipDomain("d.example", "https://other.com/x"); got != "d.example" {
		t.Errorf("authored domain should win, got %q", got)
	}
	// Absent, it is derived from the href, so a chip that was never hydrated
	// still shows where it points.
	if got := ChipDomain("", "https://en.wikipedia.org/wiki/X"); got != "en.wikipedia.org" {
		t.Errorf("derived domain=%q, want en.wikipedia.org", got)
	}
}

func TestChipTitleFallsBackToPathThenDomain(t *testing.T) {
	// An authored title always wins.
	if got := ChipTitle("T", "https://e.com/some/page"); got != "T" {
		t.Errorf("authored title should win, got %q", got)
	}
	// Absent, the last path segment reads better than a bare domain.
	if got := ChipTitle("", "https://en.wikipedia.org/wiki/Hamad_International_Airport"); got != "Hamad International Airport" {
		t.Errorf("path-derived title=%q", got)
	}
	// A fragment names the section, which is more specific than the page.
	if got := ChipTitle("", "https://e.com/wiki/Page#Slave_labor"); got != "Slave labor" {
		t.Errorf("fragment-derived title=%q", got)
	}
	// With no path to work from, the domain is the only thing left.
	if got := ChipTitle("", "https://example.com/"); got != "example.com" {
		t.Errorf("domain fallback title=%q", got)
	}
}

func TestDomainInitial(t *testing.T) {
	cases := map[string]string{
		"d2lang.com": "D",
		"example":    "E",
		"":           "?",
		"  ":         "?",
	}
	for in, want := range cases {
		if got := DomainInitial(in); got != want {
			t.Errorf("DomainInitial(%q)=%q, want %q", in, got, want)
		}
	}
}
