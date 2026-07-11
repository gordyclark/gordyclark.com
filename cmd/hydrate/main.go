package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const userAgent = "gordyclark.com-hydrate/1.0 (+https://gordyclark.com)"

// fetchFunc fetches link-preview metadata for a URL.
type fetchFunc func(url string) (title, desc string, err error)

// splice records a byte-range replacement.
type splice struct {
	start, end int
	text       string
}

// processSource performs steps 2-10 with fetching injected, returning the
// (possibly rewritten) source and counts.
func processSource(src []byte, fetch fetchFunc) (out []byte, hydrated, skipped, failed int, failures []string) {
	locs := linkRe.FindAllSubmatchIndex(src, -1)
	var splices []splice

	for _, loc := range locs {
		// loc: [matchStart, matchEnd, g1Start, g1End, g2Start, g2End]
		urlStr := string(src[loc[2]:loc[3]])
		var attrBlock string
		hasAttr := loc[4] >= 0
		if hasAttr {
			attrBlock = string(src[loc[4]:loc[5]])
		}

		// Only margin links (must have an attribute block with .margin).
		if !hasAttr || !hasMarginClass(attrBlock) {
			continue
		}

		attrs := parsedAttrs(attrBlock)
		if attrs["domain"] != "" && attrs["title"] != "" && attrs["desc"] != "" {
			skipped++
			continue
		}

		domain, err := hostname(urlStr)
		if err != nil {
			failed++
			failures = append(failures, fmt.Sprintf("%s: %v", urlStr, err))
			continue
		}

		title, desc, err := fetch(urlStr)
		if err != nil {
			failed++
			failures = append(failures, fmt.Sprintf("%s: %v", urlStr, err))
			continue
		}

		block := buildBlock(attrs, parsedClasses(attrBlock), domain, sanitize(title), sanitize(desc))

		// Replace the {...} region (loc[4]:loc[5]); it always exists here.
		splices = append(splices, splice{start: loc[4], end: loc[5], text: block})
		hydrated++
	}

	if len(splices) == 0 {
		return src, hydrated, skipped, failed, failures
	}

	// Apply in reverse order (rightmost first) to keep offsets valid.
	out = append([]byte(nil), src...)
	for i := len(splices) - 1; i >= 0; i-- {
		s := splices[i]
		var buf bytes.Buffer
		buf.Write(out[:s.start])
		buf.WriteString(s.text)
		buf.Write(out[s.end:])
		out = buf.Bytes()
	}
	return out, hydrated, skipped, failed, failures
}

// hostname returns the URL host with a leading "www." stripped.
func hostname(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	return strings.TrimPrefix(u.Hostname(), "www."), nil
}

// realFetch performs a single GET and extracts metadata.
func realFetch(client *http.Client) fetchFunc {
	return func(rawURL string) (string, string, error) {
		req, err := http.NewRequest(http.MethodGet, rawURL, nil)
		if err != nil {
			return "", "", err
		}
		req.Header.Set("User-Agent", userAgent)
		resp, err := client.Do(req)
		if err != nil {
			return "", "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", "", fmt.Errorf("status %d", resp.StatusCode)
		}
		return extractMeta(resp.Body)
	}
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: hydrate <file.md> [<file2.md> ...]")
		os.Exit(2)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	fetch := realFetch(client)

	anyFailed := false
	for _, path := range args {
		src, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			anyFailed = true
			continue
		}

		out, hydrated, skipped, failed, failures := processSource(src, fetch)

		if failed > 0 {
			anyFailed = true
		}

		if !bytes.Equal(src, out) {
			if err := os.WriteFile(path, out, 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "%s: write: %v\n", path, err)
				anyFailed = true
			}
		}

		fmt.Printf("%s: hydrated %d, skipped %d, failed %d\n", path, hydrated, skipped, failed)
		for _, f := range failures {
			fmt.Printf("  FAIL %s\n", f)
		}
	}

	if anyFailed {
		os.Exit(1)
	}
}
