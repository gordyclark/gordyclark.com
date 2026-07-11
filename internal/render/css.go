package render

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// buildCSS reads the manifest at <assetsDir>/css/manifest.txt, concatenates the
// listed CSS files (in order, each resolved relative to <assetsDir>/css), hashes
// the concatenation, writes the bundle to <outDir>/style.<hash>.css and returns
// the bare hashed filename (e.g. "style.abc12345.css").
//
// The hash is the first 8 hex characters of the sha256 of the concatenated
// bytes, so identical inputs always yield the same filename (deterministic
// output — acceptance criterion 7) and any change to any listed CSS file changes
// the hash.
func buildCSS(assetsDir, outDir string) (string, error) {
	cssDir := filepath.Join(assetsDir, "css")
	manifestPath := filepath.Join(cssDir, "manifest.txt")

	f, err := os.Open(manifestPath)
	if err != nil {
		return "", fmt.Errorf("reading css manifest %s: %w", manifestPath, err)
	}
	defer f.Close()

	var buf bytes.Buffer
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cssPath := filepath.Join(cssDir, filepath.FromSlash(line))
		data, err := os.ReadFile(cssPath)
		if err != nil {
			return "", fmt.Errorf("reading css file %q listed in manifest: %w", line, err)
		}
		buf.Write(data)
		// Guarantee a separating newline between files so concatenation never
		// glues two rules together.
		if len(data) > 0 && data[len(data)-1] != '\n' {
			buf.WriteByte('\n')
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scanning css manifest %s: %w", manifestPath, err)
	}

	sum := sha256.Sum256(buf.Bytes())
	hash := hex.EncodeToString(sum[:])[:8]
	name := fmt.Sprintf("style.%s.css", hash)

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("creating out dir %s: %w", outDir, err)
	}
	// Remove any stale hashed stylesheets from a previous build so a rebuild
	// into an existing out dir does not leave orphans that `rclone sync` would
	// otherwise push to the bucket.
	if old, _ := filepath.Glob(filepath.Join(outDir, "style.*.css")); old != nil {
		for _, p := range old {
			if filepath.Base(p) != name {
				_ = os.Remove(p)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(outDir, name), buf.Bytes(), 0o644); err != nil {
		return "", fmt.Errorf("writing css bundle: %w", err)
	}
	return name, nil
}
