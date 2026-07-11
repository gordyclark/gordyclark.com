package render

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/gordyclark/gordyclark.com/internal/content"
	"github.com/gordyclark/gordyclark.com/internal/diagrams"
	"github.com/gordyclark/gordyclark.com/internal/highlight"
	"github.com/gordyclark/gordyclark.com/internal/margin"
)

// essayMeta is the render-time metadata computed for one essay.
type essayMeta struct {
	Slug        string
	Title       string
	Subtitle    string
	DateRaw     string
	Tags        []string
	Status      content.Status
	ReadingTime int
	Related     []relatedEssay
}

// relatedEssay is one entry in the essay's "Related on this site" block.
type relatedEssay struct {
	Slug     string
	Title    string
	Subtitle string
}

// ---------------------------------------------------------------------------
// Footnote suppression
//
// goldmark's footnote extension normally (a) appends a FootnoteList block at
// the end of the document rendering the note bodies, and (b) renders each inline
// reference as a superscript <a> whose href points into that list. Because we
// relocate the note/citation CONTENT into the margin column, we must suppress
// both behaviours or the page would carry a broken/duplicate footnote list with
// dangling backlinks.
//
// Approach: we register our OWN node renderers for FootnoteLink and FootnoteList
// on the goldmark instance, overriding the extension's (renderer options added
// later win — see fnRefRenderer priority below). The FootnoteLink renderer emits
// a small, self-contained superscript marker (<sup class="fn-ref">N</sup>) with
// no backlink, and the FootnoteList / Footnote / FootnoteBacklink renderers emit
// nothing at all. This keeps a lightweight inline marker in the content column
// while the full note body lives in the margin, and guarantees no footnote list
// or dangling anchor survives at the bottom of the document.
// ---------------------------------------------------------------------------

type footnoteSuppressor struct{}

func (footnoteSuppressor) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	// Inline reference: emit a plain superscript index marker, no link.
	reg.Register(extast.KindFootnoteLink, func(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			fl := n.(*extast.FootnoteLink)
			fmt.Fprintf(w, `<sup class="fn-ref">%d</sup>`, fl.Index)
		}
		return ast.WalkContinue, nil
	})
	// Trailing list and its parts: render nothing (content moved to margin).
	noop := func(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
		return ast.WalkSkipChildren, nil
	}
	reg.Register(extast.KindFootnoteList, noop)
	reg.Register(extast.KindFootnote, noop)
	reg.Register(extast.KindFootnoteBacklink, noop)
}

// footnoteSuppressExtender registers footnoteSuppressor at a high priority so it
// wins over the footnote extension's default renderers.
type footnoteSuppressExtender struct{}

func (footnoteSuppressExtender) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(footnoteSuppressor{}, 1),
		),
	)
}

// newMarkdown builds the goldmark instance used for essay bodies.
func newMarkdown() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			extension.Footnote,
			extension.NewTypographer(),
			margin.AttributeExtension(),
			footnoteSuppressExtender{},
		),
		goldmark.WithParserOptions(
			// Enables "{#id .class key=val}" attribute blocks on headings and
			// fenced-code blocks (goldmark's built-in block attribute support).
			parser.WithAttribute(),
		),
		goldmark.WithRendererOptions(
			// Let raw HTML blocks and inline SVG (timeline/dialogue markup) pass
			// through instead of being stripped in safe mode.
			html.WithUnsafe(),
		),
	)
}

// lineOf returns the 1-based line number in the essay's SOURCE FILE for a given
// byte offset into the body, using the essay's BodyOffset.
func lineOf(essay *content.Essay, bodyByteOffset int) int {
	// Count newlines in body[:offset]; body line 1 corresponds to file line
	// BodyOffset.
	if bodyByteOffset < 0 {
		bodyByteOffset = 0
	}
	if bodyByteOffset > len(essay.Body) {
		bodyByteOffset = len(essay.Body)
	}
	nl := bytes.Count(essay.Body[:bodyByteOffset], []byte("\n"))
	return essay.BodyOffset + nl
}

// blockStartLine returns the source-file line number of a block node's first
// line, or the body offset if it cannot be determined.
func blockStartLine(essay *content.Essay, n ast.Node) int {
	if n.Type() == ast.TypeBlock {
		if lines := n.Lines(); lines != nil && lines.Len() > 0 {
			return lineOf(essay, lines.At(0).Start)
		}
	}
	// Fall back to a descendant with line info.
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if l := c.Lines(); l != nil && l.Len() > 0 {
			return lineOf(essay, l.At(0).Start)
		}
	}
	return essay.BodyOffset
}

