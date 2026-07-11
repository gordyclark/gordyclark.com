package charts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const sampleSpec = `{"data":{"values":[{"a":"x","b":1}]},"mark":"bar","encoding":{"x":{"field":"a","type":"nominal"},"y":{"field":"b","type":"quantitative"}}}`

func TestMergeThemeInjectsMochaConfig(t *testing.T) {
	out, err := mergeTheme(sampleSpec)
	if err != nil {
		t.Fatalf("mergeTheme: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("result not valid JSON: %v", err)
	}
	if m["background"] != mochaDiagramBG {
		t.Errorf("background = %v, want %s", m["background"], mochaDiagramBG)
	}
	cfg, ok := m["config"].(map[string]any)
	if !ok {
		t.Fatalf("config missing or wrong type: %T", m["config"])
	}
	if cfg["font"] != mochaFont {
		t.Errorf("config.font = %v, want site font", cfg["font"])
	}
	// original spec content preserved
	if m["mark"] != "bar" {
		t.Errorf("author's mark lost: %v", m["mark"])
	}
}

func TestMergeThemeAuthorConfigWins(t *testing.T) {
	spec := `{"mark":"line","config":{"font":"Comic Sans","background":"#fff"},"background":"#123456"}`
	out, err := mergeTheme(spec)
	if err != nil {
		t.Fatalf("mergeTheme: %v", err)
	}
	var m map[string]any
	_ = json.Unmarshal(out, &m)
	cfg := m["config"].(map[string]any)
	if cfg["font"] != "Comic Sans" {
		t.Errorf("author config.font should win, got %v", cfg["font"])
	}
	// author-set top-level background must be preserved (not overwritten)
	if m["background"] != "#123456" {
		t.Errorf("author background should win, got %v", m["background"])
	}
	// theme keys the author didn't set are still present
	if _, ok := cfg["axis"]; !ok {
		t.Errorf("theme axis config should remain when author overrides only font")
	}
}

func TestMergeThemeRejectsBadJSON(t *testing.T) {
	if _, err := mergeTheme("{not json"); err == nil {
		t.Fatal("expected error on malformed spec")
	}
}

func TestRenderCacheHitSkipsSubprocess(t *testing.T) {
	dir := t.TempDir()
	r := New(dir)
	// Pre-seed the cache at the exact hash of the merged spec.
	merged, err := mergeTheme(sampleSpec)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(merged)
	hash := hex.EncodeToString(sum[:])
	seeded := "<svg>SEEDED-CHART</svg>"
	writeSeed(t, filepath.Join(dir, "charts", hash+".svg"), seeded)

	got, err := r.Render(sampleSpec)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got != seeded {
		t.Errorf("cache hit not used: got %q", got)
	}
}

// TestRenderProducesSVG exercises the real vl_convert path when python3 with the
// module is available; otherwise it is skipped (e.g. outside the nix shell).
func TestRenderProducesSVG(t *testing.T) {
	if !vlConvertAvailable() {
		t.Skip("python3 with vl_convert not available; skipping live render")
	}
	dir := t.TempDir()
	r := New(dir)
	svg, err := r.Render(sampleSpec)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(svg, "<svg") {
		t.Errorf("output is not SVG:\n%s", svg[:min(200, len(svg))])
	}
	// second call should hit cache and return identical bytes
	svg2, err := r.Render(sampleSpec)
	if err != nil || svg2 != svg {
		t.Errorf("second Render differs or errored: %v", err)
	}
}

func vlConvertAvailable() bool {
	py, err := exec.LookPath("python3")
	if err != nil {
		return false
	}
	return exec.Command(py, "-c", "import vl_convert").Run() == nil
}

func writeSeed(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
