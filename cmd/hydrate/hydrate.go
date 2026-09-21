package main

import (
	"io"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/net/html"

	"github.com/gordyclark/gordyclark.com/internal/margin"
)

// linkRe matches an external markdown link with an OPTIONAL trailing {...} block.
// Group 1 = URL, group 2 = the attribute block (empty string if absent).
var linkRe = regexp.MustCompile(`\[[^\]]*\]\((https?://[^)\s]+)\)(\{[^}]*\})?`)

// wsRe matches runs of whitespace (including newlines).
var wsRe = regexp.MustCompile(`\s+`)

// extractMeta walks an HTML document and returns its <title> text and a
// description (og:description preferred, else name=description).
func extractMeta(r io.Reader) (title, desc string, err error) {
	doc, err := html.Parse(r)
	if err != nil {
		return "", "", err
	}
	var ogDesc, nameDesc string
	var inTitle bool
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "title":
				inTitle = true
				defer func() { inTitle = false }()
			case "meta":
				var prop, name, content string
				for _, a := range n.Attr {
					switch strings.ToLower(a.Key) {
					case "property":
						prop = strings.ToLower(a.Val)
					case "name":
						name = strings.ToLower(a.Val)
					case "content":
						content = a.Val
					}
				}
				if prop == "og:description" && ogDesc == "" {
					ogDesc = content
				}
				if name == "description" && nameDesc == "" {
					nameDesc = content
				}
			}
		}
		if n.Type == html.TextNode && inTitle && title == "" {
			title = n.Data
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	desc = ogDesc
	if desc == "" {
		desc = nameDesc
	}
	return title, desc, nil
}

// The three readers below delegate to margin.ParseAttrs, the same parser the
// renderer uses. They used to scan the attribute block with regexps, which read
// the dots inside an already-written value: on a second run, a block containing
// domain="en.wikipedia.org" yielded the classes .margin .wikipedia .org, so
// hydrating a hydrated file corrupted it. Sharing the parser makes hydrate and
// the build agree on what a block means, by construction.
//
// Each takes the block WITH its braces, as linkRe captures it.

// attrBlockBody strips the surrounding braces from a captured attribute block.
func attrBlockBody(attrBlock string) []byte {
	b := strings.TrimSpace(attrBlock)
	b = strings.TrimPrefix(b, "{")
	b = strings.TrimSuffix(b, "}")
	return []byte(b)
}

// hasMarginClass reports whether the attribute block contains the .margin class.
func hasMarginClass(attrBlock string) bool {
	classes, _ := margin.ParseAttrs(attrBlockBody(attrBlock))
	for _, c := range classes {
		if c == "margin" {
			return true
		}
	}
	return false
}

// parsedAttrs pulls key="value" pairs from an attribute block.
func parsedAttrs(attrBlock string) map[string]string {
	_, kv := margin.ParseAttrs(attrBlockBody(attrBlock))
	return kv
}

// parsedClasses pulls .class tokens (without the dot) from an attribute block.
func parsedClasses(attrBlock string) []string {
	classes, _ := margin.ParseAttrs(attrBlockBody(attrBlock))
	return classes
}

// sanitize collapses whitespace runs to single spaces, trims, and replaces
// double quotes with single quotes so the value cannot break the attribute.
func sanitize(s string) string {
	s = wsRe.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	return strings.ReplaceAll(s, `"`, `'`)
}

// buildBlock builds an attribute block preserving existing classes (ensuring
// .margin is present) and existing non-hydration keys, then setting the
// domain/title/desc trio.
func buildBlock(existing map[string]string, existingClasses []string, domain, title, desc string) string {
	// Classes: keep order, ensure margin present exactly once.
	classes := make([]string, 0, len(existingClasses)+1)
	seen := map[string]bool{}
	hasMargin := false
	for _, c := range existingClasses {
		if seen[c] {
			continue
		}
		seen[c] = true
		if c == "margin" {
			hasMargin = true
		}
		classes = append(classes, c)
	}
	if !hasMargin {
		classes = append([]string{"margin"}, classes...)
	}

	var parts []string
	for _, c := range classes {
		parts = append(parts, "."+c)
	}

	// Preserve extra keys (deterministic order), excluding the trio.
	var extraKeys []string
	for k := range existing {
		switch k {
		case "domain", "title", "desc":
			continue
		}
		extraKeys = append(extraKeys, k)
	}
	sort.Strings(extraKeys)
	for _, k := range extraKeys {
		parts = append(parts, k+`="`+existing[k]+`"`)
	}

	parts = append(parts, `domain="`+domain+`"`)
	parts = append(parts, `title="`+title+`"`)
	parts = append(parts, `desc="`+desc+`"`)

	return "{" + strings.Join(parts, " ") + "}"
}
