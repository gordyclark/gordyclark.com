// Package margin provides the two facilities the render layer needs to build
// the marginalia column: (1) a goldmark inline parser that recognises an
// attribute block "{...}" immediately following a markdown link and attaches
// the parsed classes / key-value pairs to the ast.Link node, and (2) the data
// model + classification helpers for the four kinds of margin items. Rendering
// (templates, HTML) is deliberately NOT done here; that is the render layer's
// job. This package only does detection, classification and attribute parsing.
package margin

import (
	"sync"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Attribute storage choice:
//
// The parsed attributes are stored in a package-level side table (linkAttrs)
// keyed by the *ast.Link pointer, NOT via node.SetAttributeString. This is
// deliberate: goldmark's default HTML renderer serialises anything set with
// SetAttributeString as a real HTML attribute on the <a> tag, which would leak
// our internal metadata (domain/title/desc) into the output. A side table
// keeps the parsed attributes invisible to the renderer while still travelling
// logically with the node for the render layer to read via LinkAttrs.
//
// The side table lives for the duration of a single parse+render pass; entries
// are never removed (documents are short-lived per render call). Access is
// guarded by a mutex because goldmark parsing is single-goroutine per document
// but the render layer may process essays concurrently.
var (
	linkAttrsMu sync.Mutex
	linkAttrs   = map[*ast.Link]*LinkAttributes{}
)

// LinkAttributes is the parsed content of a "{...}" attribute block.
type LinkAttributes struct {
	Classes []string
	KV      map[string]string
}

// linkAttrParser is a goldmark InlineParser triggered by '{'. When the reader
// sits on a '{' and the immediately preceding inline node is an *ast.Link, it
// parses the attribute block, attaches the attributes to that link, and
// consumes the "{...}" bytes so they do not render. In every other case it
// returns nil (declining), leaving the '{' as ordinary text.
type linkAttrParser struct{}

func (linkAttrParser) Trigger() []byte { return []byte{'{'} }

func (linkAttrParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	// The preceding link must be the last child of the current parent.
	last := parent.LastChild()
	link, ok := last.(*ast.Link)
	if !ok || link == nil {
		return nil
	}

	line, _ := block.PeekLine()
	if len(line) == 0 || line[0] != '{' {
		return nil
	}
	// Find the matching closing brace on this line. Attribute blocks do not
	// span lines and do not contain a '}' inside a value in our syntax.
	end := -1
	for i := 1; i < len(line); i++ {
		if line[i] == '}' {
			end = i
			break
		}
	}
	if end < 0 {
		return nil
	}

	classes, kv := parseAttrBlock(line[1:end])
	if len(classes) == 0 && len(kv) == 0 {
		// Nothing useful inside the braces; leave the text alone.
		return nil
	}

	linkAttrsMu.Lock()
	linkAttrs[link] = &LinkAttributes{Classes: classes, KV: kv}
	linkAttrsMu.Unlock()

	// Consume "{...}" (end is index of '}', so end+1 bytes) so it never renders.
	block.Advance(end + 1)
	// Return a zero-width node so goldmark honours the Advance above. Returning
	// nil would tell goldmark we consumed nothing, and the "{...}" bytes would
	// be re-emitted as literal text. The emptyNode renders to nothing.
	return &emptyNode{}
}

// emptyNode is an inline AST node that occupies the parsed "{...}" span but
// renders to no output. It exists solely so the inline parser can signal a
// successful (non-nil) parse while emitting nothing.
type emptyNode struct {
	ast.BaseInline
}

var kindEmpty = ast.NewNodeKind("MarginLinkAttrs")

func (*emptyNode) Kind() ast.NodeKind              { return kindEmpty }
func (n *emptyNode) Dump(source []byte, level int) { ast.DumpHelper(n, source, level, nil, nil) }

// emptyRenderer registers a no-op render function for emptyNode.
type emptyRenderer struct{}

func (emptyRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindEmpty, func(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
		return ast.WalkContinue, nil
	})
}

// parseAttrBlock parses the inside of a "{...}" block: ".class" shorthands
// (one or more, space separated) and key="value" pairs.
func parseAttrBlock(b []byte) (classes []string, kv map[string]string) {
	kv = map[string]string{}
	s := string(b)
	i := 0
	n := len(s)
	for i < n {
		// skip whitespace
		for i < n && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		if i >= n {
			break
		}
		if s[i] == '.' {
			i++
			start := i
			for i < n && s[i] != ' ' && s[i] != '\t' {
				i++
			}
			if i > start {
				classes = append(classes, s[start:i])
			}
			continue
		}
		// key="value" or key=value
		start := i
		for i < n && s[i] != '=' && s[i] != ' ' && s[i] != '\t' {
			i++
		}
		key := s[start:i]
		if i < n && s[i] == '=' {
			i++
			var val string
			if i < n && (s[i] == '"' || s[i] == '\'') {
				q := s[i]
				i++
				vstart := i
				for i < n && s[i] != q {
					i++
				}
				val = s[vstart:i]
				if i < n {
					i++ // consume closing quote
				}
			} else {
				vstart := i
				for i < n && s[i] != ' ' && s[i] != '\t' {
					i++
				}
				val = s[vstart:i]
			}
			if key != "" {
				kv[key] = val
			}
		}
	}
	return classes, kv
}

// LinkAttrs reads the classes and key/value attributes previously attached to a
// link node by the inline parser. ok is false if no attribute block was found.
func LinkAttrs(n *ast.Link) (classes []string, kv map[string]string, ok bool) {
	if n == nil {
		return nil, nil, false
	}
	linkAttrsMu.Lock()
	a, found := linkAttrs[n]
	linkAttrsMu.Unlock()
	if !found {
		return nil, nil, false
	}
	return a.Classes, a.KV, true
}

// attributeExtender registers the inline attribute parser and the no-op
// renderer for the empty node it emits. The trigger character is '{', distinct
// from the link parser's '[', so priority relative to the link parser is
// irrelevant; we use 100.
type attributeExtender struct{}

func (attributeExtender) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithInlineParsers(
			util.Prioritized(linkAttrParser{}, 100),
		),
	)
	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(emptyRenderer{}, 100),
		),
	)
}

// AttributeExtension returns a goldmark Extender that adds inline link
// attribute support: a "{...}" block immediately following a link's closing
// ")" is parsed and attached to the ast.Link (readable via LinkAttrs), and the
// raw braces are consumed so they never appear in the rendered output.
func AttributeExtension() goldmark.Extender { return attributeExtender{} }
