package margin

import (
	"net/url"
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
	// TargetURL is the resolved absolute path of an internal chip's target,
	// e.g. "/lists/seven-things/". It is resolved from the content index so a
	// chip can point at any content type, not just essays.
	TargetURL string
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

// Hostname returns a URL's host with a leading "www." stripped, or "" if the
// URL does not parse or carries no host. Both the renderer and cmd/hydrate
// derive a chip's domain this way, so a hydrated file and a bare one agree.
func Hostname(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(u.Hostname(), "www.")
}

// ChipDomain returns the domain to show on a chip: the authored value when it
// is present, otherwise one derived from the href.
//
// A margin link no longer has to be hydrated to build, so the renderer cannot
// assume the attribute block supplies anything. The href is the one thing
// always present, which makes it the reliable fallback.
func ChipDomain(authored, href string) string {
	if d := strings.TrimSpace(authored); d != "" {
		return d
	}
	return Hostname(href)
}

// ChipTitle returns the title to show on a chip. An authored title wins; with
// none, the URL's fragment or last path segment is un-slugified into something
// readable, and a URL with neither falls back to the domain.
//
// A chip with no title at all renders as an unlabelled box, so this never
// returns empty for a URL that has a host.
func ChipTitle(authored, href string) string {
	if t := strings.TrimSpace(authored); t != "" {
		return t
	}

	raw := strings.TrimSpace(href)
	u, err := url.Parse(raw)
	if err != nil {
		return Hostname(raw)
	}

	// A fragment names a section, which is more specific than the page it
	// sits in, so it is preferred when present.
	if seg := unslug(u.Fragment); seg != "" {
		return seg
	}
	path := strings.Trim(u.Path, "/")
	if i := strings.LastIndex(path, "/"); i >= 0 {
		path = path[i+1:]
	}
	if seg := unslug(path); seg != "" {
		return seg
	}
	return Hostname(raw)
}

// unslug turns a URL segment into prose: percent-decoded, underscores and
// hyphens become spaces, any file extension is dropped, and whitespace is
// collapsed. It returns "" when nothing readable is left.
func unslug(seg string) string {
	s := strings.TrimSpace(seg)
	if s == "" {
		return ""
	}
	if dec, err := url.PathUnescape(s); err == nil {
		s = dec
	}
	// Drop a trailing extension (".html", ".pdf") but not a dot inside a name.
	if i := strings.LastIndex(s, "."); i > 0 && len(s)-i <= 6 && !strings.Contains(s[i+1:], " ") {
		s = s[:i]
	}
	s = strings.NewReplacer("_", " ", "-", " ", "+", " ").Replace(s)
	return strings.Join(strings.Fields(s), " ")
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
