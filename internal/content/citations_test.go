package content

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCitationsHappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "citations.yaml")
	contents := "smith2020:\n" +
		"  author: Jane Smith\n" +
		"  title: On Things\n" +
		"  source: Journal\n" +
		"  year: \"2020\"\n" +
		"  url: https://example.com\n" +
		"doe2019:\n" +
		"  author: John Doe\n" +
		"  title: More Things\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	cites, err := LoadCitations(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cites) != 2 {
		t.Fatalf("expected 2 citations, got %d", len(cites))
	}
	if cites["smith2020"].Author != "Jane Smith" {
		t.Errorf("smith2020.Author = %q, want Jane Smith", cites["smith2020"].Author)
	}
	if cites["smith2020"].Year != "2020" {
		t.Errorf("smith2020.Year = %q, want 2020", cites["smith2020"].Year)
	}
	if cites["doe2019"].Title != "More Things" {
		t.Errorf("doe2019.Title = %q, want More Things", cites["doe2019"].Title)
	}
}

func TestLoadCitationsEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "citations.yaml")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cites, err := LoadCitations(path)
	if err != nil {
		t.Fatalf("unexpected error for empty file: %v", err)
	}
	if cites == nil {
		t.Fatal("expected non-nil map for empty file")
	}
	if len(cites) != 0 {
		t.Errorf("expected empty map, got %d entries", len(cites))
	}
}

func TestLoadCitationsMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope.yaml")
	_, err := LoadCitations(path)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
