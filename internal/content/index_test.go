package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeEssay(t *testing.T, dir, name, title, slug, date string) {
	t.Helper()
	contents := "---\n" +
		"title: " + title + "\n" +
		"slug: " + slug + "\n" +
		"date: " + date + "\n" +
		"---\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
		t.Fatalf("writing essay: %v", err)
	}
}

func TestLoadIndexOrdering(t *testing.T) {
	dir := t.TempDir()
	writeEssay(t, dir, "old.md", "Old", "old", "2020-01-01")
	writeEssay(t, dir, "new.md", "New", "new", "2024-06-01")
	writeEssay(t, dir, "mid.md", "Mid", "mid", "2022-03-15")
	// non-md files should be ignored
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore"), 0o644); err != nil {
		t.Fatal(err)
	}

	ix, err := LoadIndex(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ix.Ordered) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(ix.Ordered))
	}
	wantOrder := []string{"new", "mid", "old"}
	for i, w := range wantOrder {
		if ix.Ordered[i].Slug != w {
			t.Errorf("Ordered[%d] = %q, want %q", i, ix.Ordered[i].Slug, w)
		}
	}
	if ix.Get("new") == nil || ix.Get("new").Title != "New" {
		t.Errorf("BySlug lookup failed for 'new'")
	}
	if ix.Get("new").SourcePath != filepath.Join(dir, "new.md") {
		t.Errorf("SourcePath = %q, want %q", ix.Get("new").SourcePath, filepath.Join(dir, "new.md"))
	}
}

func TestLoadIndexTieBreakBySlug(t *testing.T) {
	dir := t.TempDir()
	// same date; expect slug ascending
	writeEssay(t, dir, "b.md", "B", "banana", "2024-01-01")
	writeEssay(t, dir, "a.md", "A", "apple", "2024-01-01")

	ix, err := LoadIndex(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ix.Ordered[0].Slug != "apple" || ix.Ordered[1].Slug != "banana" {
		t.Errorf("tie-break order = [%q, %q], want [apple, banana]",
			ix.Ordered[0].Slug, ix.Ordered[1].Slug)
	}
}

func TestLoadIndexDuplicateSlug(t *testing.T) {
	dir := t.TempDir()
	writeEssay(t, dir, "one.md", "One", "dup", "2024-01-01")
	writeEssay(t, dir, "two.md", "Two", "dup", "2024-02-01")

	_, err := LoadIndex(dir)
	if err == nil {
		t.Fatal("expected duplicate slug error")
	}
	if !strings.Contains(err.Error(), "one.md") || !strings.Contains(err.Error(), "two.md") {
		t.Errorf("error %q should name both files", err.Error())
	}
}

func TestLoadIndexBadDate(t *testing.T) {
	dir := t.TempDir()
	writeEssay(t, dir, "bad.md", "Bad", "bad", "not-a-date")
	_, err := LoadIndex(dir)
	if err == nil {
		t.Fatal("expected date parse error")
	}
	if !strings.Contains(err.Error(), "bad.md") {
		t.Errorf("error %q should name the file", err.Error())
	}
}

func TestLoadIndexMissingDir(t *testing.T) {
	_, err := LoadIndex(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}
