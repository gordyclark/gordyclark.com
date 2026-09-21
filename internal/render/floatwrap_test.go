package render

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// contentCells returns each content-cell's inner HTML, in document order.
func contentCells(html string) []string {
	re := regexp.MustCompile(`(?s)<div class="content-cell">(.*?)</div><div class="margin-cell">`)
	var cells []string
	for _, m := range re.FindAllStringSubmatch(html, -1) {
		cells = append(cells, m[1])
	}
	return cells
}

// cellWithFloat returns the first content cell holding a floated image.
func cellWithFloat(t *testing.T, html string) string {
	t.Helper()
	for _, c := range contentCells(html) {
		if strings.Contains(c, "align-left") || strings.Contains(c, "align-right") {
			return c
		}
	}
	t.Fatal("no content cell contains a floated image")
	return ""
}

func buildPost(t *testing.T, body string) string {
	t.Helper()
	opts, tmp := scaffoldKinds(t)
	writeFileT(t, filepath.Join(opts.ContentDir, "blog", "p.md"), body)
	if err := Build(opts); err != nil {
		t.Fatalf("build: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(tmp, "static", "blog", "p", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const floatFront = `---
title: "P"
slug: p
date: 2026-09-21
status: finished
---

`

// A floated image must share its content cell with the text that follows it.
// Each top-level block otherwise gets its own cell, and a float cannot escape
// its containing block, so an image alone in a cell has nothing to wrap.
func TestFloatedImageSharesACellWithFollowingText(t *testing.T) {
	html := buildPost(t, floatFront+"```img\nsrc=\"/img/sample.png\"\nalign=\"right\"\nalt=\"A\"\n```\n\nThis text should wrap beside the image.\n")

	cell := cellWithFloat(t, html)
	if !strings.Contains(cell, "This text should wrap beside the image.") {
		t.Error("the paragraph after a floated image must share its cell, or the float has nothing to wrap")
	}
}

// The run keeps collecting blocks, so a list after a paragraph wraps too.
func TestFloatRunCollectsSeveralBlocks(t *testing.T) {
	html := buildPost(t, floatFront+"```img\nsrc=\"/img/sample.png\"\nalign=\"left\"\nalt=\"A\"\n```\n\nIntro line.\n\n* first\n* second\n")

	cell := cellWithFloat(t, html)
	if !strings.Contains(cell, "Intro line.") {
		t.Error("paragraph should join the float's cell")
	}
	if !strings.Contains(cell, "<li>first</li>") {
		t.Error("list should join the float's cell too")
	}
}

// A heading starts a new section, so it must not be pulled up beside the
// previous section's image.
func TestFloatRunEndsAtAHeading(t *testing.T) {
	html := buildPost(t, floatFront+"```img\nsrc=\"/img/sample.png\"\nalign=\"right\"\nalt=\"A\"\n```\n\nWraps beside it.\n\n## Next Section\n\nBelongs below.\n")

	cell := cellWithFloat(t, html)
	if !strings.Contains(cell, "Wraps beside it.") {
		t.Error("the paragraph before the heading should be in the float's cell")
	}
	if strings.Contains(cell, "Next Section") {
		t.Error("a heading must end the run, not join it")
	}
	if strings.Contains(cell, "Belongs below.") {
		t.Error("content after the heading must not be pulled into the float's cell")
	}
}

// Two floats in a row would collide inside one cell, so an image ends the run.
func TestFloatRunEndsAtTheNextImage(t *testing.T) {
	html := buildPost(t, floatFront+"```img\nsrc=\"/img/sample.png\"\nalign=\"right\"\nalt=\"A\"\n```\n\nBeside the first.\n\n```img\nsrc=\"/img/sample.png\"\nalign=\"left\"\nalt=\"B\"\n```\n\nBeside the second.\n")

	cells := contentCells(html)
	var floatCells []string
	for _, c := range cells {
		if strings.Contains(c, "post-img align-") {
			floatCells = append(floatCells, c)
		}
	}
	if len(floatCells) != 2 {
		t.Fatalf("expected each float in its own cell, got %d cells with floats", len(floatCells))
	}
	if strings.Contains(floatCells[0], `alt="B"`) {
		t.Error("two floated images must not share a cell; they would collide")
	}
	if !strings.Contains(floatCells[0], "Beside the first.") {
		t.Error("first float should still wrap its own paragraph")
	}
	if !strings.Contains(floatCells[1], "Beside the second.") {
		t.Error("second float should wrap the paragraph after it")
	}
}

// A centered image is an ordinary block and keeps its own cell, so the text
// under it starts below rather than beside it.
func TestCenteredImageDoesNotOpenARun(t *testing.T) {
	html := buildPost(t, floatFront+"```img\nsrc=\"/img/sample.png\"\nalign=\"center\"\nalt=\"A\"\n```\n\nBelow the image.\n")

	for _, c := range contentCells(html) {
		if strings.Contains(c, "align-center") && strings.Contains(c, "Below the image.") {
			t.Error("a centered image should not pull the following text into its cell")
		}
	}
}

// The grid pairs one margin cell with each content cell. A run emits a single
// pair, so the counts must stay equal or the two columns fall out of step.
func TestFloatRunKeepsGridCellsBalanced(t *testing.T) {
	html := buildPost(t, floatFront+"```img\nsrc=\"/img/sample.png\"\nalign=\"right\"\nalt=\"A\"\n```\n\nOne.\n\nTwo.\n\n## Heading\n\nThree.\n")

	content := strings.Count(html, `<div class="content-cell">`)
	margin := strings.Count(html, `<div class="margin-cell">`)
	if content != margin {
		t.Errorf("%d content cells vs %d margin cells; the grid columns must stay paired", content, margin)
	}
}

// A float that runs to the end of the document still has its cell closed.
func TestFloatRunAtEndOfDocumentIsClosed(t *testing.T) {
	html := buildPost(t, floatFront+"```img\nsrc=\"/img/sample.png\"\nalign=\"right\"\nalt=\"A\"\n```\n\nLast paragraph.\n")

	if strings.Count(html, `<div class="content-cell">`) != strings.Count(html, `<div class="margin-cell">`) {
		t.Error("an unterminated float run left the grid unbalanced")
	}
	if !strings.Contains(html, "</div></div>") && !strings.Contains(html, `Last paragraph.</p>`) {
		t.Error("the trailing run should still render its content")
	}
}
