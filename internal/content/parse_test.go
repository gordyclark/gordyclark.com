package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, name, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	return path
}

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name       string
		contents   string
		wantErr    bool
		errSubstr  string
		wantStatus Status
		wantTags   []string
		wantTitle  string
	}{
		{
			name: "happy path",
			contents: "---\n" +
				"title: Hello\n" +
				"slug: hello\n" +
				"date: 2024-01-02\n" +
				"tags: [a, b]\n" +
				"status: draft\n" +
				"---\nbody here\n",
			wantStatus: StatusDraft,
			wantTags:   []string{"a", "b"},
			wantTitle:  "Hello",
		},
		{
			name: "default status and tags",
			contents: "---\n" +
				"title: T\n" +
				"slug: s\n" +
				"date: 2024-01-02\n" +
				"---\nbody\n",
			wantStatus: StatusFinished,
			wantTags:   []string{},
			wantTitle:  "T",
		},
		{
			name: "missing title",
			contents: "---\n" +
				"slug: s\n" +
				"date: 2024-01-02\n" +
				"---\n",
			wantErr:   true,
			errSubstr: `missing required frontmatter field "title"`,
		},
		{
			name: "missing slug",
			contents: "---\n" +
				"title: T\n" +
				"date: 2024-01-02\n" +
				"---\n",
			wantErr:   true,
			errSubstr: `missing required frontmatter field "slug"`,
		},
		{
			name: "missing date",
			contents: "---\n" +
				"title: T\n" +
				"slug: s\n" +
				"---\n",
			wantErr:   true,
			errSubstr: `missing required frontmatter field "date"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTemp(t, "essay.md", tt.contents)
			fm, err := ParseFrontmatter(path)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
				// error should name the file
				if !strings.Contains(err.Error(), filepath.Base(path)) {
					t.Fatalf("error %q does not name the file", err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if fm.Title != tt.wantTitle {
				t.Errorf("title = %q, want %q", fm.Title, tt.wantTitle)
			}
			if fm.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", fm.Status, tt.wantStatus)
			}
			if len(fm.Tags) != len(tt.wantTags) {
				t.Errorf("tags = %v, want %v", fm.Tags, tt.wantTags)
			}
			if fm.Tags == nil {
				t.Errorf("tags should be non-nil after defaults")
			}
		})
	}
}

func TestParseEssayBodyOffset(t *testing.T) {
	tests := []struct {
		name       string
		contents   string
		wantOffset int
		wantBody   string
	}{
		{
			name: "single-line frontmatter body",
			// line1: ---, line2: title, line3: slug, line4: date, line5: ---, line6: body
			contents: "---\n" +
				"title: T\n" +
				"slug: s\n" +
				"date: 2024-01-02\n" +
				"---\n" +
				"first body line\n" +
				"second body line\n",
			wantOffset: 6,
			wantBody:   "first body line\nsecond body line\n",
		},
		{
			name: "multi-line frontmatter",
			// 1:--- 2:title 3:subtitle 4:slug 5:date 6:tags 7:- a 8:- b 9:--- 10:body
			contents: "---\n" +
				"title: T\n" +
				"subtitle: Sub\n" +
				"slug: s\n" +
				"date: 2024-01-02\n" +
				"tags:\n" +
				"  - a\n" +
				"  - b\n" +
				"---\n" +
				"body starts here\n",
			wantOffset: 10,
			wantBody:   "body starts here\n",
		},
		{
			name: "empty body after frontmatter",
			contents: "---\n" +
				"title: T\n" +
				"slug: s\n" +
				"date: 2024-01-02\n" +
				"---\n",
			wantOffset: 6,
			wantBody:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTemp(t, "essay.md", tt.contents)
			e, err := ParseEssay(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if e.BodyOffset != tt.wantOffset {
				t.Errorf("BodyOffset = %d, want %d", e.BodyOffset, tt.wantOffset)
			}
			if string(e.Body) != tt.wantBody {
				t.Errorf("Body = %q, want %q", string(e.Body), tt.wantBody)
			}
			if e.SourcePath != path {
				t.Errorf("SourcePath = %q, want %q", e.SourcePath, path)
			}
		})
	}
}

func TestParseEssayMissingField(t *testing.T) {
	path := writeTemp(t, "bad.md", "---\nslug: s\ndate: 2024-01-02\n---\nbody\n")
	if _, err := ParseEssay(path); err == nil {
		t.Fatal("expected error for missing title")
	}
}
