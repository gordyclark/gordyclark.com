package render

import (
	"strings"
	"testing"
)

// Body cells carry their column's header text so the narrow-screen layout can
// print it as a label — CSS can render an attribute but cannot go looking for
// the header cell above.
func TestLabelTableCellsCopiesHeadersOntoCells(t *testing.T) {
	in := "<table>\n<thead>\n<tr>\n<th>Title</th>\n<th>Author</th>\n</tr>\n</thead>\n" +
		"<tbody>\n<tr>\n<td>Dune</td>\n<td>Herbert</td>\n</tr>\n</tbody>\n</table>"
	got := labelTableCells(in)
	for _, want := range []string{
		`<td data-label="Title">Dune</td>`,
		`<td data-label="Author">Herbert</td>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in:\n%s", want, got)
		}
	}
	// Header cells are left alone; they are their own label.
	if strings.Contains(got, `<th data-label`) {
		t.Error("header cells must not be labelled")
	}
}

// goldmark writes alignment onto the cell tags, so the labeller has to cope
// with cells that already carry attributes.
func TestLabelTableCellsKeepsExistingCellAttributes(t *testing.T) {
	in := `<table>` + "\n<thead>\n<tr>\n" + `<th align="left">Title</th>` + "\n</tr>\n</thead>\n" +
		"<tbody>\n<tr>\n" + `<td align="left">Dune</td>` + "\n</tr>\n</tbody>\n</table>"
	got := labelTableCells(in)
	if !strings.Contains(got, `<td data-label="Title" align="left">Dune</td>`) {
		t.Errorf("alignment attribute lost:\n%s", got)
	}
}

// A header written with inline markup still yields a plain-text label, and one
// containing a quote cannot break out of the attribute.
func TestLabelTableCellsSanitisesLabels(t *testing.T) {
	in := "<table>\n<thead>\n<tr>\n<th><em>Ti\"tle</em></th>\n</tr>\n</thead>\n" +
		"<tbody>\n<tr>\n<td>Dune</td>\n</tr>\n</tbody>\n</table>"
	got := labelTableCells(in)
	if !strings.Contains(got, `<td data-label="Ti&#34;tle">Dune</td>`) {
		t.Errorf("label not stripped and escaped:\n%s", got)
	}
}

// Rows are labelled independently: the column counter resets on every <tr>, so
// a short row cannot shift the labels of the rows after it.
func TestLabelTableCellsResetsPerRow(t *testing.T) {
	in := "<table>\n<thead>\n<tr>\n<th>A</th>\n<th>B</th>\n</tr>\n</thead>\n<tbody>\n" +
		"<tr>\n<td>1</td>\n</tr>\n<tr>\n<td>2</td>\n<td>3</td>\n</tr>\n</tbody>\n</table>"
	got := labelTableCells(in)
	for _, want := range []string{
		`<td data-label="A">1</td>`,
		`<td data-label="A">2</td>`,
		`<td data-label="B">3</td>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in:\n%s", want, got)
		}
	}
}

// A table with no header row has no labels to copy, and must come back
// untouched rather than mangled.
func TestLabelTableCellsLeavesHeaderlessTableAlone(t *testing.T) {
	in := "<table>\n<tbody>\n<tr>\n<td>Dune</td>\n</tr>\n</tbody>\n</table>"
	if got := labelTableCells(in); got != in {
		t.Errorf("headerless table was rewritten:\n%s", got)
	}
}
