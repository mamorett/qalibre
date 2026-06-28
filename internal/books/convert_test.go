package books

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanUTF8(t *testing.T) {
	input := []byte("Hello, \xffWorld!")
	expected := "Hello, \ufffdWorld!"
	got := cleanUTF8(input)
	if got != expected {
		t.Errorf("cleanUTF8 failed: expected %q, got %q", expected, got)
	}
}

func TestToMarkdownFromTxt(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "qalibre_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test.txt")
	content := "Line 1\n\nLine 2"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := ToMarkdown(filePath, "txt")
	if err != nil {
		t.Fatalf("ToMarkdown txt error: %v", err)
	}
	if got != content {
		t.Errorf("ToMarkdown txt failed: expected %q, got %q", content, got)
	}
}

func TestToMarkdownFromHtml(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "qalibre_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test.html")
	content := "<html><head><title>Page Title</title></head><body><h1>Chapter 1</h1><p>Paragraph text.</p></body></html>"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := ToMarkdown(filePath, "html")
	if err != nil {
		t.Fatalf("ToMarkdown html error: %v", err)
	}
	if !strings.Contains(got, "Chapter 1") {
		t.Errorf("ToMarkdown html heading mismatch: %q", got)
	}
	if !strings.Contains(got, "Paragraph text.") {
		t.Errorf("ToMarkdown html paragraph text missing: %q", got)
	}
}
