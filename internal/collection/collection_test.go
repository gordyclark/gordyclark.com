package collection

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeCSV writes a temporary CSV and returns its path.
func writeCSV(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "books.csv")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	return path
}

const header = "Title,Author,Genre,Notes\n"

func TestLoadParsesRowsAndTrimsFields(t *testing.T) {
	path := writeCSV(t, header+
		"Eisenhorn Omnibus,\"Abnett, Dan\",WH40K,\"Nice atmosphere, subpar mystery\"\n"+
		"  Bunny  ,\"Awad, Mona\",Literature,Mean Girls with rabbits\n")

	books, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(books) != 2 {
		t.Fatalf("got %d books, want 2", len(books))
	}
	if books[0].Title != "Eisenhorn Omnibus" || books[0].Author != "Abnett, Dan" {
		t.Errorf("first row parsed as %+v", books[0])
	}
	// A quoted field containing a comma must survive as one field.
	if books[0].Notes != "Nice atmosphere, subpar mystery" {
		t.Errorf("quoted notes mangled: %q", books[0].Notes)
	}
	if books[1].Title != "Bunny" || books[1].Author != "Awad, Mona" {
		t.Errorf("whitespace not trimmed: %+v", books[1])
	}
}

func TestLoadSkipsBlankTrailingRows(t *testing.T) {
	path := writeCSV(t, header+"Bunny,\"Awad, Mona\",Literature,Note\n,,,\n")
	books, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(books) != 1 {
		t.Fatalf("got %d books, want 1 (blank row should be skipped)", len(books))
	}
}

func TestLoadFailsOnMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "absent.csv")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func TestLoadFailsOnMissingColumn(t *testing.T) {
	path := writeCSV(t, "Title,Author,Notes\nBunny,\"Awad, Mona\",Note\n")
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected an error for a missing Genre column")
	}
	if !strings.Contains(err.Error(), "Genre") {
		t.Errorf("error should name the missing column, got: %v", err)
	}
}

func TestLoadFailsOnRowMissingTitleOrAuthor(t *testing.T) {
	t.Run("missing title", func(t *testing.T) {
		path := writeCSV(t, header+",\"Awad, Mona\",Literature,Note\n")
		err := mustLoadError(t, path)
		if !strings.Contains(err.Error(), "title") {
			t.Errorf("error should mention the title, got: %v", err)
		}
		// The message must point at the offending line, header included.
		if !strings.Contains(err.Error(), ":2:") {
			t.Errorf("error should carry the line number, got: %v", err)
		}
	})

	t.Run("missing author", func(t *testing.T) {
		path := writeCSV(t, header+"Bunny,,Literature,Note\n")
		err := mustLoadError(t, path)
		if !strings.Contains(err.Error(), "Bunny") {
			t.Errorf("error should name the book, got: %v", err)
		}
	})
}

func mustLoadError(t *testing.T, path string) error {
	t.Helper()
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected an error")
	}
	return err
}

// sample covers the ordering edge cases: a leading article, a lowercase
// surname, a diacritic, a numeral-leading title and a blank genre.
var sample = []Book{
	{Title: "The Windup Girl", Author: "Bacigalupi, Paolo", Genre: "Science Fiction"},
	{Title: "Bunny", Author: "Awad, Mona", Genre: "Literature"},
	{Title: "An Empty Room", Author: "Zhu, Wen", Genre: "Literature"},
	{Title: "1984", Author: "Orwell, George", Genre: ""},
	{Title: "Cien Años", Author: "Ángel, Ana", Genre: "Literature"},
}

func viewByKey(t *testing.T, key string) View {
	t.Helper()
	for _, v := range Views(sample) {
		if v.Key == key {
			return v
		}
	}
	t.Fatalf("no view with key %q", key)
	return View{}
}

func TestViewsReturnsThreeOrderingsWithAuthorFirst(t *testing.T) {
	views := Views(sample)
	if len(views) != 3 {
		t.Fatalf("got %d views, want 3", len(views))
	}
	// The first view is the one the page shows by default.
	if views[0].Key != "author" {
		t.Errorf("default view is %q, want \"author\"", views[0].Key)
	}
	var keys []string
	for _, v := range views {
		keys = append(keys, v.Key)
	}
	if got := strings.Join(keys, ","); got != "author,title,genre" {
		t.Errorf("view keys = %q", got)
	}
}

func TestEveryViewHoldsTheWholeCollection(t *testing.T) {
	for _, v := range Views(sample) {
		if v.Total() != len(sample) {
			t.Errorf("view %q holds %d books, want %d", v.Key, v.Total(), len(sample))
		}
	}
}

func TestAuthorViewGroupsBySurnameInitial(t *testing.T) {
	v := viewByKey(t, "author")
	got := sectionLabels(v)
	// Ángel folds to A and sorts with the As, ahead of Awad.
	want := "A,B,O,Z"
	if got != want {
		t.Errorf("author sections = %q, want %q", got, want)
	}
	if first := v.Sections[0].Books[0].Author; first != "Ángel, Ana" {
		t.Errorf("diacritic author sorted to %q, want it first among the As", first)
	}
}

