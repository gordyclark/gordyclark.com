package postimage

import (
	"strings"
	"testing"
)

func TestParseAllAttributes(t *testing.T) {
	img, err := Parse(`src="/img/photo.jpg" align="left" alt="A photo" caption="A caption"`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if img.Src != "/img/photo.jpg" {
		t.Errorf("Src = %q, want /img/photo.jpg", img.Src)
	}
	if img.Align != AlignLeft {
		t.Errorf("Align = %q, want left", img.Align)
	}
	if img.Alt != "A photo" {
		t.Errorf("Alt = %q, want %q", img.Alt, "A photo")
	}
	if img.Caption != "A caption" {
		t.Errorf("Caption = %q, want %q", img.Caption, "A caption")
	}
}

// Authors write these blocks across several lines; the attribute grammar is
// single-line, so the body must be collapsed before parsing.
func TestParseMultiLineBody(t *testing.T) {
	img, err := Parse("src=\"/img/a.png\"\nalign=\"right\"\nalt=\"Alt text\"\n")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if img.Align != AlignRight {
		t.Errorf("Align = %q, want right", img.Align)
	}
	if img.Alt != "Alt text" {
		t.Errorf("Alt = %q, want %q", img.Alt, "Alt text")
	}
}

func TestParseDefaultsToCenter(t *testing.T) {
	img, err := Parse(`src="/img/a.png" alt="Alt"`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if img.Align != AlignCenter {
		t.Errorf("Align = %q, want center by default", img.Align)
	}
}

func TestParseEachAlignment(t *testing.T) {
	for _, want := range []Align{AlignLeft, AlignRight, AlignCenter} {
		img, err := Parse(`src="/img/a.png" alt="Alt" align="` + string(want) + `"`)
		if err != nil {
			t.Fatalf("Parse(%s): %v", want, err)
		}
		if img.Align != want {
			t.Errorf("Align = %q, want %q", img.Align, want)
		}
	}
}

func TestParseRejectsMissingSrc(t *testing.T) {
	if _, err := Parse(`alt="Alt"`); err == nil {
		t.Fatal("expected an error when src is missing")
	}
}

// An image with no alt text must fail the build rather than ship unlabelled.
func TestParseRejectsMissingAlt(t *testing.T) {
	_, err := Parse(`src="/img/a.png"`)
	if err == nil {
		t.Fatal("expected an error when alt is missing")
	}
	if !strings.Contains(err.Error(), "alt") {
		t.Errorf("error should name the missing attribute, got: %v", err)
	}
}

func TestParseRejectsBadAlign(t *testing.T) {
	_, err := Parse(`src="/img/a.png" alt="Alt" align="middle"`)
	if err == nil {
		t.Fatal("expected an error for an unrecognised align value")
	}
	if !strings.Contains(err.Error(), "middle") {
		t.Errorf("error should quote the bad value, got: %v", err)
	}
}

func TestRenderIncludesAlignmentClassAndAlt(t *testing.T) {
	html := string(Render(Image{Src: "/img/a.png", Alt: "Alt", Align: AlignLeft}))
	for _, want := range []string{`class="post-img align-left"`, `src="/img/a.png"`, `alt="Alt"`, `loading="lazy"`} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered HTML missing %q:\n%s", want, html)
		}
	}
	if strings.Contains(html, "<figcaption>") {
		t.Errorf("no caption was set, so none should render:\n%s", html)
	}
}

func TestRenderCaption(t *testing.T) {
	html := string(Render(Image{Src: "/img/a.png", Alt: "Alt", Caption: "Hello", Align: AlignCenter}))
	if !strings.Contains(html, "<figcaption>Hello</figcaption>") {
		t.Errorf("caption missing:\n%s", html)
	}
}

// Attribute values are author-supplied and land in HTML attributes, so quotes
// and angle brackets must not be able to break out of the markup.
func TestRenderEscapesAttributes(t *testing.T) {
	html := string(Render(Image{
		Src:   `/img/a.png" onerror="alert(1)`,
		Alt:   `<script>alert(1)</script>`,
		Align: AlignCenter,
	}))
	if strings.Contains(html, `onerror="alert(1)"`) {
		t.Errorf("src was not escaped:\n%s", html)
	}
	if strings.Contains(html, "<script>") {
		t.Errorf("alt was not escaped:\n%s", html)
	}
}