// fencedSource concatenates the literal source lines of a fenced code block.
func fencedSource(n *ast.FencedCodeBlock, source []byte) string {
	var b strings.Builder
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		b.Write(seg.Value(source))
	}
	return b.String()
}

// renderEssay renders one essay to its article-grid HTML blob and computes its
// metadata. cites is the resolved citation map; ix is the site index used to
// resolve internal chips and related essays; dr renders d2 diagrams.
// citeRefRe matches a citation footnote reference `[^cite:<key>]` in the body.
var citeRefRe = regexp.MustCompile(`\[\^cite:([^\]]+)\]`)

// citeDefRe matches an authored citation footnote DEFINITION line
// `[^cite:<key>]:` so we never inject a duplicate for one that already exists.
var citeDefRe = regexp.MustCompile(`(?m)^\[\^cite:([^\]]+)\]:`)

// injectCiteDefinitions appends a synthetic footnote definition for every
// `[^cite:<key>]` reference that lacks an authored definition, so goldmark's
// footnote extension recognises the reference and emits a FootnoteLink node.
// Definitions are appended at the END of the body, so byte offsets and line
// numbers of the original references are unchanged (keeping validation error
// line numbers accurate). The placeholder body is never rendered.
func injectCiteDefinitions(body []byte) []byte {
	refs := citeRefRe.FindAllSubmatch(body, -1)
	if len(refs) == 0 {
		return body
	}
	defined := map[string]bool{}
	for _, m := range citeDefRe.FindAllSubmatch(body, -1) {
		defined[string(m[1])] = true
	}
	need := map[string]bool{}
	for _, m := range refs {
		key := string(m[1])
		if !defined[key] {
			need[key] = true
		}
	}
	if len(need) == 0 {
		return body
	}
	keys := make([]string, 0, len(need))
	for k := range need {
		keys = append(keys, k)
	}
	sort.Strings(keys) // deterministic output order

	var buf bytes.Buffer
	buf.Write(body)
	if !bytes.HasSuffix(body, []byte("\n")) {
		buf.WriteByte('\n')
	}
	buf.WriteByte('\n')
	for _, k := range keys {
		// The body text is a placeholder; the margin renderer resolves the real
		// citation from citations.yaml by the cite key.
		fmt.Fprintf(&buf, "[^cite:%s]: (citation)\n", k)
	}
	return buf.Bytes()
}

