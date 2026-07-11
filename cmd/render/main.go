// Command render builds the static essay site: it reads content from --content,
// writes the rendered site to --out, and caches rendered diagrams under --cache.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/gordyclark/gordyclark.com/internal/render"
)

func main() {
	content := flag.String("content", "./content", "path to the content directory")
	out := flag.String("out", "./static", "path to the output directory")
	cache := flag.String("cache", "./.cache", "path to the diagram cache directory")
	assets := flag.String("assets", "./assets", "path to the assets directory (css, fonts)")
	templates := flag.String("templates", "./templates", "path to the templates directory")
	booksCSV := flag.String("books", "books.csv", "path to the reading-log CSV (/books/ page)")
	flag.Parse()

	err := render.Build(render.Options{
		ContentDir:   *content,
		OutDir:       *out,
		CacheDir:     *cache,
		AssetsDir:    *assets,
		TemplatesDir: *templates,
		BooksCSV:     *booksCSV,
		// Seed the featured-book pick from the wall clock so it changes each
		// build. This is the one intentionally non-deterministic bit of the
		// build; it affects only which book the /books/ spotlight shows.
		BooksSeed: time.Now().UnixNano(),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
