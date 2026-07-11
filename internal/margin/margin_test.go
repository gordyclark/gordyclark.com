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

func TestValidateExternalChip(t *testing.T) {
	// Non-margin link: always fine.
	if err := ValidateExternalChip([]string{"foo"}, nil); err != nil {
		t.Errorf("non-margin should pass: %v", err)
	}
	// Complete margin chip: fine.
	good := map[string]string{"domain": "d", "title": "t", "desc": "e"}
	if err := ValidateExternalChip([]string{"margin"}, good); err != nil {
		t.Errorf("complete margin chip should pass: %v", err)
	}
	// Missing desc: error.
	bad := map[string]string{"domain": "d", "title": "t"}
	err := ValidateExternalChip([]string{"margin"}, bad)
	if err == nil {
		t.Fatal("expected error for missing desc")
	}
	if !strings.Contains(err.Error(), "desc") {
		t.Errorf("error should mention desc, got: %v", err)
	}
	// Empty value counts as missing.
	empty := map[string]string{"domain": "", "title": "t", "desc": "e"}
	if err := ValidateExternalChip([]string{"margin"}, empty); err == nil {
		t.Error("expected error for empty domain")
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
