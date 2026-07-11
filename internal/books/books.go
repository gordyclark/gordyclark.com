// Package books reads the reading-log CSV and computes the aggregates the books
// page displays: a total count, genre distribution (top 8 + "Other"), the top
// authors by count, and the distribution of how many authors have been read N
// times. It is pure data: chart specs and HTML live in the render layer.
package books

import (
	"encoding/csv"
	"fmt"
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

// Bucket is one "read N books by an author" group: N books each, by Authors
// distinct authors.
type Bucket struct {
	Books   int // books read per author in this bucket (1, 2, 3, …)
	Authors int // how many distinct authors fall in this bucket
}

// Data is the full aggregated view the books page renders.
type Data struct {
	Books      []Book   // every row, in file order (for the table)
	Total      int      // total books read
	Authors    int      // distinct authors
	Genres     []Count  // top 8 genres + an "Other" fold, largest first
	TopAuthors []Count  // top N authors by count, largest first
	Buckets    []Bucket // read-frequency distribution, by books-per-author asc
	GenreCount int      // distinct genres before folding (for the caption)
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
		Buckets:    frequencyBuckets(authorN),
		GenreCount: len(genreN),
	}
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

// frequencyBuckets computes, for each distinct books-per-author value N, how
// many authors were read exactly N times. Returned ascending by N.
func frequencyBuckets(authorN map[string]int) []Bucket {
	byFreq := map[int]int{}
	for _, n := range authorN {
		byFreq[n]++
	}
	out := make([]Bucket, 0, len(byFreq))
	for books, authors := range byFreq {
		out = append(out, Bucket{Books: books, Authors: authors})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Books < out[j].Books })
	return out
}
