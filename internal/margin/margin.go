package margin

import (
	"fmt"
	"slices"
	"strings"

	"github.com/yuin/goldmark/ast"

	"github.com/gordyclark/gordyclark.com/internal/content"
)

// MarginKind enumerates the four kinds of item that can appear in the margin
// column. The render layer maps each kind to a template.
type MarginKind int

const (
	// MarginNote is a footnote whose definition body is shown in the margin.
	MarginNote MarginKind = iota
	// MarginCitation is a bibliographic citation resolved from citations.yaml.
	MarginCitation
	// MarginChipExternal is a ".margin" link to an external site.
	MarginChipExternal
	// MarginChipInternal is a ".margin" link to another essay on this site.
	MarginChipInternal
)

// MarginItem is the classified, render-ready description of one margin entry.
// Only the fields relevant to its Kind are populated.
type MarginItem struct {
	Kind MarginKind

	// Note (MarginNote): the footnote label as authored (e.g. "1" or "why"),
	// plus a reference to the footnote definition node so render can render its
	// body. Ref is the corresponding footnote reference in the body text.
	Label   string
	DefNode ast.Node
	RefNode ast.Node

	// Citation (MarginCitation): the cite key and the resolved entry (nil if
	// the key was not found in citations.yaml).
	CiteKey  string
	Citation *content.Citation

	// External chip (MarginChipExternal): populated from the link href and the
	// attribute block's domain/title/desc values.
	URL    string
	Domain string
	Title  string
	Desc   string

	// Internal chip (MarginChipInternal): the raw href plus the slug resolved
	// from it. URL (above) also holds the raw href for this kind.
	TargetSlug string
}

// IsExternal reports whether a link URL points off-site. A URL is external iff
// it begins with an http:// or https:// scheme. Anything else (a rooted "/"
// path, a "./" relative path, or a bare slug with no scheme) is internal.
func IsExternal(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

// SlugFromInternalHref extracts the essay slug from an internal href such as
// "/essays/foo.md", "/essays/foo", "./foo.md", "foo/", or "foo". It strips any
// leading directory component, a trailing slash, and a ".md" extension.
func SlugFromInternalHref(href string) string {
	s := strings.TrimSpace(href)
	// Drop query string / fragment if present.
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimRight(s, "/")
	// Keep only the final path segment.
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	s = strings.TrimSuffix(s, ".md")
	return s
}

// ValidateExternalChip validates the attributes of a ".margin" link. If the
// link carries the "margin" class and is external, it must supply non-empty
// domain, title and desc values. Non-margin links, and internal margin links,
// impose no requirement. The returned error is a bare message; the render
// layer prepends the source file:line and formats it.
func ValidateExternalChip(classes []string, kv map[string]string) error {
	if !hasClass(classes, "margin") {
		return nil
	}
	url := kv["url"]
	// The href itself is not in kv; callers validate external-ness separately
	// by also passing an explicit check. We treat a chip as external-requiring
	// when a domain is expected: require the trio whenever this is a margin
	// chip and any of domain/title/desc is intended. To keep the contract
	// simple and match the spec, require all three for every external margin
	// chip; internal margin chips call this only when external.
	_ = url
	var missing []string
	for _, k := range []string{"domain", "title", "desc"} {
		if strings.TrimSpace(kv[k]) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("external .margin link is missing required attribute(s): %s", strings.Join(missing, ", "))
	}
	return nil
}

// DomainInitial returns the first letter of a domain, uppercased, for use as a
// chip badge. It returns "?" when the domain is empty.
func DomainInitial(domain string) string {
	d := strings.TrimSpace(domain)
	if d == "" {
		return "?"
	}
	return strings.ToUpper(d[:1])
}

func hasClass(classes []string, want string) bool {
	return slices.Contains(classes, want)
}