func renderEssay(essay *content.Essay, ix *content.Index, cites map[string]content.Citation, dr *diagrams.Renderer) (template.HTML, essayMeta, error) {
	// Citation footnotes (labels of the form `cite:<key>`) are authored WITHOUT
	// a `[^cite:<key>]: ...` definition line — the definition is resolved from
	// citations.yaml at render time (spec §2.2). goldmark's footnote extension,
	// however, only emits a FootnoteLink node when a matching definition exists;
	// an undefined `[^cite:foo]` reference is left as literal text. So we inject
	// a synthetic definition for every cite reference that lacks one, before
	// parsing. The synthetic body is a placeholder and is never shown — the
	// margin renderer routes `cite:` labels to a citation card built from
	// citations.yaml, not from the footnote body.
	source := injectCiteDefinitions(essay.Body)
	md := newMarkdown()
	doc := md.Parser().Parse(text.NewReader(source))

	// ---- Pre-collect footnote definitions by index. -----------------------
	// The footnote AST transformer moves definitions into a trailing
	// FootnoteList; each *Footnote carries its Ref (authored label) and Index.
	// FootnoteLink references carry the matching Index.
	defByIndex := map[int]*extast.Footnote{}
	labelByIndex := map[int]string{}
	var fnList *extast.FootnoteList
	for c := doc.FirstChild(); c != nil; c = c.NextSibling() {
		if fl, ok := c.(*extast.FootnoteList); ok {
			fnList = fl
			for f := fl.FirstChild(); f != nil; f = f.NextSibling() {
				if fn, ok := f.(*extast.Footnote); ok {
					defByIndex[fn.Index] = fn
					labelByIndex[fn.Index] = string(fn.Ref)
				}
			}
		}
	}

	// Pre-render each footnote definition body to HTML (keyed by def node) so
	// MarginNote items can carry their body. We render the Footnote's block
	// children, skipping the trailing FootnoteBacklink the extension appended.
	noteBodies := map[any]template.HTML{}
	for _, fn := range defByIndex {
		htmlBody, err := renderFootnoteBody(md, source, fn)
		if err != nil {
			return "", essayMeta{}, fmt.Errorf("%s: rendering footnote %q: %w", essay.SourcePath, string(fn.Ref), err)
		}
		noteBodies[ast.Node(fn)] = htmlBody
	}

	// Detach the footnote list so it is not walked as a top-level block. (Our
	// suppressor would render it to nothing anyway, but detaching keeps the
	// top-level block walk clean.)
	if fnList != nil {
		doc.RemoveChild(doc, fnList)
	}

	// ---- Walk top-level blocks in document order. --------------------------
	var pairs bytes.Buffer
	first := true
	meta := metaForEssay(essay, ix)

	diagramCount := 0
	for block := doc.FirstChild(); block != nil; block = block.NextSibling() {
		contentHTML, err := renderBlockContent(md, source, block, essay, dr, &diagramCount)
		if err != nil {
			return "", essayMeta{}, err
		}

		items, err := collectMarginItems(block, essay, ix, cites, labelByIndex, defByIndex)
		if err != nil {
			return "", essayMeta{}, err
		}

		var marginBuf bytes.Buffer
		if first {
			card, err := renderMetaCard(metaCardData{
				Title:       meta.Title,
				Date:        meta.DateRaw,
				ReadingTime: meta.ReadingTime,
				Tags:        meta.Tags,
			})
			if err != nil {
				return "", essayMeta{}, err
			}
			marginBuf.WriteString(string(card))
			first = false
		}
		for _, it := range items {
			h, err := renderMarginItem(it, noteBodies)
			if err != nil {
				return "", essayMeta{}, err
			}
			marginBuf.WriteString(string(h))
		}

		pairs.WriteString(`<div class="content-cell">`)
		pairs.WriteString(string(contentHTML))
		pairs.WriteString(`</div>`)
		pairs.WriteString(`<div class="margin-cell">`)
		pairs.WriteString(marginBuf.String())
		pairs.WriteString(`</div>`)
	}

	// If the document had no top-level blocks (empty body), still emit one
	// empty pair carrying the metadata card so the page is well formed.
	if first {
		card, err := renderMetaCard(metaCardData{
			Title:       meta.Title,
			Date:        meta.DateRaw,
			ReadingTime: meta.ReadingTime,
			Tags:        meta.Tags,
		})
		if err != nil {
			return "", essayMeta{}, err
		}
		pairs.WriteString(`<div class="content-cell"></div>`)
		pairs.WriteString(`<div class="margin-cell">`)
		pairs.WriteString(string(card))
		pairs.WriteString(`</div>`)
	}

	article := `<div class="article-grid">` + pairs.String() + `</div>`
	return template.HTML(article), meta, nil //nolint:gosec // composed from trusted renderers
}

// renderFootnoteBody renders a footnote definition's block children to HTML,
// skipping the FootnoteBacklink node the extension appends.
func renderFootnoteBody(md goldmark.Markdown, source []byte, fn *extast.Footnote) (template.HTML, error) {
	var buf bytes.Buffer
	for c := fn.FirstChild(); c != nil; c = c.NextSibling() {
		// Skip the backlink paragraph tail the extension adds inside the last
		// child; our suppressor renders FootnoteBacklink to nothing, so we can
		// safely render the child subtree as-is.
		if err := md.Renderer().Render(&buf, source, c); err != nil {
			return "", err
		}
	}
	return template.HTML(buf.String()), nil //nolint:gosec // trusted renderer output
}

// renderBlockContent renders a single top-level block to its content-cell HTML,
// applying the two special substitutions for fenced code blocks (d2 diagrams and
// syntax-highlighted code).
func renderBlockContent(md goldmark.Markdown, source []byte, block ast.Node, essay *content.Essay, dr *diagrams.Renderer, diagramN *int) (template.HTML, error) {
	if fcb, ok := block.(*ast.FencedCodeBlock); ok {
		lang := string(fcb.Language(source))
		src := fencedSource(fcb, source)
		switch {
		case lang == "d2":
			svg, err := dr.Render(src)
			if err != nil {
				return "", fmt.Errorf("%s:%d: d2 diagram render failed: %w", essay.SourcePath, blockStartLine(essay, block), err)
			}
			*diagramN++
			return wrapDiagram(svg, *diagramN), nil
		case lang != "":
			hl, err := highlight.Highlight(src, lang)
			if err != nil {
				return "", fmt.Errorf("%s:%d: highlighting %q code: %w", essay.SourcePath, blockStartLine(essay, block), lang, err)
			}
			return hl, nil
		}
		// Tagless fenced block: fall through to normal goldmark rendering.
	}

	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, source, block); err != nil {
		return "", fmt.Errorf("%s: rendering block: %w", essay.SourcePath, err)
	}
	return template.HTML(buf.String()), nil //nolint:gosec // trusted renderer output
}

