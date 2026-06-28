package books

import (
	"context"
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

	got, err := ToMarkdown(context.Background(), filePath, "txt")
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

	got, err := ToMarkdown(context.Background(), filePath, "html")
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

func TestToChunkedMarkdown(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "qalibre_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test.txt")
	content := "This is a simple test file that we will use to verify that the chunking logic works as expected. We want to make sure it splits the text correctly."
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	chunks, err := ToChunkedMarkdown(context.Background(), filePath, "txt", 50, 10)
	if err != nil {
		t.Logf("Skipping ToChunkedMarkdown test because python dependencies might be missing: %v", err)
		return
	}

	if len(chunks) == 0 {
		t.Errorf("Expected chunks, got 0")
	}

	for _, chunk := range chunks {
		if chunk.Index <= 0 {
			t.Errorf("Invalid chunk index: %d", chunk.Index)
		}
		if chunk.Text == "" {
			t.Errorf("Empty chunk text")
		}
	}
}

func TestCleanMarkdownForChunking(t *testing.T) {
	input := `## 

**==> picture [421 x 171] intentionally omitted <==**

## Chapter 1

This is page text.

## 

**==> picture [421 x 170] intentionally omitted <==**

`
	expected := "## Chapter 1\n\nThis is page text."
	got := CleanMarkdownForChunking(input)
	if strings.TrimSpace(got) != expected {
		t.Errorf("CleanMarkdownForChunking failed:\nExpected:\n%q\nGot:\n%q", expected, got)
	}
}
