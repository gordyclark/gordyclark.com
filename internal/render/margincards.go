package render

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/gordyclark/gordyclark.com/internal/margin"
)

// The margin-cell markup produced here matches the CSS component classes in
// assets/css/components/ exactly:
//
//   .margin-card    (metadata card: crumb + dl rows + tags)
//   .margin-note    (footnote body relocated to the margin)
//   .margin-citation(bibliographic citation from citations.yaml)
//   .margin-chip    (external link chip)
//   .margin-chip.internal (same-site link chip; markup mirrors essay.html.tmpl's
//                          "Related" chips so the two read identically)
//
// Card templates receive already-rendered, trusted template.HTML for any body
// content (footnote bodies) so nested markup survives; scalar strings are
// auto-escaped by html/template.

var (
	tmplMarginCard = template.Must(template.New("margin-card").Parse(`<div class="margin-card">
  <div class="crumb"><a href="/">Home</a> / <a href="/">Essays</a> / {{.Title}}</div>
  <dl>
    <div class="row"><dt>Published</dt><dd>{{.Date}}</dd></div>
    <div class="row"><dt>Reading time</dt><dd>{{.ReadingTime}} min</dd></div>
  </dl>
{{- if .Tags}}
  <div class="tags">{{range .Tags}}<span class="tag">{{.}}</span>{{end}}</div>
{{- end}}
</div>
`))

	tmplMarginNote = template.Must(template.New("margin-note").Parse(`<div class="margin-note">
  <span class="kind">Note</span>
  {{.Body}}
</div>
`))

	tmplMarginCitation = template.Must(template.New("margin-citation").Parse(`<div class="margin-citation">
  <span class="kind">Citation</span>
  <cite>{{.Title}}</cite>
{{- if .Author}}
  <span class="src">{{.Author}}{{if .Year}}, {{.Year}}{{end}}</span>
{{- else if .Year}}
  <span class="src">{{.Year}}</span>
{{- end}}
{{- if .Source}}
  <span class="src">{{.Source}}</span>
{{- end}}
{{- if .URL}}
  <a href="{{.URL}}">{{.URL}}</a>
{{- end}}
</div>
`))

	tmplMarginChipExternal = template.Must(template.New("margin-chip").Parse(`<a class="margin-chip" href="{{.URL}}">
  <span class="domain"><span class="dot">{{.Initial}}</span>{{.Domain}}</span>
  <span class="chip-title">{{.Title}}</span>
{{- if .Desc}}
  <span class="chip-desc">{{.Desc}}</span>
{{- end}}
</a>
`))

	// The internal chip markup mirrors templates/essay.html.tmpl's related list:
	// the diamond glyph &#9670; and the "On this site" label.
	tmplMarginChipInternal = template.Must(template.New("margin-chip-internal").Parse(`<a class="margin-chip internal" href="/essays/{{.Slug}}/">
  <span class="domain"><span class="dot">&#9670;</span>On this site</span>
  <span class="chip-title">{{.Title}}</span>
{{- if .Desc}}
  <span class="chip-desc">{{.Desc}}</span>
{{- end}}
</a>
`))
)

// metaCardData is the data for the metadata card prepended to the first block.
type metaCardData struct {
	Title       string
	Date        string
	ReadingTime int
	Tags        []string
}

func renderMetaCard(d metaCardData) (template.HTML, error) {
	var buf bytes.Buffer
	if err := tmplMarginCard.Execute(&buf, d); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil //nolint:gosec // trusted internal markup
}

// renderMarginItem renders one classified MarginItem to its margin-cell HTML.
// noteBodies maps a MarginNote's DefNode to its pre-rendered footnote body HTML.
func renderMarginItem(it margin.MarginItem, noteBodies map[any]template.HTML) (template.HTML, error) {
	var buf bytes.Buffer
	switch it.Kind {
	case margin.MarginNote:
		body := noteBodies[it.DefNode]
		if err := tmplMarginNote.Execute(&buf, struct{ Body template.HTML }{body}); err != nil {
			return "", err
		}
	case margin.MarginCitation:
		c := it.Citation
		data := struct {
			Title, Author, Year, Source, URL string
		}{}
		if c != nil {
			data.Title = c.Title
			data.Author = c.Author
			data.Year = c.Year
			data.Source = c.Source
			data.URL = c.URL
		}
		if err := tmplMarginCitation.Execute(&buf, data); err != nil {
			return "", err
		}
	case margin.MarginChipExternal:
		data := struct {
			URL, Domain, Initial, Title, Desc string
		}{
			URL:     it.URL,
			Domain:  it.Domain,
			Initial: margin.DomainInitial(it.Domain),
			Title:   it.Title,
			Desc:    it.Desc,
		}
		if err := tmplMarginChipExternal.Execute(&buf, data); err != nil {
			return "", err
		}
	case margin.MarginChipInternal:
		data := struct {
			Slug, Title, Desc string
		}{
			Slug:  it.TargetSlug,
			Title: it.Title,
			Desc:  it.Desc,
		}
		if err := tmplMarginChipInternal.Execute(&buf, data); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unknown margin item kind %d", it.Kind)
	}
	return template.HTML(buf.String()), nil //nolint:gosec // trusted internal markup
}
