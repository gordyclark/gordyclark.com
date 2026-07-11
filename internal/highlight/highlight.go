// Package highlight renders source code to classed HTML using chroma.
package highlight

import (
	"html/template"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// formatter emits CSS-classed spans (no inline colours); colours come from CSS.
var formatter = chromahtml.New(chromahtml.WithClasses(true))

// Highlight tokenises source with the lexer for lang and formats it as a
// classed <pre class="chroma"><code> block. Unknown languages fall back to a
// plain, HTML-escaped <pre><code> block with a nil error.
func Highlight(source, lang string) (template.HTML, error) {
	lexer := lexers.Get(lang)
	if lexer == nil {
		return fallback(source), nil
	}

	iterator, err := lexer.Tokenise(nil, source)
	if err != nil {
		return "", err
	}

	// The formatter requires a *chroma.Style even in classes mode; with
	// WithClasses(true) its colours are suppressed in favour of CSS classes.
	style := styles.Get("github")
	if style == nil {
		style = styles.Fallback
	}

	var buf strings.Builder
	if err := formatter.Format(&buf, style, iterator); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil //nolint:gosec // chroma output is trusted/escaped
}

// fallback returns a plain, HTML-escaped code block matching the chroma wrapper.
func fallback(source string) template.HTML {
	var buf strings.Builder
	buf.WriteString(`<pre class="chroma"><code>`)
	buf.WriteString(template.HTMLEscapeString(source))
	buf.WriteString(`</code></pre>`)
	return template.HTML(buf.String()) //nolint:gosec // source is escaped above
}
