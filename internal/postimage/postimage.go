// Package postimage renders the ```img fenced block: an author-placed image
// with an alignment, optional caption, and required alt text.
//
// The block reuses the site's existing attribute grammar (margin.ParseAttrs)
// rather than inventing a second one, and is dispatched from the render
// layer's fenced-block switch alongside ```d2 and ```vega. Authors write:
//
//	```img
//	src="/img/photo.jpg"
//	align="left"
//	alt="A photo of the thing"
//	caption="An optional caption"
//	```
//
// Attributes may be spread across lines or written on one line; the block's
// whole body is parsed as one attribute string.
package postimage

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/gordyclark/gordyclark.com/internal/margin"
)

// Align is a validated image alignment.
type Align string

const (
	AlignLeft   Align = "left"
	AlignRight  Align = "right"
	AlignCenter Align = "center"
)

// DefaultAlign is used when a block omits the align attribute.
const DefaultAlign = AlignCenter

// Image is one parsed ```img block.
type Image struct {
	Src     string
	Alt     string
	Caption string
	Align   Align
}

// Parse reads the body of an ```img block and returns the validated image.
//
// src and alt are required: an image with no alt text would ship an unlabelled
// image, so it fails the build rather than degrading silently. An unrecognised
// align value also fails the build, matching how a malformed chart spec is
// treated — a typo should be loud at build time, not a silently centered image.
func Parse(body string) (Image, error) {
	_, kv := margin.ParseAttrs([]byte(collapse(body)))

	img := Image{
		Src:     strings.TrimSpace(kv["src"]),
		Alt:     strings.TrimSpace(kv["alt"]),
		Caption: strings.TrimSpace(kv["caption"]),
	}

	if img.Src == "" {
		return Image{}, fmt.Errorf("img block: missing required attribute %q", "src")
	}
	if img.Alt == "" {
		return Image{}, fmt.Errorf("img block for %q: missing required attribute %q (alt text is required)", img.Src, "alt")
	}

	switch a := Align(strings.TrimSpace(kv["align"])); a {
	case "":
		img.Align = DefaultAlign
	case AlignLeft, AlignRight, AlignCenter:
		img.Align = a
	default:
		return Image{}, fmt.Errorf("img block for %q: invalid align %q (want left, right or center)", img.Src, string(a))
	}

	return img, nil
}

// collapse joins a multi-line attribute body into a single line. The attribute
// grammar is single-line by design, so newlines become spaces before parsing.
func collapse(body string) string {
	return strings.Join(strings.Fields(body), " ")
}

// Render returns the figure markup for a parsed image. Alignment is expressed
// as a class so all layout and float behaviour lives in CSS, where the
// single-column breakpoint can override it.
func Render(img Image) template.HTML {
	var b strings.Builder
	fmt.Fprintf(&b, `<figure class="post-img align-%s">`, img.Align)
	fmt.Fprintf(&b, `<img src="%s" alt="%s" loading="lazy" decoding="async">`,
		template.HTMLEscapeString(img.Src), template.HTMLEscapeString(img.Alt))
	if img.Caption != "" {
		fmt.Fprintf(&b, `<figcaption>%s</figcaption>`, template.HTMLEscapeString(img.Caption))
	}
	b.WriteString(`</figure>`)
	return template.HTML(b.String())
}
