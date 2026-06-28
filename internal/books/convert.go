package books

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

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
