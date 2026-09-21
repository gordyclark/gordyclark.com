// Package listtoc builds the auto-generated table of contents for list posts.
//
// A list post's items are ordinary level-2 markdown headings ("## Item"). No
// new author syntax is introduced: the TOC is derived by walking the parsed
// document for level-2 headings, assigning each a stable anchor id, and
// emitting a numbered list of plain anchor links.
//
// The links are plain <a href="#id"> anchors. Following one is the reader's
// own click, so the browser's native jump is the only thing that moves the
// page — there is no JavaScript and no :target CSS that would shift layout or
// move the reader's scroll position on its own.
package listtoc

import (
	"fmt"
	"html/template"
	"strings"
	"unicode"
)

// Item is one entry in a list post's table of contents.
type Item struct {
	// Number is the 1-based position of the item in the list.
	Number int
	// Title is the heading's plain text.
	Title string
	// ID is the anchor id assigned to the heading.
	ID string
}

// Slugify converts heading text into an anchor id: lowercase, alphanumerics
// kept, every other run collapsed to a single hyphen, no leading or trailing
// hyphens. A heading with no usable characters yields "".
func Slugify(s string) string {
	var b strings.Builder
	lastHyphen := true // suppresses a leading hyphen
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastHyphen = false
		default:
			if !lastHyphen {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// Assign returns TOC items for the given heading titles, in document order,
// giving each a unique anchor id.
//
// Ids must be unique within a page or the anchors become ambiguous, so a
// repeated (or empty) slug gets a numeric suffix.
func Assign(titles []string) []Item {
	seen := make(map[string]int, len(titles))
	items := make([]Item, 0, len(titles))

	for i, title := range titles {
		base := Slugify(title)
		if base == "" {
			base = "item"
		}
		id := base
		if n, dup := seen[base]; dup {
			seen[base] = n + 1
			id = fmt.Sprintf("%s-%d", base, n+1)
		} else {
			seen[base] = 1
		}
		items = append(items, Item{Number: i + 1, Title: title, ID: id})
	}
	return items
}

// Render returns the table-of-contents markup, or "" when there are no items
// (a list post with no headings simply has no TOC rather than an empty box).
func Render(items []Item) template.HTML {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<nav class="list-toc" aria-label="Contents">`)
	b.WriteString(`<h2 class="list-toc-heading">Contents</h2>`)
	b.WriteString(`<ol class="list-toc-items">`)
	for _, it := range items {
		fmt.Fprintf(&b, `<li><a href="#%s">%s</a></li>`,
			template.HTMLEscapeString(it.ID), template.HTMLEscapeString(it.Title))
	}
	b.WriteString(`</ol></nav>`)
	return template.HTML(b.String())
}
