package highlight

import (
	"strings"
	"testing"
)

func TestHighlightGoProducesClassedSpans(t *testing.T) {
	src := "package main\n\nfunc main() {\n\tvar x int = 42\n}\n"
	out, err := Highlight(src, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "<pre") {
		t.Errorf("expected a <pre element, got: %q", s)
	}
	if !strings.Contains(s, `<span class="`) {
		t.Errorf("expected at least one classed span, got: %q", s)
	}
}

func TestHighlightUnknownLangFallsBack(t *testing.T) {
	src := "some arbitrary text"
	out, err := Highlight(src, "zzznotalang")
	if err != nil {
		t.Fatalf("expected nil error for unknown lang, got: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "<pre") || !strings.Contains(s, "<code>") {
		t.Errorf("expected plain <pre><code> fallback, got: %q", s)
	}
	if strings.Contains(s, `<span class="`) {
		t.Errorf("fallback should contain no chroma spans, got: %q", s)
	}
}

func TestHighlightFallbackEscapesHTML(t *testing.T) {
	src := `<script>alert("x & y")</script>`
	out, err := Highlight(src, "zzznotalang")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "<script>") {
		t.Errorf("raw <script> must be escaped, got: %q", s)
	}
	for _, want := range []string{"&lt;script&gt;", "&amp;", "&#34;"} {
		if !strings.Contains(s, want) {
			t.Errorf("expected escaped %q in output, got: %q", want, s)
		}
	}
}
