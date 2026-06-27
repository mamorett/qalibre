// Package mime maps file extensions to MIME content types.
// It replaces the mimetypes init in cps/__init__.py.
package mime

import "strings"

// extensionMap covers all formats used in Qalibre (audio types removed).
var extensionMap = map[string]string{
	// ebook formats
	".epub":  "application/epub+zip",
	".kepub": "application/epub+zip",
	".mobi":  "application/x-mobipocket-ebook",
	".azw":   "application/vnd.amazon.ebook",
	".azw3":  "application/vnd.amazon.ebook",
	".prc":   "application/x-mobipocket-ebook",
	".pdf":   "application/pdf",
	".txt":   "text/plain; charset=utf-8",
	".html":  "text/html; charset=utf-8",
	".rtf":   "application/rtf",
	".lit":   "application/x-ms-reader",
	".odt":   "application/vnd.oasis.opendocument.text",
	".docx":  "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".doc":   "application/msword",
	".fb2":   "application/x-fictionbook+xml",
	".djvu":  "image/vnd.djvu",
	".djv":   "image/vnd.djvu",
	".lrf":   "application/x-sony-bbeb",
	".htmlz": "application/zip",
	".azw4":  "application/vnd.amazon.ebook",
	// comic formats
	".cbz": "application/zip",
	".cbt": "application/x-tar",
	".cbr": "application/x-cbr",
	".cb7": "application/x-7z-compressed",
	// image formats
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".bmp":  "image/bmp",
	// opds / xml
	".opf": "application/oebps-package+xml",
	".xml": "application/xml",
	".atom": "application/atom+xml",
}

// ByExtension returns the MIME type for a file extension (including the dot).
// Falls back to "application/octet-stream".
func ByExtension(ext string) string {
	ext = strings.ToLower(ext)
	if ct, ok := extensionMap[ext]; ok {
		return ct
	}
	return "application/octet-stream"
}

// ByFilename returns the MIME type for a filename based on its extension.
func ByFilename(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx < 0 {
		return "application/octet-stream"
	}
	return ByExtension(name[idx:])
}
