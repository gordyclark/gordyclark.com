package listtoc

import (
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"First Thing":            "first-thing",
		"  Padded  ":             "padded",
		"Punctuation! Here?":     "punctuation-here",
		"Multiple   Spaces":      "multiple-spaces",
		"Hyphen-Already":         "hyphen-already",
		"Numbers 123 Kept":       "numbers-123-kept",
		"MiXeD CaSe":             "mixed-case",
		"---":                    "",
		"Trailing punctuation!!": "trailing-punctuation",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAssignNumbersInOrder(t *testing.T) {
	items := Assign([]string{"Alpha", "Beta", "Gamma"})
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	for i, want := range []string{"alpha", "beta", "gamma"} {
		if items[i].Number != i+1 {
			t.Errorf("item %d Number = %d, want %d", i, items[i].Number, i+1)
		}
		if items[i].ID != want {
			t.Errorf("item %d ID = %q, want %q", i, items[i].ID, want)
		}
	}
}

// Duplicate headings must not produce duplicate ids, or the anchors become
// ambiguous and a TOC link jumps to the wrong item.
func TestAssignDisambiguatesDuplicates(t *testing.T) {
	items := Assign([]string{"Same", "Same", "Same"})
	seen := map[string]bool{}
	for _, it := range items {
		if seen[it.ID] {
			t.Fatalf("duplicate anchor id %q in %+v", it.ID, items)
		}
		seen[it.ID] = true
	}
}

func TestAssignHandlesUnslugifiableHeading(t *testing.T) {
	items := Assign([]string{"!!!", "???"})
	for _, it := range items {
		if it.ID == "" {
			t.Fatalf("empty anchor id in %+v", items)
		}
	}
	if items[0].ID == items[1].ID {
		t.Errorf("ids should be unique, both were %q", items[0].ID)
	}
}

func TestRenderEmpty(t *testing.T) {
	if got := Render(nil); got != "" {
		t.Errorf("Render(nil) = %q, want empty", got)
	}
}

func TestRenderLinksToAnchors(t *testing.T) {
	html := string(Render(Assign([]string{"First Thing", "Second Thing"})))
	for _, want := range []string{
		`class="list-toc"`,
		`<a href="#first-thing">First Thing</a>`,
		`<a href="#second-thing">Second Thing</a>`,
		`<ol class="list-toc-items">`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("TOC missing %q:\n%s", want, html)
		}
	}
}

// The TOC must stay plain anchors: no JS hooks and no scroll manipulation.
func TestRenderUsesPlainAnchorsOnly(t *testing.T) {
	html := strings.ToLower(string(Render(Assign([]string{"One", "Two"}))))
	for _, forbidden := range []string{"onclick", "<script", "scrollto", "javascript:"} {
		if strings.Contains(html, forbidden) {
			t.Errorf("TOC should contain no %q:\n%s", forbidden, html)
		}
	}
}

func TestRenderEscapesTitles(t *testing.T) {
	html := string(Render(Assign([]string{`<script>alert(1)</script>`})))
	if strings.Contains(html, "<script>") {
		t.Errorf("heading text was not escaped:\n%s", html)
	}
}
