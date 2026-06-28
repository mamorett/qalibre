package books

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/dslipak/pdf"
	"github.com/microcosm-cc/bluemonday"
)

var (
	ErrImageOnlyPDF = errors.New("image-only PDF skipped")
	titleRx         = regexp.MustCompile(`(?i)<title>(.*?)</title>`)
	h1Rx            = regexp.MustCompile(`(?i)<h1>(.*?)</h1>`)
)

const MIN_TEXT_CHARS = 64

// ToMarkdown converts a single book file to Markdown text.
// format = lowercased extension without dot ("txt", "epub", "pdf", "html").
func ToMarkdown(filePath, format string) (string, error) {
	slog.Info("convert: dispatching format to converter", "format", format, "filePath", filePath)
	switch strings.ToLower(format) {
	case "txt":
		return ToMarkdownFromTxt(filePath)
	case "epub":
		return ToMarkdownFromEpub(filePath)
	case "pdf":
		return ToMarkdownFromPdf(filePath)
	case "html", "htm":
		return ToMarkdownFromHtml(filePath)
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

func ToMarkdownFromEpub(filePath string) (string, error) {
	slog.Info("convert: epub conversion starting", "filePath", filePath)
	zr, err := zip.OpenReader(filePath)
	if err != nil {
		return "", err
	}
	defer zr.Close()

	// 1. Read META-INF/container.xml to find OPF path
	var opfPath string
	for _, f := range zr.File {
		if f.Name == "META-INF/container.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			var container ContainerXml
			err = xml.NewDecoder(rc).Decode(&container)
			rc.Close()
			if err == nil && len(container.Rootfiles) > 0 {
				opfPath = container.Rootfiles[0].FullPath
			}
			break
		}
	}

	if opfPath == "" {
		for _, f := range zr.File {
			if strings.HasSuffix(f.Name, ".opf") {
				opfPath = f.Name
				break
			}
		}
	}

	if opfPath == "" {
		return "", fmt.Errorf("could not locate OPF file in EPUB")
	}

	slog.Info("convert: epub OPF path located", "filePath", filePath, "opfPath", opfPath)

	// 2. Read OPF
	var opf OpfXml
	var opfFile *zip.File
	for _, f := range zr.File {
		if f.Name == opfPath {
			opfFile = f
			break
		}
	}

	if opfFile == nil {
		return "", fmt.Errorf("OPF file %s not found in EPUB", opfPath)
	}

	rc, err := opfFile.Open()
	if err != nil {
		return "", err
	}
	err = xml.NewDecoder(rc).Decode(&opf)
	rc.Close()
	if err != nil {
		return "", err
	}

	// Build manifest item map: id -> href
	manifestMap := make(map[string]string)
	for _, item := range opf.Manifest.Items {
		manifestMap[item.ID] = item.Href
	}

	// Walk spine in order
	var markdownParts []string
	opfDir := filepath.Dir(opfPath)
	totalSpineItems := len(opf.Spine.ItemRefs)
	slog.Info("convert: epub spine items count", "filePath", filePath, "count", totalSpineItems)

	for idx, itemref := range opf.Spine.ItemRefs {
		href, exists := manifestMap[itemref.IDRef]
		if !exists {
			continue
		}

		// Resolve relative path to zip file name
		var fullPath string
		if opfDir == "." {
			fullPath = href
		} else {
			fullPath = filepath.Clean(filepath.Join(opfDir, href))
		}
		fullPath = strings.ReplaceAll(fullPath, "\\", "/")
		if unescaped, err := url.PathUnescape(fullPath); err == nil {
			fullPath = unescaped
		}

		slog.Info("convert: epub spine item reading", "filePath", filePath, "index", idx+1, "total", totalSpineItems, "fullPath", fullPath)

		// Find file in zip
		var targetFile *zip.File
		for _, f := range zr.File {
			if f.Name == fullPath {
				targetFile = f
				break
			}
		}

		if targetFile == nil {
			slog.Warn("convert: epub spine item file not found in zip", "filePath", filePath, "fullPath", fullPath)
			continue
		}

		trc, err := targetFile.Open()
		if err != nil {
			slog.Error("convert: failed to open epub spine item file", "filePath", filePath, "fullPath", fullPath, "err", err)
			continue
		}
		itemBytes, err := io.ReadAll(trc)
		trc.Close()
		if err != nil {
			slog.Error("convert: failed to read epub spine item file", "filePath", filePath, "fullPath", fullPath, "err", err)
			continue
		}

		cleaned := cleanXHTML(itemBytes)
		if cleaned != "" {
			markdownParts = append(markdownParts, cleaned)
		}
	}

	if len(markdownParts) == 0 {
		return "", fmt.Errorf("no content extracted from epub spine")
	}

	slog.Info("convert: epub conversion successful", "filePath", filePath, "sections", len(markdownParts))
	return strings.Join(markdownParts, "\n\n---\n\n"), nil
}

func cleanXHTML(xhtmlBytes []byte) string {
	content := string(xhtmlBytes)

	var chapterTitle string
	if m := h1Rx.FindStringSubmatch(content); len(m) > 1 {
		chapterTitle = html.UnescapeString(m[1])
	} else if m := titleRx.FindStringSubmatch(content); len(m) > 1 {
		chapterTitle = html.UnescapeString(m[1])
	}

	plainBytes := bluemonday.StrictPolicy().SanitizeBytes(xhtmlBytes)
	plain := html.UnescapeString(string(plainBytes))
	plain = collapseWhitespace(plain)

	if chapterTitle != "" {
		chapterTitle = collapseWhitespace(chapterTitle)
		if chapterTitle != "" {
			return "## " + chapterTitle + "\n\n" + plain
		}
	}
	return plain
}

func collapseWhitespace(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	lines := strings.Split(s, "\n")
	var cleanedLines []string
	for _, line := range lines {
		cleanedLines = append(cleanedLines, strings.TrimSpace(line))
	}
	s = strings.Join(cleanedLines, "\n")

	reNewlines := regexp.MustCompile(`\n{3,}`)
	s = reNewlines.ReplaceAllString(s, "\n\n")

	reSpaces := regexp.MustCompile(`[ \t]+`)
	s = reSpaces.ReplaceAllString(s, " ")

	return strings.TrimSpace(s)
}

func ToMarkdownFromPdf(filePath string) (string, error) {
	slog.Info("convert: pdf conversion starting", "filePath", filePath)

	// Try using pdftotext command-line tool first if installed in the system
	if _, err := exec.LookPath("pdftotext"); err == nil {
		slog.Info("convert: using system pdftotext for extraction", "filePath", filePath)
		cmd := exec.Command("pdftotext", "-layout", filePath, "-")
		var out bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err == nil {
			fullText := out.String()
			if len(strings.TrimSpace(fullText)) < MIN_TEXT_CHARS {
				slog.Warn("convert: pdf skipped - insufficient text content (likely scanned image PDF)", "filePath", filePath, "charsCount", len(fullText))
				return "", ErrImageOnlyPDF
			}
			res := collapseWhitespace(fullText)
			slog.Info("convert: pdf conversion successful (via pdftotext)", "filePath", filePath, "chars", len(res))
			return res, nil
		}
		slog.Warn("convert: pdftotext failed, falling back to pure-Go", "filePath", filePath, "err", err, "stderr", stderr.String())
	}

	// Fallback to pure-Go dslipak/pdf library
	slog.Info("convert: falling back to pure-Go dslipak/pdf", "filePath", filePath)
	r, err := pdf.Open(filePath)
	if err != nil {
		slog.Error("convert: failed to open pdf (pure-Go)", "filePath", filePath, "err", err)
		return "", err
	}

	reader, err := r.GetPlainText()
	if err != nil {
		slog.Error("convert: failed to get plain text reader from pdf (pure-Go)", "filePath", filePath, "err", err)
		return "", err
	}

	var buf bytes.Buffer
	_, err = buf.ReadFrom(reader)
	if err != nil {
		slog.Error("convert: failed to read plain text stream from pdf (pure-Go)", "filePath", filePath, "err", err)
		return "", err
	}

	fullText := buf.String()
	if len(strings.TrimSpace(fullText)) < MIN_TEXT_CHARS {
		slog.Warn("convert: pdf skipped - insufficient text content (likely scanned image PDF)", "filePath", filePath, "charsCount", len(fullText))
		return "", ErrImageOnlyPDF
	}

	res := collapseWhitespace(fullText)
	slog.Info("convert: pdf conversion successful (pure-Go)", "filePath", filePath, "chars", len(res))
	return res, nil
}

func ToMarkdownFromHtml(filePath string) (string, error) {
	slog.Info("convert: html conversion starting", "filePath", filePath)
	b, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	cleaned := cleanXHTML(b)
	slog.Info("convert: html conversion successful", "filePath", filePath, "chars", len(cleaned))
	return cleaned, nil
}
