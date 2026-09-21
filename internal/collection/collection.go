// Package collection turns a CSV of books into the grouped, sorted views a
// collection page renders. The generator does the sorting work once at build
// time so the published page is a static document the browser only has to
// reveal, never compute.
package collection

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"
)

// Book is one row of the source CSV.
type Book struct {
	Title  string
	Author string
	Genre  string
	Notes  string
}

// Section is a run of books under one heading — a letter in the alphabetical
// views, a genre name in the genre view. Anchor is the id the jump bar links
// to; it is assigned here so the bar and the heading cannot disagree.
type Section struct {
	Label  string
	Anchor string
	Books  []Book
}

// View is one complete ordering of the whole collection.
type View struct {
	// Key is the stable identifier used in element ids and CSS selectors
	// ("author", "title", "genre").
	Key string
	// Label is the human-readable name shown on the sort control.
	Label    string
	Sections []Section
}

// UnfiledGenre is the section a book with no genre falls into. A blank genre
// is data drift rather than an authoring error, so it degrades to a section
// instead of failing the build.
const UnfiledGenre = "Unfiled"

// requiredColumns are the CSV headers a collection file must provide.
var requiredColumns = []string{"Title", "Author", "Genre", "Notes"}

// Load reads and validates the CSV at path. It fails on a missing file, a
// missing required column, or any row with a blank title or author — the same
// spirit as the hero/alt rule, where incomplete data fails the build rather
// than shipping a broken page.
func Load(path string) ([]Book, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening collection data: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // validated per row below, with a better message
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%s: parsing CSV: %w", path, err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("%s: file is empty (expected a header row)", path)
	}

	col := make(map[string]int, len(records[0]))
	for i, name := range records[0] {
		col[strings.TrimSpace(name)] = i
	}
	for _, want := range requiredColumns {
		if _, ok := col[want]; !ok {
			return nil, fmt.Errorf("%s: missing required column %q", path, want)
		}
	}

	field := func(rec []string, name string) string {
		i := col[name]
		if i >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[i])
	}

	books := make([]Book, 0, len(records)-1)
	for n, rec := range records[1:] {
		line := n + 2 // 1-based, and the header is line 1
		b := Book{
			Title:  field(rec, "Title"),
			Author: field(rec, "Author"),
			Genre:  field(rec, "Genre"),
			Notes:  field(rec, "Notes"),
		}
		if b.Title == "" && b.Author == "" && b.Genre == "" && b.Notes == "" {
			continue // a blank trailing line is not an error
		}
		if b.Title == "" {
			return nil, fmt.Errorf("%s:%d: row is missing a title", path, line)
		}
		if b.Author == "" {
			return nil, fmt.Errorf("%s:%d: %q is missing an author", path, line, b.Title)
		}
		books = append(books, b)
	}
	return books, nil
}

// Views builds the three orderings the page presents. The first is the default
// the page shows before any reader interaction.
func Views(books []Book) []View {
	return []View{
		{Key: "author", Label: "Author", Sections: byAuthor(books)},
		{Key: "title", Label: "Title", Sections: byTitle(books)},
		{Key: "genre", Label: "Genre", Sections: byGenre(books)},
	}
}

// byAuthor groups by the first letter of the author's surname. The CSV stores
// authors as "Surname, First", so the surname is already leading.
func byAuthor(books []Book) []Section {
	sorted := append([]Book(nil), books...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sortKey(sorted[i].Author), sortKey(sorted[j].Author)
		if a != b {
			return a < b
		}
		return sortKey(titleSortName(sorted[i].Title)) < sortKey(titleSortName(sorted[j].Title))
	})
	return groupByInitial(sorted, "author", func(b Book) string { return b.Author })
}

// byTitle groups by the first letter of the title, ignoring a leading article
// so "The Windup Girl" files under W.
func byTitle(books []Book) []Section {
	sorted := append([]Book(nil), books...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sortKey(titleSortName(sorted[i].Title)), sortKey(titleSortName(sorted[j].Title))
		if a != b {
			return a < b
		}
		return sortKey(sorted[i].Author) < sortKey(sorted[j].Author)
	})
	return groupByInitial(sorted, "title", func(b Book) string { return titleSortName(b.Title) })
}

// byGenre groups by genre name, alphabetically, with books inside each genre
// ordered by author surname. Alphabetical order is predictable and, unlike
// ordering by count, does not reshuffle the page when books are added.
func byGenre(books []Book) []Section {
	grouped := map[string][]Book{}
	for _, b := range books {
		g := b.Genre
		if g == "" {
			g = UnfiledGenre
		}
		grouped[g] = append(grouped[g], b)
	}

	names := make([]string, 0, len(grouped))
	for g := range grouped {
		names = append(names, g)
	}
	sort.Slice(names, func(i, j int) bool { return sortKey(names[i]) < sortKey(names[j]) })

	sections := make([]Section, 0, len(names))
	for _, g := range names {
		in := grouped[g]
		sort.SliceStable(in, func(i, j int) bool {
			a, b := sortKey(in[i].Author), sortKey(in[j].Author)
			if a != b {
				return a < b
			}
			return sortKey(titleSortName(in[i].Title)) < sortKey(titleSortName(in[j].Title))
		})
		sections = append(sections, Section{
			Label:  g,
			Anchor: "genre-" + slugify(g),
			Books:  in,
		})
	}
	return sections
}

