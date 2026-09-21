package diagrams

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// hashPath mirrors the production sha256 cache-path computation.
func hashPath(cacheDir, source string) string {
	sum := sha256.Sum256([]byte(source))
	return filepath.Join(cacheDir, "diagrams", hex.EncodeToString(sum[:])+".svg")
}

func TestRenderCacheHit(t *testing.T) {
	dir := t.TempDir()
	const source = "a -> b"
	const sentinel = "<svg>CACHED</svg>"

	path := hashPath(dir, source)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(sentinel), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := New(dir).Render(source)
	if err != nil {
		t.Fatalf("Render returned error on cache hit: %v", err)
	}
	if got != sentinel {
		t.Fatalf("cache hit = %q, want %q", got, sentinel)
	}
}

// Rendering is in-process, so a cache miss renders rather than depending on a
// d2 binary being installed. This test is unconditional for that reason.
func TestRenderCacheMissRendersInProcess(t *testing.T) {
	dir := t.TempDir()
	svg, err := New(dir).Render("x -> y")
	if err != nil {
		t.Fatalf("Render on cache miss: %v", err)
	}
	if !strings.Contains(svg, "<svg") {
		t.Errorf("expected SVG output, got %.80q", svg)
	}
	// The site ships no JavaScript; an embedded <script> would break that.
	if strings.Contains(svg, "<script") {
		t.Error("rendered diagram must not contain a <script> tag")
	}
	// The miss must have populated the cache for the next build.
	if _, err := os.Stat(New(dir).cachePath("x -> y")); err != nil {
		t.Errorf("cache not written after a miss: %v", err)
	}
}

// A malformed diagram must fail the build rather than emit a broken figure.
func TestRenderInvalidSourceFails(t *testing.T) {
	dir := t.TempDir()
	if _, err := New(dir).Render("a -> "); err == nil {
		t.Fatal("expected an error for malformed d2 source")
	}
}

func TestCachePathStable(t *testing.T) {
	r := New("/cache")
	const source = "foo -> bar"
	if r.cachePath(source) != r.cachePath(source) {
		t.Fatal("cachePath is not stable for identical input")
	}
	if r.cachePath(source) == r.cachePath(source+" ") {
		t.Fatal("cachePath collides for differing input")
	}
}
