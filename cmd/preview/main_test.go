package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// newTestServer builds a small static tree and returns a server over it.
func newTestServer(t *testing.T) *server {
	t.Helper()
	root := t.TempDir()

	files := map[string]string{
		"index.html":             "<h1>home</h1>",
		"404.html":               "<h1>not found</h1>",
		"lists/index.html":       "<h1>lists</h1>",
		"lists/books/index.html": "<h1>books</h1>",
		"style.abc123.css":       "body{}",
		"img/a.png":              "notreallyapng",
		"_headers": `# comment
/style.*.css
  Cache-Control: public, max-age=31536000, immutable

/img/*
  Cache-Control: public, max-age=604800

/*/
  Cache-Control: public, max-age=60
/
  Cache-Control: public, max-age=60
`,
	}
	for name, body := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return &server{root: root, headers: loadHeaders(root)}
}

func get(t *testing.T, s *server, path string) *http.Response {
	t.Helper()
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec.Result()
}

func TestServesDirectoryIndexes(t *testing.T) {
	s := newTestServer(t)
	for _, path := range []string{"/", "/lists/", "/lists/books/"} {
		resp := get(t, s, path)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", path, resp.StatusCode)
		}
	}
}

// An unmatched path must return the site's own 404 page with a 404 status,
// never the homepage — that would be the SPA behaviour the Worker disables.
func TestUnknownPathServes404Page(t *testing.T) {
	s := newTestServer(t)
	resp := get(t, s, "/no-such-page")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	buf := make([]byte, 64)
	n, _ := resp.Body.Read(buf)
	if got := string(buf[:n]); got != "<h1>not found</h1>" {
		t.Errorf("body = %q, want the 404 page", got)
	}
}

// Mirrors the Worker's auto-trailing-slash handling.
func TestHTMLPathRedirectsToCleanURL(t *testing.T) {
	s := newTestServer(t)
	for path, want := range map[string]string{
		"/index.html":       "/",
		"/lists/index.html": "/lists/",
	} {
		resp := get(t, s, path)
		if resp.StatusCode != http.StatusTemporaryRedirect {
			t.Errorf("GET %s = %d, want 307", path, resp.StatusCode)
		}
		if loc := resp.Header.Get("Location"); loc != want {
			t.Errorf("GET %s redirected to %q, want %q", path, loc, want)
		}
	}
}

// Page URLs keep their trailing slash through path cleaning, or the "/*/"
// header rule silently stops matching them.
func TestCacheControlPerAssetType(t *testing.T) {
	s := newTestServer(t)
	cases := map[string]string{
		"/":                 "public, max-age=60",
		"/lists/":           "public, max-age=60",
		"/lists/books/":     "public, max-age=60",
		"/style.abc123.css": "public, max-age=31536000, immutable",
		"/img/a.png":        "public, max-age=604800",
	}
	for path, want := range cases {
		resp := get(t, s, path)
		if got := resp.Header.Get("Cache-Control"); got != want {
			t.Errorf("GET %s Cache-Control = %q, want %q", path, got, want)
		}
	}
}

// Only one rule may apply, or Cloudflare would join them into a doubled header.
func TestOnlyMostSpecificRuleApplies(t *testing.T) {
	s := newTestServer(t)
	resp := get(t, s, "/style.abc123.css")
	if vals := resp.Header.Values("Cache-Control"); len(vals) != 1 {
		t.Errorf("Cache-Control set %d times, want exactly 1: %v", len(vals), vals)
	}
}

// A path traversal attempt must not escape the served root.
func TestPathTraversalRefused(t *testing.T) {
	s := newTestServer(t)
	resp := get(t, s, "/../../etc/passwd")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("traversal returned %d, want 404", resp.StatusCode)
	}
}

func TestMatchPattern(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"/", "/", true},
		{"/", "/lists/", false},
		{"/*/", "/lists/", true},
		{"/*/", "/lists/books/", true},
		{"/*/", "/style.css", false},
		{"/style.*.css", "/style.abc.css", true},
		{"/style.*.css", "/other.css", false},
		{"/img/*", "/img/a.png", true},
		{"/img/*", "/fonts/a.woff2", false},
	}
	for _, c := range cases {
		if got := matchPattern(c.pattern, c.path); got != c.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}