func TestTitleViewIgnoresLeadingArticles(t *testing.T) {
	v := viewByKey(t, "title")
	// "The Windup Girl" files under W, "An Empty Room" under E, "1984" under #.
	got := sectionLabels(v)
	want := "#,B,C,E,W"
	if got != want {
		t.Errorf("title sections = %q, want %q", got, want)
	}
	for _, s := range v.Sections {
		if s.Label == "W" && s.Books[0].Title != "The Windup Girl" {
			t.Errorf("W section holds %q", s.Books[0].Title)
		}
	}
}

func TestTitleSortNameStripsOnlyWholeLeadingWords(t *testing.T) {
	cases := map[string]string{
		"The Windup Girl": "Windup Girl",
		"An Empty Room":   "Empty Room",
		"A Wizard of Us":  "Wizard of Us",
		"Theory of Games": "Theory of Games", // "The" must not match inside "Theory"
		"Antarctica":      "Antarctica",      // nor "An" inside "Antarctica"
		"The":             "The",             // stripping everything would leave nothing
	}
	for in, want := range cases {
		if got := titleSortName(in); got != want {
			t.Errorf("titleSortName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGenreViewIsAlphabeticalAndUnfilesBlankGenres(t *testing.T) {
	v := viewByKey(t, "genre")
	got := sectionLabels(v)
	want := "Literature,Science Fiction,Unfiled"
	if got != want {
		t.Errorf("genre sections = %q, want %q", got, want)
	}
	// Within a genre, books order by author surname.
	for _, s := range v.Sections {
		if s.Label != "Literature" {
			continue
		}
		var authors []string
		for _, b := range s.Books {
			authors = append(authors, b.Author)
		}
		if got := strings.Join(authors, "|"); got != "Ángel, Ana|Awad, Mona|Zhu, Wen" {
			t.Errorf("Literature authors = %q", got)
		}
	}
}

func TestAnchorsAreUniqueAcrossAllViews(t *testing.T) {
	seen := map[string]string{}
	for _, v := range Views(sample) {
		for _, s := range v.Sections {
			if s.Anchor == "" {
				t.Errorf("view %q section %q has no anchor", v.Key, s.Label)
			}
			if prev, dup := seen[s.Anchor]; dup {
				t.Errorf("anchor %q used by both %s and %s/%s", s.Anchor, prev, v.Key, s.Label)
			}
			seen[s.Anchor] = v.Key + "/" + s.Label
		}
	}
}

func TestJumpBarCoversTheFullAlphabetAndMarksEmptyLetters(t *testing.T) {
	v := viewByKey(t, "author")
	bar := v.JumpBar()

	// "#" only appears when a non-letter section exists; the author view has none.
	if bar[0].Label != "A" {
		t.Errorf("author jump bar starts at %q, want \"A\"", bar[0].Label)
	}
	if len(bar) != 26 {
		t.Fatalf("author jump bar has %d entries, want 26", len(bar))
	}

	byLabel := map[string]JumpEntry{}
	for _, e := range bar {
		byLabel[e.Label] = e
	}
	if byLabel["A"].Empty || byLabel["A"].Anchor == "" {
		t.Error("A has books, so it must be linked")
	}
	// No author surname starts with Q here.
	if !byLabel["Q"].Empty {
		t.Error("Q has no books, so it must render as an empty slot")
	}
	if byLabel["Q"].Anchor != "" {
		t.Error("an empty slot must not carry an anchor")
	}
}

func TestJumpBarIncludesHashWhenNonLetterSectionExists(t *testing.T) {
	v := viewByKey(t, "title")
	bar := v.JumpBar()
	if bar[0].Label != "#" {
		t.Errorf("title jump bar starts at %q, want \"#\" (1984 sorts there)", bar[0].Label)
	}
	if bar[0].Empty {
		t.Error("the # slot has a book, so it must be linked")
	}
}

func TestJumpBarForGenreListsOnlyPresentGenres(t *testing.T) {
	v := viewByKey(t, "genre")
	bar := v.JumpBar()
	if len(bar) != len(v.Sections) {
		t.Fatalf("genre jump bar has %d entries, want %d", len(bar), len(v.Sections))
	}
	for _, e := range bar {
		if e.Empty {
			t.Errorf("genre slot %q is empty; the bar should list only present genres", e.Label)
		}
	}
}

// TestJumpBarAnchorsMatchSectionAnchors guards the invariant that makes the
// bar work at all: every link must point at a section that exists on the page.
func TestJumpBarAnchorsMatchSectionAnchors(t *testing.T) {
	for _, v := range Views(sample) {
		anchors := map[string]bool{}
		for _, s := range v.Sections {
			anchors[s.Anchor] = true
		}
		for _, e := range v.JumpBar() {
			if e.Empty {
				continue
			}
			if !anchors[e.Anchor] {
				t.Errorf("view %q: jump link %q points at %q, which is not a section", v.Key, e.Label, e.Anchor)
			}
		}
	}
}

func TestViewsDoesNotMutateInput(t *testing.T) {
	books := append([]Book(nil), sample...)
	Views(books)
	for i := range books {
		if books[i] != sample[i] {
			t.Fatalf("Views reordered the caller's slice at %d", i)
		}
	}
}

func sectionLabels(v View) string {
	var labels []string
	for _, s := range v.Sections {
		labels = append(labels, s.Label)
	}
	return strings.Join(labels, ",")
}
