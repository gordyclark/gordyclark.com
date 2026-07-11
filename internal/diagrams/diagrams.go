// Package diagrams renders d2 diagram source into inline SVG, caching
// results on disk keyed by the sha256 of the source text.
package diagrams

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
// hit the cached SVG is returned without invoking d2. On a miss it invokes the
// d2 binary; if d2 is missing or fails, an informative error (including d2's
// stderr) is returned rather than a placeholder.
func (r *Renderer) Render(source string) (string, error) {
	path := r.cachePath(source)

	if data, err := os.ReadFile(path); err == nil {
		return string(data), nil
	}

	if _, err := exec.LookPath("d2"); err != nil {
		return "", fmt.Errorf("diagrams: d2 binary not found on PATH: %w", err)
	}

	cmd := exec.Command("d2", "-", "-")
	cmd.Stdin = strings.NewReader(source)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("diagrams: d2 render failed: %v: %s", err, strings.TrimSpace(stderr.String()))
	}

	svg := stdout.String()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("diagrams: creating cache dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(svg), 0o644); err != nil {
		return "", fmt.Errorf("diagrams: writing cache file: %w", err)
	}

	return svg, nil
}
