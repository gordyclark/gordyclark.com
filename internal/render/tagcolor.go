package render

import (
	"hash/fnv"
	"strconv"
)

// tagPaletteSize is the number of tag hues defined in assets/css/tokens.css as
// --tag-1 .. --tag-N. Keep the two in step: a tag's class is derived modulo
// this number, so changing it re-colors existing tags.
const tagPaletteSize = 8

// tagColorClass maps a tag name to one of the palette classes, "tag-c1" through
// "tag-cN".
//
// The assignment looks arbitrary but is a pure function of the name, which is
// what makes it useful: a tag gets a color the first time it is used, with
// nobody assigning one, and it keeps that color on every page and across
// rebuilds. A real random draw would repaint every tag on every build.
//
// FNV-1a is used rather than Go's map hash because the latter is seeded per
// process and would not be stable between builds.
func tagColorClass(tag string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(tag))
	return "tag-c" + strconv.Itoa(int(h.Sum32()%tagPaletteSize)+1)
}
