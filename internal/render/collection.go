package render

import (
	"bytes"
	"fmt"
	"html/template"
	"path/filepath"

	"github.com/gordyclark/gordyclark.com/internal/collection"
	"github.com/gordyclark/gordyclark.com/internal/content"
)

// collectionDataDir is the directory under the content root that holds the CSV
// files collection pages render. A page's `data:` field names a file inside it.
const collectionDataDir = "data"

// renderCollection loads a collection page's CSV and renders every sorted view
// into one block of HTML.
//
// All three views ship in the same document and the reader reveals one with a
// radio button, so switching views costs no request and runs no JavaScript.
// That is what keeps the page inside the site's rule against moving the
// reader's scroll position: there is nothing to move it.
func renderCollection(contentDir string, doc *content.Essay) (template.HTML, error) {
	path := filepath.Join(contentDir, collectionDataDir, doc.Front.Data)
	books, err := collection.Load(path)
	if err != nil {
		return "", fmt.Errorf("%s: %w", doc.SourcePath, err)
	}
	if len(books) == 0 {
		return "", fmt.Errorf("%s: %s holds no books", doc.SourcePath, path)
	}
	return renderViews(collection.Views(books)), nil
}

// renderViews writes the radio inputs, the sort control and every view. The
// inputs come first so the CSS can reach each view with a sibling combinator.
func renderViews(views []collection.View) template.HTML {
	var b bytes.Buffer

	b.WriteString(`<div class="collection">`)

	// One radio per view. The first is checked, so a reader who never touches
	// the control still sees a fully rendered list.
	for i, v := range views {
		checked := ""
		if i == 0 {
			checked = " checked"
		}
		fmt.Fprintf(&b, `<input type="radio" name="collection-sort" id="sort-%s" class="collection-sort-input"%s>`,
			esc(v.Key), checked)
	}

	// The control is a group of labels pointing at those radios. It is marked
	// up as a radiogroup so a screen reader announces it as a set of choices
	// rather than a row of loose text.
	b.WriteString(`<div class="collection-sort" role="radiogroup" aria-label="Sort books by">`)
	b.WriteString(`<span class="collection-sort-label">Sort by</span>`)
	for _, v := range views {
		fmt.Fprintf(&b, `<label class="collection-sort-option" for="sort-%s">%s</label>`,
			esc(v.Key), esc(v.Label))
	}
	b.WriteString(`</div>`)

	for _, v := range views {
		writeView(&b, v)
	}

	b.WriteString(`</div>`)
	return template.HTML(b.String())
}

// writeView writes one complete ordering: its jump bar, then its sections.
func writeView(b *bytes.Buffer, v collection.View) {
	fmt.Fprintf(b, `<div class="collection-view" data-view="%s">`, esc(v.Key))

	// Each view carries its own jump bar, shown and hidden with the view, so
	// the links on screen always match the sections on screen.
	fmt.Fprintf(b, `<nav class="collection-jump" aria-label="Jump to a section, sorted by %s">`, esc(v.Label))
	for _, e := range v.JumpBar() {
		if e.Empty {
			// Rendered but unlinked: the bar keeps a stable shape, and nothing
			// invites a click that would go nowhere.
			fmt.Fprintf(b, `<span class="collection-jump-item is-empty" aria-hidden="true">%s</span>`, esc(e.Label))
			continue
		}
		fmt.Fprintf(b, `<a class="collection-jump-item" href="#%s">%s</a>`, esc(e.Anchor), esc(e.Label))
	}
	// Back to top closes the bar. The bar is sticky, so it is on screen
	// wherever the reader has scrolled to, which is exactly where a way back to
	// the top is wanted. It is a plain anchor, so following it is their click.
	b.WriteString(`<a class="collection-jump-item collection-jump-top" href="#top">&uarr; Top</a>`)
	b.WriteString(`</nav>`)

	fmt.Fprintf(b, `<p class="collection-count">%d books</p>`, v.Total())

	for _, s := range v.Sections {
		fmt.Fprintf(b, `<section class="collection-section">`)
		fmt.Fprintf(b, `<h2 class="collection-section-heading" id="%s">%s</h2>`, esc(s.Anchor), esc(s.Label))
		b.WriteString(`<ul class="book-list">`)
		for _, bk := range s.Books {
			writeBook(b, bk)
		}
		b.WriteString(`</ul></section>`)
	}

	b.WriteString(`</div>`)
}

// writeBook writes one book as a definition-style block: title, then author
// and genre, then the note.
func writeBook(b *bytes.Buffer, bk collection.Book) {
	b.WriteString(`<li class="book">`)
	fmt.Fprintf(b, `<span class="book-title">%s</span>`, esc(bk.Title))

	b.WriteString(`<span class="book-meta">`)
	fmt.Fprintf(b, `<span class="book-author">%s</span>`, esc(bk.Author))
	if bk.Genre != "" {
		// The genre pill reuses the tag palette, so a genre takes a stable
		// color from the same hash that colors tags, with nobody assigning one.
		fmt.Fprintf(b, `<span class="book-genre tag %s">%s</span>`, tagColorClass(bk.Genre), esc(bk.Genre))
	}
	b.WriteString(`</span>`)

	if bk.Notes != "" {
		fmt.Fprintf(b, `<span class="book-note">%s</span>`, esc(bk.Notes))
	}
	b.WriteString(`</li>`)
}

// esc escapes a value for interpolation into the HTML built above. Book data
// comes from a CSV rather than from markdown, so it is never trusted as HTML.
func esc(s string) string { return template.HTMLEscapeString(s) }