// svgIDRefRe matches D2's internal SVG id definitions and references so we can
// namespace them per copy. D2 emits ids like `d2-319849221`,
// `streaks-bright-d2-319849221`, `mk-...`, referenced via url(#id),
// xlink:href="#id", and href="#id".
var svgIDRefRe = regexp.MustCompile(`(id=")([^"]+)(")|(url\(#)([^)]+)(\))|((?:xlink:)?href="#)([^"]+)(")`)

// namespaceSVGIDs rewrites every internal id definition and reference in an SVG
// by prefixing it, so two copies of the same diagram (thumbnail + modal) can
// coexist on one page without id collisions (which would otherwise make
// url(#...) references resolve to the wrong copy's gradients and markers).
func namespaceSVGIDs(svg, prefix string) string {
	return svgIDRefRe.ReplaceAllStringFunc(svg, func(m string) string {
		g := svgIDRefRe.FindStringSubmatch(m)
		switch {
		case g[1] != "": // id="..."
			return g[1] + prefix + g[2] + g[3]
		case g[4] != "": // url(#...)
			return g[4] + prefix + g[5] + g[6]
		default: // href="#..."
			return g[7] + prefix + g[8] + g[9]
		}
	})
}

// wrapDiagram wraps an inlined D2 SVG as a small, clickable thumbnail plus a
// CSS-only (:target) modal overlay holding the full-size copy — no JavaScript.
// The thumbnail links to the modal's fragment id; the modal is revealed by its
// :target rule (see components/figure.css) and dismissed by a full-bleed
// backdrop link and a close control that both navigate back to "#". The modal's
// SVG ids are namespaced so its gradients/markers don't collide with the
// thumbnail's identical copy.
func wrapDiagram(svg string, n int) template.HTML {
	id := fmt.Sprintf("dia-%d", n)
	modalSVG := namespaceSVGIDs(svg, fmt.Sprintf("m%d-", n))
	var b strings.Builder
	fmt.Fprintf(&b, `<figure class="diagram">`)
	fmt.Fprintf(&b, `<a class="diagram-thumb" href="#%s" aria-label="Enlarge diagram">%s</a>`, id, svg)
	fmt.Fprintf(&b, `<figcaption class="diagram-hint">Click to enlarge</figcaption>`)
	fmt.Fprintf(&b, `</figure>`)
	// Modal overlay. The backdrop link and the close button both go to "#".
	fmt.Fprintf(&b, `<div class="diagram-modal" id="%s" role="dialog" aria-label="Enlarged diagram">`, id)
	fmt.Fprintf(&b, `<a class="diagram-modal-backdrop" href="#" aria-label="Close"></a>`)
	fmt.Fprintf(&b, `<div class="diagram-modal-body">%s<a class="diagram-modal-close" href="#" aria-label="Close">×</a></div>`, modalSVG)
	fmt.Fprintf(&b, `</div>`)
	return template.HTML(b.String()) //nolint:gosec // diagram SVG is trusted
}

// collectMarginItems scans a block's inline descendants in document order for
// footnote references (notes / citations) and ".margin" links (internal /
// external chips), returning the classified items in order.
func collectMarginItems(block ast.Node, essay *content.Essay, ix *content.Index, cites map[string]content.Citation, labelByIndex map[int]string, defByIndex map[int]*extast.Footnote) ([]margin.MarginItem, error) {
	var items []margin.MarginItem
	var walkErr error

	err := ast.Walk(block, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *extast.FootnoteLink:
			label := labelByIndex[node.Index]
			def := defByIndex[node.Index]
			line := nodeLine(essay, n, block)
			if key, isCite := strings.CutPrefix(label, "cite:"); isCite {
				c, ok := cites[key]
				if !ok {
					walkErr = fmt.Errorf("%s:%d: unknown citation key %q", essay.SourcePath, line, key)
					return ast.WalkStop, nil
				}
				cc := c
				items = append(items, margin.MarginItem{
					Kind:     margin.MarginCitation,
					Label:    label,
					CiteKey:  key,
					Citation: &cc,
					RefNode:  n,
					DefNode:  def,
				})
			} else {
				items = append(items, margin.MarginItem{
					Kind:    margin.MarginNote,
					Label:   label,
					RefNode: n,
					DefNode: def,
				})
			}
		case *ast.Link:
			classes, kv, ok := margin.LinkAttrs(node)
			if !ok || !hasClass(classes, "margin") {
				return ast.WalkContinue, nil
			}
			href := string(node.Destination)
			line := nodeLine(essay, n, block)
			if margin.IsExternal(href) {
				if err := margin.ValidateExternalChip(classes, kv); err != nil {
					walkErr = unhydratedLinkError(essay, line, linkText(node, essay.Body), href)
					return ast.WalkStop, nil
				}
				items = append(items, margin.MarginItem{
					Kind:   margin.MarginChipExternal,
					URL:    href,
					Domain: kv["domain"],
					Title:  kv["title"],
					Desc:   kv["desc"],
				})
			} else {
				slug := margin.SlugFromInternalHref(href)
				entry := ix.Get(slug)
				if entry == nil {
					walkErr = fmt.Errorf("%s:%d: internal .margin link references unknown essay slug %q (href %q)", essay.SourcePath, line, slug, href)
					return ast.WalkStop, nil
				}
				items = append(items, margin.MarginItem{
					Kind:       margin.MarginChipInternal,
					URL:        href,
					TargetSlug: slug,
					Title:      entry.Title,
					Desc:       entry.Subtitle,
				})
			}
		}
		return ast.WalkContinue, nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	if err != nil {
		return nil, err
	}
	return items, nil
}