// groupByInitial splits an already-sorted slice into one section per leading
// letter. Anything that does not begin with a letter collects under "#".
func groupByInitial(sorted []Book, prefix string, name func(Book) string) []Section {
	var sections []Section
	for _, b := range sorted {
		label := initial(name(b))
		if n := len(sections); n > 0 && sections[n-1].Label == label {
			sections[n-1].Books = append(sections[n-1].Books, b)
			continue
		}
		sections = append(sections, Section{
			Label:  label,
			Anchor: prefix + "-" + slugify(label),
			Books:  []Book{b},
		})
	}
	return sections
}

// initial returns the uppercase first letter of s, or "#" when s does not
// start with one (a numeral, a symbol, or an empty string).
func initial(s string) string {
	r, ok := firstRune(fold(s))
	if !ok || !unicode.IsLetter(r) {
		return "#"
	}
	return string(unicode.ToUpper(r))
}

// firstRune returns the first rune of s, reporting false when s is empty.
func firstRune(s string) (rune, bool) {
	for _, r := range s {
		return r, true
	}
	return 0, false
}

// leadingArticles are stripped from a title before it is sorted or grouped.
var leadingArticles = []string{"the ", "a ", "an "}

// titleSortName returns the title with a leading article removed, so titles
// file under their first meaningful word.
func titleSortName(title string) string {
	lower := strings.ToLower(strings.TrimSpace(title))
	for _, art := range leadingArticles {
		if strings.HasPrefix(lower, art) {
			trimmed := strings.TrimSpace(title[len(art):])
			if trimmed != "" {
				return trimmed
			}
		}
	}
	return strings.TrimSpace(title)
}

// sortKey normalises a string for comparison: case-folded and diacritic-folded,
// so "Ángel" sorts with the As rather than after Z.
func sortKey(s string) string { return strings.ToLower(fold(s)) }

// fold maps accented Latin letters to their unaccented form. It covers the
// range that appears in author names and titles; anything else passes through.
func fold(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if repl, ok := foldMap[r]; ok {
			b.WriteString(repl)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

var foldMap = map[rune]string{
	'á': "a", 'à': "a", 'â': "a", 'ä': "a", 'ã': "a", 'å': "a",
	'Á': "A", 'À': "A", 'Â': "A", 'Ä': "A", 'Ã': "A", 'Å': "A",
	'é': "e", 'è': "e", 'ê': "e", 'ë': "e",
	'É': "E", 'È': "E", 'Ê': "E", 'Ë': "E",
	'í': "i", 'ì': "i", 'î': "i", 'ï': "i",
	'Í': "I", 'Ì': "I", 'Î': "I", 'Ï': "I",
	'ó': "o", 'ò': "o", 'ô': "o", 'ö': "o", 'õ': "o", 'ø': "o",
	'Ó': "O", 'Ò': "O", 'Ô': "O", 'Ö': "O", 'Õ': "O", 'Ø': "O",
	'ú': "u", 'ù': "u", 'û': "u", 'ü': "u",
	'Ú': "U", 'Ù': "U", 'Û': "U", 'Ü': "U",
	'ñ': "n", 'Ñ': "N", 'ç': "c", 'Ç': "C",
	'ß': "ss", 'æ': "ae", 'Æ': "AE", 'œ': "oe", 'Œ': "OE",
}

// slugify turns a section label into an anchor-safe id fragment.
func slugify(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(fold(s)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case !lastDash && b.Len() > 0:
			b.WriteRune('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// JumpEntry is one slot in a view's jump bar. A slot with no books renders as
// greyed, unlinked text rather than disappearing, so the bar keeps a stable
// shape as the collection grows.
type JumpEntry struct {
	Label  string
	Anchor string
	Empty  bool
}

// JumpBar returns the jump-bar slots for a view. The alphabetical views always
// show the full A-Z run (plus "#" when non-letter entries exist) so the bar
// does not reflow; the genre view lists exactly the genres present.
func (v View) JumpBar() []JumpEntry {
	present := make(map[string]Section, len(v.Sections))
	for _, s := range v.Sections {
		present[s.Label] = s
	}

	if v.Key == "genre" {
		entries := make([]JumpEntry, 0, len(v.Sections))
		for _, s := range v.Sections {
			entries = append(entries, JumpEntry{Label: s.Label, Anchor: s.Anchor})
		}
		return entries
	}

	var entries []JumpEntry
	if s, ok := present["#"]; ok {
		entries = append(entries, JumpEntry{Label: "#", Anchor: s.Anchor})
	}
	for r := 'A'; r <= 'Z'; r++ {
		label := string(r)
		if s, ok := present[label]; ok {
			entries = append(entries, JumpEntry{Label: label, Anchor: s.Anchor})
			continue
		}
		entries = append(entries, JumpEntry{Label: label, Empty: true})
	}
	return entries
}

// Total counts the books across a view's sections. Every view holds the whole
// collection, so this is the collection size.
func (v View) Total() int {
	n := 0
	for _, s := range v.Sections {
		n += len(s.Books)
	}
	return n
}
