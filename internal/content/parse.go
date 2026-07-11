package content

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// splitFrontmatter splits raw file bytes into the YAML frontmatter block and
// the body. It returns the frontmatter YAML, the body bytes, and the 1-based
// line number where the body begins.
//
// The frontmatter is delimited by lines that are exactly "---" (ignoring a
// trailing \r) at the very top of the file. If the file does not begin with a
// "---" delimiter, there is no frontmatter: the whole file is the body and the
// body offset is 1.
func splitFrontmatter(raw []byte) (fm []byte, body []byte, bodyOffset int, err error) {
	lines := splitLinesKeepEmpty(raw)
	if len(lines) == 0 || !isDelim(lines[0]) {
		// No frontmatter block.
		return nil, raw, 1, nil
	}

	// Find the closing delimiter starting after the opening one.
	for i := 1; i < len(lines); i++ {
		if isDelim(lines[i]) {
			fmLines := lines[1:i]
			fm = bytes.Join(fmLines, []byte("\n"))
			// Body starts at line i+2 (1-based): line 1 is opening ---,
			// lines 2..i are frontmatter, line i+1 is closing ---, so the
			// body's first line is at 1-based index i+2.
			bodyOffset = i + 2
			bodyLines := lines[i+1:]
			body = bytes.Join(bodyLines, []byte("\n"))
			return fm, body, bodyOffset, nil
		}
	}
	return nil, nil, 0, fmt.Errorf("unterminated frontmatter block (missing closing \"---\")")
}

// splitLinesKeepEmpty splits on \n, stripping a single trailing \r from each
// line so both LF and CRLF files behave the same. A trailing newline does not
// produce a spurious final empty element.
func splitLinesKeepEmpty(raw []byte) [][]byte {
	if len(raw) == 0 {
		return nil
	}
	parts := bytes.Split(raw, []byte("\n"))
	for i, p := range parts {
		parts[i] = bytes.TrimSuffix(p, []byte("\r"))
	}
	return parts
}

func isDelim(line []byte) bool {
	return string(bytes.TrimSuffix(line, []byte("\r"))) == "---"
}

// applyDefaults sets defaults on parsed frontmatter: unset status becomes
// StatusFinished, and a nil tag slice becomes a non-nil empty slice.
func applyDefaults(fm *Frontmatter) {
	if fm.Status == "" {
		fm.Status = StatusFinished
	}
	if fm.Tags == nil {
		fm.Tags = []string{}
	}
}

// validateRequired checks the required frontmatter fields and returns an error
// naming the file and the first missing field.
func validateRequired(fm Frontmatter, path string) error {
	switch {
	case fm.Title == "":
		return fmt.Errorf("%s: missing required frontmatter field %q", path, "title")
	case fm.Slug == "":
		return fmt.Errorf("%s: missing required frontmatter field %q", path, "slug")
	case fm.Date == "":
		return fmt.Errorf("%s: missing required frontmatter field %q", path, "date")
	}
	return nil
}

// ParseFrontmatter reads the file at path and returns its parsed, defaulted,
// and validated frontmatter.
func ParseFrontmatter(path string) (Frontmatter, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Frontmatter{}, err
	}
	fmBytes, _, _, err := splitFrontmatter(raw)
	if err != nil {
		return Frontmatter{}, fmt.Errorf("%s: %w", path, err)
	}

	var fm Frontmatter
	if len(fmBytes) > 0 {
		if err := yaml.Unmarshal(fmBytes, &fm); err != nil {
			return Frontmatter{}, fmt.Errorf("%s: parsing frontmatter: %w", path, err)
		}
	}
	applyDefaults(&fm)
	if err := validateRequired(fm, path); err != nil {
		return Frontmatter{}, err
	}
	return fm, nil
}

// ParseEssay reads the file at path and returns the full parsed essay:
// frontmatter, raw body bytes, source path, and the 1-based line offset of the
// body's first line in the original file.
func ParseEssay(path string) (*Essay, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fmBytes, body, bodyOffset, err := splitFrontmatter(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	var fm Frontmatter
	if len(fmBytes) > 0 {
		if err := yaml.Unmarshal(fmBytes, &fm); err != nil {
			return nil, fmt.Errorf("%s: parsing frontmatter: %w", path, err)
		}
	}
	applyDefaults(&fm)
	if err := validateRequired(fm, path); err != nil {
		return nil, err
	}

	return &Essay{
		Front:      fm,
		Body:       body,
		SourcePath: path,
		BodyOffset: bodyOffset,
	}, nil
}
