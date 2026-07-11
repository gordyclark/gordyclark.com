package main

import (
	"errors"
	"strings"
	"testing"
)

func TestExtractMeta(t *testing.T) {
	tests := []struct {
		name, html, wantTitle, wantDesc string
	}{
		{
			name:      "og description",
			html:      `<html><head><title>Hello</title><meta property="og:description" content="OG desc"><meta name="description" content="Name desc"></head></html>`,
			wantTitle: "Hello",
			wantDesc:  "OG desc",
		},
		{
			name:      "only name description",
			html:      `<html><head><title>Title2</title><meta name="description" content="Name only"></head></html>`,
			wantTitle: "Title2",
			wantDesc:  "Name only",
		},
		{
			name:      "neither",
			html:      `<html><head><title>Just Title</title></head></html>`,
			wantTitle: "Just Title",
			wantDesc:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, desc, err := extractMeta(strings.NewReader(tt.html))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if title != tt.wantTitle {
				t.Errorf("title = %q, want %q", title, tt.wantTitle)
			}
			if desc != tt.wantDesc {
				t.Errorf("desc = %q, want %q", desc, tt.wantDesc)
			}
		})
	}
}

func TestSanitize(t *testing.T) {
	tests := []struct{ in, want string }{
		{"  hello   world  ", "hello world"},
		{"line1\nline2\t\tline3", "line1 line2 line3"},
		{`say "hi" there`, "say 'hi' there"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := sanitize(tt.in); got != tt.want {
			t.Errorf("sanitize(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestHasMarginClass(t *testing.T) {
	if !hasMarginClass("{.margin domain=\"x\"}") {
		t.Error("expected margin detected")
	}
	if hasMarginClass("{.marginal}") {
		t.Error("marginal is not margin")
	}
	if hasMarginClass("{.foo}") {
		t.Error("foo is not margin")
	}
	if !hasMarginClass("{.foo .margin}") {
		t.Error("margin among others should be detected")
	}
}

func TestParsedAttrs(t *testing.T) {
	got := parsedAttrs(`{.margin domain="x.com" title="A B" desc="c"}`)
	if got["domain"] != "x.com" || got["title"] != "A B" || got["desc"] != "c" {
		t.Errorf("parsedAttrs = %v", got)
	}
}

func TestBuildBlockPreservesExtras(t *testing.T) {
	block := buildBlock(map[string]string{"data-x": "y"}, []string{"foo"}, "x.com", "T", "D")
	if !strings.Contains(block, ".margin") {
		t.Errorf("missing .margin: %s", block)
	}
	if !strings.Contains(block, ".foo") {
		t.Errorf("missing .foo: %s", block)
	}
	if !strings.Contains(block, `data-x="y"`) {
		t.Errorf("missing extra key: %s", block)
	}
	if !strings.Contains(block, `domain="x.com"`) || !strings.Contains(block, `title="T"`) || !strings.Contains(block, `desc="D"`) {
		t.Errorf("missing trio: %s", block)
	}
}

func fakeFetch(title, desc string) fetchFunc {
	return func(string) (string, string, error) { return title, desc, nil }
}

func TestProcessSourceHydrateAndIdempotent(t *testing.T) {
	src := []byte("Text [t](https://example.com){.margin} more.")
	fetch := fakeFetch("My Title", "My Desc")

	out, hydrated, skipped, failed, _ := processSource(src, fetch)
	if hydrated != 1 || skipped != 0 || failed != 0 {
		t.Fatalf("counts: hydrated=%d skipped=%d failed=%d", hydrated, skipped, failed)
	}
	s := string(out)
	if !strings.Contains(s, `domain="example.com"`) || !strings.Contains(s, `title="My Title"`) || !strings.Contains(s, `desc="My Desc"`) {
		t.Fatalf("hydrated output missing trio: %s", s)
	}

	// Idempotency: running again skips and does not change bytes.
	out2, h2, sk2, f2, _ := processSource(out, fetch)
	if h2 != 0 || sk2 != 1 || f2 != 0 {
		t.Fatalf("second pass counts: hydrated=%d skipped=%d failed=%d", h2, sk2, f2)
	}
	if string(out2) != string(out) {
		t.Fatalf("not idempotent:\n%s\n---\n%s", out, out2)
	}
}

func TestProcessSourceTwoLinksReverseSplice(t *testing.T) {
	src := []byte("[a](https://a.com){.margin} and [b](https://b.org){.margin}")
	fetch := func(u string) (string, string, error) {
		if strings.Contains(u, "a.com") {
			return "TitleA", "DescA", nil
		}
		return "TitleB", "DescB", nil
	}
	out, hydrated, _, _, _ := processSource(src, fetch)
	if hydrated != 2 {
		t.Fatalf("hydrated=%d, want 2", hydrated)
	}
	s := string(out)
	if !strings.Contains(s, `domain="a.com"`) || !strings.Contains(s, `title="TitleA"`) {
		t.Errorf("link A wrong: %s", s)
	}
	if !strings.Contains(s, `domain="b.org"`) || !strings.Contains(s, `title="TitleB"`) {
		t.Errorf("link B wrong: %s", s)
	}
}

func TestProcessSourceBareLinkUntouched(t *testing.T) {
	src := []byte("An ordinary [link](https://plain.com) here.")
	out, hydrated, skipped, failed, _ := processSource(src, fakeFetch("X", "Y"))
	if hydrated != 0 || skipped != 0 || failed != 0 {
		t.Fatalf("counts: hydrated=%d skipped=%d failed=%d", hydrated, skipped, failed)
	}
	if string(out) != string(src) {
		t.Fatalf("bare link modified: %s", out)
	}
}

func TestProcessSourceAlreadyHydratedSkipped(t *testing.T) {
	src := []byte(`[t](https://ex.com){.margin domain="ex.com" title="T" desc="D"}`)
	out, hydrated, skipped, failed, _ := processSource(src, fakeFetch("NEW", "NEW"))
	if hydrated != 0 || skipped != 1 || failed != 0 {
		t.Fatalf("counts: hydrated=%d skipped=%d failed=%d", hydrated, skipped, failed)
	}
	if string(out) != string(src) {
		t.Fatalf("hydrated link changed: %s", out)
	}
}

func TestProcessSourceFetchFailure(t *testing.T) {
	src := []byte("[t](https://ex.com){.margin}")
	fetch := func(string) (string, string, error) { return "", "", errors.New("boom") }
	out, hydrated, skipped, failed, failures := processSource(src, fetch)
	if hydrated != 0 || skipped != 0 || failed != 1 {
		t.Fatalf("counts: hydrated=%d skipped=%d failed=%d", hydrated, skipped, failed)
	}
	if string(out) != string(src) {
		t.Fatalf("failed link modified: %s", out)
	}
	if len(failures) != 1 || !strings.Contains(failures[0], "boom") {
		t.Fatalf("failures = %v", failures)
	}
}

func TestProcessSourceExtraClassPreserved(t *testing.T) {
	src := []byte("[t](https://ex.com){.margin .foo}")
	out, _, _, _, _ := processSource(src, fakeFetch("T", "D"))
	s := string(out)
	if !strings.Contains(s, ".margin") || !strings.Contains(s, ".foo") {
		t.Fatalf("classes not preserved: %s", s)
	}
}
