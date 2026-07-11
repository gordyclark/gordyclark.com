// Package books reads the reading-log CSV and computes the aggregates the books
// page displays: a total count, genre distribution (top 8 + "Other"), the top
// authors by count, and a seeded "featured book" pick. It is pure data: chart
// specs and HTML live in the render layer.
package books

import (
	"encoding/csv"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strings"
)

// Book is one row of the reading log.
type Book struct {
	Title  string
	Author string
	Genre  string
	Notes  string
}

// Count is a labeled tally, used for genre and author charts.
type Count struct {
	Label string
	N     int
}

// Data is the full aggregated view the books page renders.
type Data struct {
	Books      []Book  // every row, in file order (for the table)
	Total      int     // total books read
	Authors    int     // distinct authors
	Genres     []Count // top 8 genres + an "Other" fold, largest first
	TopAuthors []Count // top N authors by count, largest first
	GenreCount int     // distinct genres before folding (for the caption)
}

const (
	// topGenres is how many genre slices the pie shows before folding the rest
	// into "Other".
	topGenres = 8
	// topAuthors is how many authors the bar chart shows.
	topAuthors = 10
	// otherLabel names the folded genre slice and the unknown-author bucket.
	otherLabel   = "Other"
	unknownLabel = "Unknown"
)

// Load reads the CSV at path and computes all aggregates. The CSV must have a
// header row with Title, Author, Genre, Notes columns (order-independent). A
// missing file or malformed CSV is an error so the build fails loudly.
func Load(path string) (*Data, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("books: opening %s: %w", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // tolerate ragged rows; we index by header
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("books: parsing %s: %w", path, err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("books: %s has no data rows", path)
	}

	col := headerIndex(records[0])
	for _, want := range []string{"title", "author", "genre"} {
		if _, ok := col[want]; !ok {
			return nil, fmt.Errorf("books: %s missing required column %q", path, want)
		}
	}

	get := func(rec []string, name string) string {
		i, ok := col[name]
		if !ok || i >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[i])
	}

	var list []Book
	for _, rec := range records[1:] {
		title := get(rec, "title")
		if title == "" {
			continue // skip blank lines
		}
		list = append(list, Book{
			Title:  title,
			Author: get(rec, "author"),
			Genre:  get(rec, "genre"),
			Notes:  get(rec, "notes"),
		})
	}

	return aggregate(list), nil
}

// aggregate turns the raw rows into the Data view.
func aggregate(list []Book) *Data {
	genreN := map[string]int{}
	authorN := map[string]int{}
	for _, b := range list {
		g := b.Genre
		if g == "" {
			g = otherLabel
		}
		genreN[g]++
		a := b.Author
		if a == "" {
			a = unknownLabel
		}
		authorN[a]++
	}

	return &Data{
		Books:      list,
		Total:      len(list),
		Authors:    len(authorN),
		Genres:     topWithOther(genreN, topGenres),
		TopAuthors: topN(authorN, topAuthors),
		GenreCount: len(genreN),
	}
}

// Featured returns one book to spotlight, chosen pseudo-randomly from the seed.
// Books that have notes are preferred (the featured card quotes the notes); if
// none have notes, any book may be chosen. Returns the zero Book if the log is
// empty. The seed comes from the caller (cmd/render passes a wall-clock seed)
// so the pick varies build-to-build.
func (d *Data) Featured(seed int64) Book {
	if len(d.Books) == 0 {
		return Book{}
	}
	withNotes := make([]Book, 0, len(d.Books))
	for _, b := range d.Books {
		if strings.TrimSpace(b.Notes) != "" {
			withNotes = append(withNotes, b)
		}
	}
	pool := withNotes
	if len(pool) == 0 {
		pool = d.Books
	}
	r := rand.New(rand.NewSource(seed))
	return pool[r.Intn(len(pool))]
}

func headerIndex(header []string) map[string]int {
	m := map[string]int{}
	for i, h := range header {
		m[strings.ToLower(strings.TrimSpace(h))] = i
	}
	return m
}

// sortedCounts returns counts sorted largest-first, ties broken alphabetically
// for deterministic output across builds.
func sortedCounts(m map[string]int) []Count {
	out := make([]Count, 0, len(m))
	for k, v := range m {
		out = append(out, Count{Label: k, N: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].N != out[j].N {
			return out[i].N > out[j].N
		}
		return out[i].Label < out[j].Label
	})
	return out
}

// topN returns the n largest counts.
func topN(m map[string]int, n int) []Count {
	all := sortedCounts(m)
	if len(all) > n {
		all = all[:n]
	}
	return all
}

// topWithOther returns the n largest counts, folding everything else into a
// single "Other" entry appended at the end (only if there is a remainder).
func topWithOther(m map[string]int, n int) []Count {
	all := sortedCounts(m)
	if len(all) <= n {
		return all
	}
	head := all[:n]
	other := 0
	for _, c := range all[n:] {
		other += c.N
	}
	return append(head, Count{Label: otherLabel, N: other})
}
