package books

import (
	"os"
	"path/filepath"
	"testing"
)

func writeCSV(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "Books.csv")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const sample = `Title,Author,Genre,Notes
A,"Sanderson, Brandon",Fantasy,x
B,"Sanderson, Brandon",Fantasy,x
C,"Sanderson, Brandon",Fantasy,x
D,"le Guin, Ursula",Science Fiction,y
E,"le Guin, Ursula",Science Fiction,y
F,"Solo, Han",Science Fiction,z
G,"",Nonfiction,no author
`

func TestLoadAggregates(t *testing.T) {
	d, err := Load(writeCSV(t, sample))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if d.Total != 7 {
		t.Errorf("Total = %d, want 7", d.Total)
	}
	// distinct authors: Sanderson, le Guin, Solo, Unknown = 4
	if d.Authors != 4 {
		t.Errorf("Authors = %d, want 4", d.Authors)
	}
	// top author is Sanderson with 3
	if d.TopAuthors[0].Label != "Sanderson, Brandon" || d.TopAuthors[0].N != 3 {
		t.Errorf("TopAuthors[0] = %+v, want Sanderson/3", d.TopAuthors[0])
	}
	// blank author folds to Unknown with 1
	foundUnknown := false
	for _, a := range d.TopAuthors {
		if a.Label == unknownLabel {
			foundUnknown = true
			if a.N != 1 {
				t.Errorf("Unknown author N = %d, want 1", a.N)
			}
		}
	}
	if !foundUnknown {
		t.Error("expected an Unknown author entry")
	}
}

func TestBuckets(t *testing.T) {
	d, _ := Load(writeCSV(t, sample))
	// Sanderson=3, le Guin=2, Solo=1, Unknown=1
	// buckets: read 1 -> 2 authors (Solo, Unknown); read 2 -> 1; read 3 -> 1
	want := map[int]int{1: 2, 2: 1, 3: 1}
	for _, b := range d.Buckets {
		if want[b.Books] != b.Authors {
			t.Errorf("bucket books=%d authors=%d, want %d", b.Books, b.Authors, want[b.Books])
		}
	}
	// ascending order by Books
	for i := 1; i < len(d.Buckets); i++ {
		if d.Buckets[i-1].Books >= d.Buckets[i].Books {
			t.Errorf("buckets not ascending: %+v", d.Buckets)
		}
	}
}

func TestGenreTopWithOther(t *testing.T) {
	// 10 distinct genres, 1 book each, should fold to top 8 + Other.
	csv := "Title,Author,Genre,Notes\n"
	genres := []string{"G1", "G2", "G3", "G4", "G5", "G6", "G7", "G8", "G9", "G10"}
	for i, g := range genres {
		csv += "Book" + string(rune('A'+i)) + ",Auth,\"" + g + "\",n\n"
	}
	d, err := Load(writeCSV(t, csv))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Genres) != topGenres+1 {
		t.Fatalf("got %d genre slices, want %d (top8+Other)", len(d.Genres), topGenres+1)
	}
	last := d.Genres[len(d.Genres)-1]
	if last.Label != otherLabel || last.N != 2 {
		t.Errorf("Other slice = %+v, want Other/2", last)
	}
	if d.GenreCount != 10 {
		t.Errorf("GenreCount = %d, want 10", d.GenreCount)
	}
}

func TestDeterministicTieBreak(t *testing.T) {
	// Equal counts must sort alphabetically for stable builds.
	csv := "Title,Author,Genre,Notes\n" +
		"a,Zed,Zeta,n\n" +
		"b,Amy,Alpha,n\n"
	d, _ := Load(writeCSV(t, csv))
	if d.Genres[0].Label != "Alpha" {
		t.Errorf("tie-break: first genre = %q, want Alpha", d.Genres[0].Label)
	}
}

func TestMissingFileAndColumns(t *testing.T) {
	if _, err := Load("/no/such/file.csv"); err == nil {
		t.Error("expected error for missing file")
	}
	p := writeCSV(t, "Foo,Bar\n1,2\n")
	if _, err := Load(p); err == nil {
		t.Error("expected error for missing required columns")
	}
}
