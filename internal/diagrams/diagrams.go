// Package diagrams renders d2 diagram source into inline SVG, caching
// results on disk keyed by the sha256 of the source text.
//
// Rendering happens in-process via the d2 Go library, not by shelling out to a
// d2 binary. Since d2 v0.8.2 the dagre and ELK layout engines are native Go
// ports rather than embedded JavaScript, so the whole pipeline is pure Go with
// no CGO, no JS runtime and nothing to install. That is what lets the site
// build in environments that only provide a Go toolchain (Cloudflare Pages).
package diagrams

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/d2lang/d2/d2graph"
	"github.com/d2lang/d2/d2layouts/d2dagrelayout"
	"github.com/d2lang/d2/d2lib"
	"github.com/d2lang/d2/d2renderers/d2svg"
	"github.com/d2lang/d2/lib/log"
	"github.com/d2lang/d2/lib/textmeasure"
)

// Renderer renders d2 sources to SVG, caching output under CacheDir.
type Renderer struct {
	CacheDir string
}

// New returns a Renderer that caches rendered diagrams under cacheDir.
func New(cacheDir string) *Renderer {
	return &Renderer{CacheDir: cacheDir}
}

// cachePath returns the on-disk cache path for a given source string.
func (r *Renderer) cachePath(source string) string {
	sum := sha256.Sum256([]byte(source))
	hash := hex.EncodeToString(sum[:])
	return filepath.Join(r.CacheDir, "diagrams", hash+".svg")
}

// Render returns the inline SVG for the given d2 diagram source. On a cache
// hit the cached SVG is returned without re-rendering. A malformed diagram
// returns an informative error rather than a placeholder, so a typo fails the
// build instead of shipping a broken figure.
func (r *Renderer) Render(source string) (string, error) {
	path := r.cachePath(source)

	if data, err := os.ReadFile(path); err == nil {
		return string(data), nil
	}

	svg, err := renderSVG(source)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("diagrams: creating cache dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(svg), 0o644); err != nil {
		return "", fmt.Errorf("diagrams: writing cache file: %w", err)
	}

	return svg, nil
}

// renderSVG compiles d2 source to a standalone SVG string. The returned SVG
// embeds its own fonts, so it does not depend on fonts being installed on the
// build machine.
func renderSVG(source string) (string, error) {
	// d2 pulls its logger out of the context and dumps a goroutine stack trace
	// to stderr if one is absent. Discard its output so build logs stay clean;
	// real failures come back as errors, not log lines.
	ctx := log.With(context.Background(), slog.New(slog.DiscardHandler))

	ruler, err := textmeasure.NewRuler()
	if err != nil {
		return "", fmt.Errorf("diagrams: creating text ruler: %w", err)
	}

	// LayoutResolver receives the engine name requested by the diagram; this
	// site always uses dagre.
	layout := func(string) (d2graph.LayoutGraph, error) {
		return d2dagrelayout.DefaultLayout, nil
	}

	diagram, _, err := d2lib.Compile(ctx, source,
		&d2lib.CompileOptions{LayoutResolver: layout, Ruler: ruler},
		&d2svg.RenderOpts{})
	if err != nil {
		return "", fmt.Errorf("diagrams: d2 compile failed: %w", err)
	}

	svg, err := d2svg.Render(diagram, &d2svg.RenderOpts{})
	if err != nil {
		return "", fmt.Errorf("diagrams: d2 render failed: %w", err)
	}
	return string(svg), nil
}
