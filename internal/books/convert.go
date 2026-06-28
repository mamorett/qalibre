package books

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Chunk is one chunk of markdown text with provenance metadata.
type Chunk struct {
	Index int    // 1-based index within the book
	Text  string // chunk body (without front matter)
}

var (
	ErrImageOnlyPDF = errors.New("image-only PDF skipped")
)

const MIN_TEXT_CHARS = 64

// ToMarkdown converts a single book file to Markdown text.
// format = lowercased extension without dot ("txt", "epub", "pdf", "html").
func ToMarkdown(filePath, format string) (string, error) {
	slog.Info("convert: dispatching format to converter", "format", format, "filePath", filePath)
	formatLower := strings.ToLower(format)
	switch formatLower {
	case "txt":
		return ToMarkdownFromTxt(filePath)
	case "epub", "pdf", "html", "htm":
		res, err := toMarkdownViaPyMuPDF(filePath)
		if err != nil {
			return "", err
		}
		if formatLower == "pdf" && len(strings.TrimSpace(res)) < MIN_TEXT_CHARS {
			slog.Warn("convert: pdf skipped - insufficient text content (likely scanned image PDF)", "filePath", filePath, "charsCount", len(res))
			return "", ErrImageOnlyPDF
		}
		return res, nil
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

func ToMarkdownFromTxt(filePath string) (string, error) {
	slog.Info("convert: txt conversion starting", "filePath", filePath)
	b, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	res := cleanUTF8(b)
	slog.Info("convert: txt conversion successful", "filePath", filePath, "chars", len(res))
	return res, nil
}

func cleanUTF8(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	var r []rune
	for len(b) > 0 {
		rn, size := utf8.DecodeRune(b)
		if rn == utf8.RuneError && size == 1 {
			r = append(r, utf8.RuneError)
		} else {
			r = append(r, rn)
		}
		b = b[size:]
	}
	return string(r)
}

func toMarkdownViaPyMuPDF(filePath string) (string, error) {
	slog.Info("convert: converting via pymupdf4llm", "filePath", filePath)

	var cmd *exec.Cmd
	script := "import pymupdf4llm; import sys; print(pymupdf4llm.to_markdown(sys.argv[1]))"

	// 1. Try running direct python3 if pymupdf4llm is already installed
	if pythonPath, err := exec.LookPath("python3"); err == nil {
		// Check if pymupdf4llm is importable
		checkCmd := exec.Command(pythonPath, "-c", "import pymupdf4llm")
		if err := checkCmd.Run(); err == nil {
			slog.Debug("convert: using system python3 with pre-installed pymupdf4llm")
			cmd = exec.Command(pythonPath, "-c", script, filePath)
		}
	}

	// 2. Fall back to uvx / uv run if system python3 doesn't have it
	if cmd == nil {
		if uvxPath, err := exec.LookPath("uvx"); err == nil {
			slog.Debug("convert: falling back to uvx with pymupdf4llm")
			cmd = exec.Command(uvxPath, "--with", "pymupdf4llm", "python", "-c", script, filePath)
		} else if uvPath, err := exec.LookPath("uv"); err == nil {
			slog.Debug("convert: falling back to uv run with pymupdf4llm")
			cmd = exec.Command(uvPath, "run", "--with", "pymupdf4llm", "python", "-c", script, filePath)
		} else {
			// Try a common user fallback path for uvx
			userUvx := filepath.Join(os.Getenv("HOME"), ".local/bin/uvx")
			if _, err := os.Stat(userUvx); err == nil {
				slog.Debug("convert: using local user uvx")
				cmd = exec.Command(userUvx, "--with", "pymupdf4llm", "python", "-c", script, filePath)
			} else {
				// Final attempt: run python3 directly and hope for the best
				if pythonPath, err := exec.LookPath("python3"); err == nil {
					slog.Warn("convert: uv/uvx not found, trying system python3 direct")
					cmd = exec.Command(pythonPath, "-c", script, filePath)
				} else {
					return "", fmt.Errorf("neither 'python3' nor 'uvx'/'uv' was found in PATH to run pymupdf4llm")
				}
			}
		}
	}

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pymupdf4llm failed: %w (stderr: %s)", err, stderr.String())
	}

	return out.String(), nil
}

func ToChunkedMarkdown(filePath, format string, chunkSize, chunkOverlap int) ([]Chunk, error) {
	// 1. Convert to markdown
	mdText, err := ToMarkdown(filePath, format)
	if err != nil {
		return nil, err
	}

	// 2. Invoke a Python helper that uses langchain MarkdownTextSplitter to split it.
	script := `import sys, json
try:
    from langchain_text_splitters import MarkdownTextSplitter
except ImportError:
    from langchain.text_splitter import MarkdownTextSplitter

md_text = sys.stdin.read()
chunk_size = int(sys.argv[1])
chunk_overlap = int(sys.argv[2])

splitter = MarkdownTextSplitter(chunk_size=chunk_size, chunk_overlap=chunk_overlap)
chunks = splitter.split_text(md_text)
print(json.dumps({"chunks": chunks}))
`

	var cmd *exec.Cmd

	// Probe system python3 first
	if pythonPath, err := exec.LookPath("python3"); err == nil {
		// Check if langchain or langchain-text-splitters is importable
		checkCmd := exec.Command(pythonPath, "-c", "import langchain_text_splitters")
		if err := checkCmd.Run(); err == nil {
			cmd = exec.Command(pythonPath, "-c", script, fmt.Sprintf("%d", chunkSize), fmt.Sprintf("%d", chunkOverlap))
		} else {
			checkCmd2 := exec.Command(pythonPath, "-c", "from langchain.text_splitter import MarkdownTextSplitter")
			if err := checkCmd2.Run(); err == nil {
				cmd = exec.Command(pythonPath, "-c", script, fmt.Sprintf("%d", chunkSize), fmt.Sprintf("%d", chunkOverlap))
			}
		}
	}

	// Fallback to uvx / uv run
	if cmd == nil {
		if uvxPath, err := exec.LookPath("uvx"); err == nil {
			slog.Debug("convert: using uvx for chunking")
			cmd = exec.Command(uvxPath, "--with", "langchain-text-splitters", "python", "-c", script, fmt.Sprintf("%d", chunkSize), fmt.Sprintf("%d", chunkOverlap))
		} else if uvPath, err := exec.LookPath("uv"); err == nil {
			slog.Debug("convert: using uv run for chunking")
			cmd = exec.Command(uvPath, "run", "--with", "langchain-text-splitters", "python", "-c", script, fmt.Sprintf("%d", chunkSize), fmt.Sprintf("%d", chunkOverlap))
		} else {
			userUvx := filepath.Join(os.Getenv("HOME"), ".local/bin/uvx")
			if _, err := os.Stat(userUvx); err == nil {
				slog.Debug("convert: using local user uvx for chunking")
				cmd = exec.Command(userUvx, "--with", "langchain-text-splitters", "python", "-c", script, fmt.Sprintf("%d", chunkSize), fmt.Sprintf("%d", chunkOverlap))
			} else {
				if pythonPath, err := exec.LookPath("python3"); err == nil {
					slog.Warn("convert: uv/uvx not found, trying system python3 direct for chunking")
					cmd = exec.Command(pythonPath, "-c", script, fmt.Sprintf("%d", chunkSize), fmt.Sprintf("%d", chunkOverlap))
				} else {
					return nil, fmt.Errorf("neither 'python3' nor 'uvx'/'uv' was found in PATH to run langchain text splitter")
				}
			}
		}
	}

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdin = strings.NewReader(mdText)
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("langchain splitter failed: %w (stderr: %s)", err, stderr.String())
	}

	var response struct {
		Chunks []string `json:"chunks"`
	}
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		return nil, fmt.Errorf("failed to parse langchain output: %w (raw output: %s)", err, out.String())
	}

	chunks := make([]Chunk, len(response.Chunks))
	for i, text := range response.Chunks {
		chunks[i] = Chunk{
			Index: i + 1,
			Text:  text,
		}
	}

	return chunks, nil
}