// hasClass reports whether classes contains want.
func hasClass(classes []string, want string) bool {
	return slices.Contains(classes, want)
}

// linkText returns the plain text of a link's children for error messages.
func linkText(link *ast.Link, source []byte) string {
	var b strings.Builder
	for c := link.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			b.Write(t.Segment.Value(source))
		}
	}
	return b.String()
}

// nodeLine returns the best-effort source-file line for an inline node by
// locating the nearest ancestor block with line information.
func nodeLine(essay *content.Essay, n ast.Node, fallbackBlock ast.Node) int {
	for a := n; a != nil; a = a.Parent() {
		if a.Type() == ast.TypeBlock {
			if l := a.Lines(); l != nil && l.Len() > 0 {
				return lineOf(essay, l.At(0).Start)
			}
		}
	}
	return blockStartLine(essay, fallbackBlock)
}

// unhydratedLinkError returns the exact SPEC §2.3 multi-line error for an
// external ".margin" link missing its domain/title/desc attributes.
func unhydratedLinkError(essay *content.Essay, line int, text, url string) error {
	rel := relContentPath(essay.SourcePath)
	return fmt.Errorf(
		"ERROR: unhydrated margin link in %s:%d\n  [%s](%s){.margin}\n  run: cmd/hydrate %s",
		rel, line, text, url, rel,
	)
}

// relContentPath normalises a source path to the "content/essays/<file>.md"
// form used in the SPEC §2.3 message, regardless of absolute prefix.
func relContentPath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	if i := strings.Index(p, "content/"); i >= 0 {
		return p[i:]
	}
	return p
}

// metaForEssay computes reading time, tags and related essays for an essay.
func metaForEssay(essay *content.Essay, ix *content.Index) essayMeta {
	m := essayMeta{
		Slug:        essay.Front.Slug,
		Title:       essay.Front.Title,
		Subtitle:    essay.Front.Subtitle,
		DateRaw:     essay.Front.Date,
		Tags:        essay.Front.Tags,
		Status:      essay.Front.Status,
		ReadingTime: readingTime(essay),
		Related:     relatedEssays(essay, ix),
	}
	return m
}

// readingTime returns the reading time in minutes: the frontmatter override if
// set, else word_count/230 rounded to the nearest integer, with a floor of 1.
func readingTime(essay *content.Essay) int {
	if essay.Front.ReadingTimeOverride != nil {
		return *essay.Front.ReadingTimeOverride
	}
	words := len(strings.Fields(string(essay.Body)))
	// Round to nearest: (words + 115) / 230 integer division. Minimum 1.
	return max((words+115)/230, 1)
}

// relatedEssays returns up to 3 finished essays (newest first) that share at
// least one tag with this essay, excluding the essay itself.
func relatedEssays(essay *content.Essay, ix *content.Index) []relatedEssay {
	if ix == nil {
		return nil
	}
	tagSet := map[string]bool{}
	for _, t := range essay.Front.Tags {
		tagSet[t] = true
	}
	if len(tagSet) == 0 {
		return nil
	}
	var out []relatedEssay
	for _, e := range ix.Ordered { // already newest-first
		if e.Slug == essay.Front.Slug {
			continue
		}
		if e.Status != content.StatusFinished {
			continue
		}
		shared := false
		for _, t := range e.Tags {
			if tagSet[t] {
				shared = true
				break
			}
		}
		if !shared {
			continue
		}
		out = append(out, relatedEssay{Slug: e.Slug, Title: e.Title, Subtitle: e.Subtitle})
		if len(out) == 3 {
			break
		}
	}
	return out
}
