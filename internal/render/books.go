package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"path/filepath"

	"github.com/gordyclark/gordyclark.com/internal/books"
	"github.com/gordyclark/gordyclark.com/internal/charts"
)

// booksPageData is the data the books template renders.
type booksPageData struct {
	pageData
	Total      int
	Authors    int
	GenreCount int
	GenrePie   template.HTML // rendered SVG (themed)
	AuthorBar  template.HTML // rendered SVG (themed)
	Featured   books.Book    // spotlight book, re-picked each build
	Rows       []books.Book
}

// booksCategoryPalette is a categorical palette derived from Catppuccin Mocha
// but deepened/saturated so it passes the dataviz validator on the dark chart
// surface (lightness band, chroma floor, CVD separation ΔE ≈ 12.9). Used for
// the genre pie's real slices; the folded "Other" slice uses a muted gray.
var booksCategoryPalette = []string{
	"#5b8ef5", "#cf7a0a", "#3fa34d", "#a855e0",
	"#b8880f", "#e14b6a", "#159b8a", "#c86fb0",
}

const bucketOtherGray = "#6c7086" // Mocha overlay0, for the "Other" pie slice

// writeBooksPage loads the reading log, renders its charts, and writes
// static/books/index.html. Called every build so new CSV rows flow straight
// into the charts and table. seed varies the featured-book pick per build.
func writeBooksPage(tmpl *template.Template, outDir, stylesheet, csvPath string, cr *charts.Renderer, seed int64) error {
	data, err := books.Load(csvPath)
	if err != nil {
		return err
	}

	pie, err := cr.Render(genrePieSpec(data.Genres))
	if err != nil {
		return fmt.Errorf("rendering genre pie: %w", err)
	}
	bar, err := cr.Render(authorBarSpec(data.TopAuthors))
	if err != nil {
		return fmt.Errorf("rendering author bar: %w", err)
	}

	pd := booksPageData{
		pageData: pageData{
			Title:          "Books",
			Subtitle:       fmt.Sprintf("%d books read", data.Total),
			StylesheetPath: stylesheet,
		},
		Total:      data.Total,
		Authors:    data.Authors,
		GenreCount: data.GenreCount,
		GenrePie:   template.HTML(pie), //nolint:gosec // trusted, build-time SVG
		AuthorBar:  template.HTML(bar), //nolint:gosec // trusted, build-time SVG
		Featured:   data.Featured(seed),
		Rows:       data.Books,
	}

	set, err := templateSetFor(tmpl, "books.html.tmpl")
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := set.ExecuteTemplate(&buf, "base", pd); err != nil {
		return fmt.Errorf("executing books template: %w", err)
	}
	return writeFile(filepath.Join(outDir, "books", "index.html"), buf.Bytes())
}

// genrePieSpec builds a Vega-Lite donut spec for the genre distribution. Colors
// are assigned in fixed order from the validated palette; the "Other" slice is
// pinned to a muted gray so the fold reads as "everything else".
func genrePieSpec(genres []books.Count) string {
	labels := make([]string, len(genres))
	colors := make([]string, len(genres))
	ci := 0
	for i, g := range genres {
		labels[i] = g.Label
		if g.Label == "Other" {
			colors[i] = bucketOtherGray
		} else {
			colors[i] = booksCategoryPalette[ci%len(booksCategoryPalette)]
			ci++
		}
	}
	values := make([]map[string]any, len(genres))
	for i, g := range genres {
		values[i] = map[string]any{"genre": g.Label, "n": g.N}
	}
	spec := map[string]any{
		"data": map[string]any{"values": values},
		"mark": map[string]any{"type": "arc", "innerRadius": 55, "stroke": "#11111b", "strokeWidth": 2},
		"encoding": map[string]any{
			"theta": map[string]any{"field": "n", "type": "quantitative", "stack": true},
			"color": map[string]any{
				"field": "genre", "type": "nominal",
				"scale":  map[string]any{"domain": labels, "range": colors},
				"legend": map[string]any{"title": "Genre"},
			},
			"order": map[string]any{"field": "n", "type": "quantitative", "sort": "descending"},
			"tooltip": []map[string]any{
				{"field": "genre", "type": "nominal", "title": "Genre"},
				{"field": "n", "type": "quantitative", "title": "Books"},
			},
		},
		"width":  260,
		"height": 260,
		"view":   map[string]any{"stroke": nil},
	}
	return mustJSON(spec)
}

// authorBarSpec builds a horizontal bar chart of the top authors by count
// (single series, so one color — the Mocha blue accent).
func authorBarSpec(authors []books.Count) string {
	values := make([]map[string]any, len(authors))
	for i, a := range authors {
		values[i] = map[string]any{"author": a.Label, "n": a.N}
	}
	spec := map[string]any{
		"data": map[string]any{"values": values},
		"mark": map[string]any{"type": "bar", "cornerRadiusEnd": 4, "color": "#89b4fa"},
		"encoding": map[string]any{
			"y": map[string]any{
				"field": "author", "type": "nominal",
				"sort":  map[string]any{"field": "n", "order": "descending"},
				"title": nil,
			},
			"x": map[string]any{
				"field": "n", "type": "quantitative",
				"title": "Books read",
				"axis":  map[string]any{"tickMinStep": 1},
			},
			"tooltip": []map[string]any{
				{"field": "author", "type": "nominal", "title": "Author"},
				{"field": "n", "type": "quantitative", "title": "Books"},
			},
		},
		"width":  360,
		"height": map[string]any{"step": 26},
	}
	return mustJSON(spec)
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		// The specs are built from static shapes; marshalling cannot realistically
		// fail. Panic would only fire on a programming error, caught in tests.
		panic(fmt.Sprintf("books: marshalling chart spec: %v", err))
	}
	return string(b)
}
