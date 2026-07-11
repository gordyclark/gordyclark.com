package diagrams

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
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

func TestRenderCacheMissNoD2(t *testing.T) {
	if _, err := exec.LookPath("d2"); err == nil {
		t.Skip("d2 is installed; cannot test the missing-binary path")
	}

	dir := t.TempDir()
	_, err := New(dir).Render("x -> y")
	if err == nil {
		t.Fatal("expected error on cache miss with d2 absent, got nil")
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
