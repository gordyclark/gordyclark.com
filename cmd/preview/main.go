// Command preview serves the built ./static tree over HTTP for local review.
//
// It exists so previewing the site needs nothing but the Go toolchain, and so
// local behaviour matches how Cloudflare serves the deployed Worker. The
// standard library's http.FileServer is close but not identical: it 301s
// "/foo/index.html" to "/foo/" yet serves "/foo" (no slash) by redirecting to
// "/foo/", and it has no notion of the site's 404 page. This server mirrors the
// Worker's assets configuration instead:
//
//   - html_handling "auto-trailing-slash": a directory URL is served from its
//     index.html, and an explicit ".html" path redirects to the clean URL.
//   - not_found_handling "404-page": an unmatched path returns the site's own
//     404.html with a 404 status, rather than a bare Go error page.
//
// It also applies the Cache-Control rules from the generated _headers file, so
// caching behaviour is visible locally too.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func main() {
	dir := flag.String("dir", "./static", "directory to serve as the web root")
	addr := flag.String("addr", ":8000", "address to listen on")
	flag.Parse()

	root, err := filepath.Abs(*dir)
	if err != nil {
		log.Fatalf("preview: resolving %s: %v", *dir, err)
	}
	if _, err := os.Stat(filepath.Join(root, "index.html")); err != nil {
		log.Fatalf("preview: %s has no index.html — run 'just build' first", root)
	}

	srv := &server{root: root, headers: loadHeaders(root)}

	host := *addr
	if strings.HasPrefix(host, ":") {
		host = "localhost" + host
	}
	fmt.Printf("Serving %s at http://%s  (Ctrl-C to stop)\n", root, host)
	if err := http.ListenAndServe(*addr, srv); err != nil {
		log.Fatalf("preview: %v", err)
	}
}

type server struct {
	root    string
	headers []headerRule
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// path.Clean strips a trailing slash, but the slash is significant here:
	// the _headers rules distinguish page URLs (which end in "/") from asset
	// URLs (which do not), so it is restored after cleaning.
	hadSlash := strings.HasSuffix(r.URL.Path, "/")
	upath := path.Clean("/" + r.URL.Path)
	if hadSlash && !strings.HasSuffix(upath, "/") {
		upath += "/"
	}

	// Mirror "auto-trailing-slash": an explicit .html URL redirects to its
	// clean form, so the canonical URL is the one the site links to.
	if strings.HasSuffix(upath, ".html") {
		clean := strings.TrimSuffix(upath, ".html")
		if strings.HasSuffix(clean, "/index") {
			clean = strings.TrimSuffix(clean, "index")
		}
		if clean == "" {
			clean = "/"
		}
		http.Redirect(w, r, clean, http.StatusTemporaryRedirect)
		return
	}

	if file, ok := s.resolve(upath); ok {
		s.applyHeaders(w, upath)
		http.ServeFile(w, r, file)
		return
	}

	// not_found_handling "404-page".
	s.serveNotFound(w, r)
}

// resolve maps a request path to a file on disk, trying the path itself and
// then its index.html, and reports whether one was found.
func (s *server) resolve(upath string) (string, bool) {
	trimmed := strings.TrimSuffix(upath, "/")
	if trimmed == "" {
		trimmed = "/"
	}
	candidates := []string{
		filepath.Join(s.root, filepath.FromSlash(trimmed)),
		filepath.Join(s.root, filepath.FromSlash(trimmed), "index.html"),
		filepath.Join(s.root, filepath.FromSlash(trimmed)+".html"),
	}
	for _, c := range candidates {
		// Refuse anything that escapes the root, e.g. via "..".
		if !strings.HasPrefix(c, s.root) {
			continue
		}
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c, true
		}
	}
	return "", false
}

func (s *server) serveNotFound(w http.ResponseWriter, r *http.Request) {
	page := filepath.Join(s.root, "404.html")
	body, err := os.ReadFile(page)
	if err != nil {
		http.Error(w, "404 page not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write(body)
}

// headerRule is one path pattern from _headers and the headers it sets.
type headerRule struct {
	pattern string
	headers [][2]string
}

// loadHeaders parses the generated _headers file. Only the subset the site
// actually uses is supported: a path pattern on its own line, followed by
// indented "Name: value" lines. An absent or malformed file is not fatal —
// preview is a convenience, not a validator.
func loadHeaders(root string) []headerRule {
	data, err := os.ReadFile(filepath.Join(root, "_headers"))
	if err != nil {
		return nil
	}
	var rules []headerRule
	var cur *headerRule
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if line == trimmed { // unindented: a new path pattern
			rules = append(rules, headerRule{pattern: trimmed})
			cur = &rules[len(rules)-1]
			continue
		}
		if cur == nil {
			continue
		}
		name, value, found := strings.Cut(trimmed, ":")
		if !found {
			continue
		}
		cur.headers = append(cur.headers, [2]string{
			strings.TrimSpace(name), strings.TrimSpace(value),
		})
	}
	return rules
}

// applyHeaders writes the headers of the most specific matching rule. Cloudflare
// joins the headers of every matching rule with a comma instead; the generated
// _headers file keeps its rules mutually exclusive so the two agree, and
// preferring the longest pattern here keeps that true even if a rule is added
// that overlaps another.
func (s *server) applyHeaders(w http.ResponseWriter, upath string) {
	best := -1
	for i, rule := range s.headers {
		if !matchPattern(rule.pattern, upath) {
			continue
		}
		if best < 0 || len(rule.pattern) > len(s.headers[best].pattern) {
			best = i
		}
	}
	if best < 0 {
		return
	}
	for _, h := range s.headers[best].headers {
		w.Header().Set(h[0], h[1])
	}
}

// matchPattern supports the "*" wildcard used in the generated _headers file.
// A "*" matches any run of characters including "/", so "/*/" matches a page
// URL at any depth, matching Cloudflare's splat behaviour.
func matchPattern(pattern, upath string) bool {
	if !strings.Contains(pattern, "*") {
		return pattern == upath
	}
	prefix, suffix, _ := strings.Cut(pattern, "*")
	if !strings.HasPrefix(upath, prefix) || !strings.HasSuffix(upath, suffix) {
		return false
	}
	// The prefix and suffix must not overlap, or "/a/" would match "/*/" twice
	// over the same characters.
	return len(upath) >= len(prefix)+len(suffix)
}
