// Package charts renders Vega-Lite specifications to static, themed SVG.
//
// It mirrors internal/diagrams: a content-addressed cache keyed by the sha256
// of the (theme-merged) spec, a subprocess invocation, and fatal-on-error
// semantics so the site build never silently degrades. The renderer is the
// vl_convert Python module (a Rust-backed, fully offline Vega-Lite compiler);
// it needs no browser, no Node, and no network.
//
// Every spec is merged with a Catppuccin Mocha theme (background, axis, legend,
// text, and categorical color range, plus the site font) so charts match the
// site without per-chart styling. An author's own `config`/`background` in the
// spec wins over the injected defaults.
package charts

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Renderer converts Vega-Lite specs to SVG, caching results under CacheDir.
type Renderer struct {
	CacheDir string
}

// New returns a chart renderer writing its cache under cacheDir.
func New(cacheDir string) *Renderer { return &Renderer{CacheDir: cacheDir} }

// vlConvertScript reads a Vega-Lite spec JSON on stdin and writes SVG on
// stdout using the vl_convert module. Kept tiny and dependency-free.
const vlConvertScript = `
import sys, vl_convert as vlc
spec = sys.stdin.read()
sys.stdout.write(vlc.vegalite_to_svg(vl_spec=spec))
`

// Render compiles a Vega-Lite spec (JSON authored in a ` + "```vega" + ` block)
// to an SVG string. The Mocha theme is merged in first, then the result is
// cached by content hash. A malformed spec or a vl_convert failure returns an
// error (the caller makes it fatal, prepending file/line); no placeholder is
// emitted.
func (r *Renderer) Render(spec string) (string, error) {
	merged, err := mergeTheme(spec)
	if err != nil {
		return "", fmt.Errorf("charts: %w", err)
	}

	sum := sha256.Sum256(merged)
	hash := hex.EncodeToString(sum[:])
	cachePath := filepath.Join(r.CacheDir, "charts", hash+".svg")
	if data, err := os.ReadFile(cachePath); err == nil {
		return string(data), nil
	}

	py, err := exec.LookPath("python3")
	if err != nil {
		return "", fmt.Errorf("charts: python3 not found on PATH (needed for vl_convert): %w", err)
	}
	cmd := exec.Command(py, "-c", vlConvertScript)
	cmd.Stdin = bytes.NewReader(merged)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("charts: vl_convert failed: %v: %s", err, stderr.String())
	}

	svg := stdout.String()
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return "", fmt.Errorf("charts: creating cache dir: %w", err)
	}
	if err := os.WriteFile(cachePath, []byte(svg), 0o644); err != nil {
		return "", fmt.Errorf("charts: writing cache: %w", err)
	}
	return svg, nil
}

// mergeTheme parses the author's Vega-Lite spec and layers the Mocha theme
// under it, so the author's explicit choices win. It returns compact JSON bytes
// suitable for hashing and feeding to vl_convert.
func mergeTheme(spec string) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(spec), &m); err != nil {
		return nil, fmt.Errorf("invalid Vega-Lite JSON: %w", err)
	}

	// Author config (if any) is merged over the theme config, key by key, so
	// authors can override individual theme settings without losing the rest.
	themeCfg := mochaConfig()
	if existing, ok := m["config"].(map[string]any); ok {
		for k, v := range existing {
			themeCfg[k] = v
		}
	}
	m["config"] = themeCfg

	// Default the background to the site's diagram surface unless the author set
	// one. (Vega puts background at the top level, not in config.)
	if _, ok := m["background"]; !ok {
		m["background"] = mochaDiagramBG
	}

	return json.Marshal(m)
}

// Catppuccin Mocha values, matching assets/css/tokens.css. Kept here (not read
// from CSS) because the chart is baked at build time and vl_convert needs
// literal colors, not CSS custom properties.
const (
	mochaText      = "#cdd6f4" // --ink
	mochaMuted     = "#a6adc8" // --muted
	mochaRule      = "#45475a" // --rule
	mochaDiagramBG = "#11111b" // --diagram-bg (crust)
	mochaFont      = "Public Sans, ui-sans-serif, Segoe UI, Arial, system-ui, sans-serif"
)

// mochaCategory is the categorical palette (blue, peach, green, mauve, yellow,
// red, teal, pink) drawn from Mocha's accent colors, for color-by-series.
var mochaCategory = []string{
	"#89b4fa", "#fab387", "#a6e3a1", "#cba6f7",
	"#f9e2af", "#f38ba8", "#94e2d5", "#f5c2e7",
}

// mochaConfig returns a fresh Vega-Lite `config` object themed for Mocha.
// Returned as a map so mergeTheme can overlay author overrides onto it.
func mochaConfig() map[string]any {
	axis := map[string]any{
		"labelColor":  mochaMuted,
		"titleColor":  mochaText,
		"gridColor":   mochaRule,
		"domainColor": mochaRule,
		"tickColor":   mochaRule,
		"labelFont":   mochaFont,
		"titleFont":   mochaFont,
	}
	return map[string]any{
		"font":       mochaFont,
		"background": mochaDiagramBG,
		"view":       map[string]any{"stroke": "transparent"},
		"axis":       axis,
		"legend": map[string]any{
			"labelColor": mochaMuted,
			"titleColor": mochaText,
			"labelFont":  mochaFont,
			"titleFont":  mochaFont,
		},
		"title": map[string]any{
			"color":         mochaText,
			"subtitleColor": mochaMuted,
			"font":          mochaFont,
			"subtitleFont":  mochaFont,
		},
		"range": map[string]any{
			"category": mochaCategory,
		},
		"mark": map[string]any{
			"color": mochaCategory[0],
		},
	}
}
