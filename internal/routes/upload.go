package routes

import (
	"archive/zip"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/qalibre/qalibre/internal/appdb"
	"github.com/qalibre/qalibre/internal/auth"
	"github.com/qalibre/qalibre/internal/books"
	"github.com/qalibre/qalibre/internal/config"
	"github.com/qalibre/qalibre/internal/helper"
)

// UploadBook handles POST /upload
func (rm *RouteManager) UploadBook(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r)
	if !ok {
		rm.ErrorJSON(w, "Login required", http.StatusUnauthorized)
		return
	}
	u := user.(*appdb.User)

	if !auth.HasRole(u.Role, config.RoleUpload) && !auth.HasRole(u.Role, config.RoleAdmin) {
		rm.ErrorJSON(w, "Permission denied", http.StatusForbidden)
		return
	}

	cfg := rm.Cfg.Get()
	if cfg.ConfigUploading != 1 {
		rm.ErrorJSON(w, "Uploading is disabled", http.StatusForbidden)
		return
	}

	// Max 100MB file uploads
	r.ParseMultipartForm(100 << 20)

	file, header, err := r.FormFile("btn-upload")
	if err != nil {
		rm.ErrorJSON(w, "No file uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	extTrimmed := strings.TrimPrefix(ext, ".")

	// Validate extension
	if !config.ExtensionsUpload[extTrimmed] {
		rm.ErrorJSON(w, "Unsupported file format", http.StatusBadRequest)
		return
	}

	// 1. Save temporarily to get file bytes and MD5
	tempDir := filepath.Join(helper.GetConfigDir(), "temp")
	_ = os.MkdirAll(tempDir, 0755)

	tempFileBytes, err := io.ReadAll(file)
	if err != nil {
		rm.ErrorJSON(w, "Failed to read uploaded file", http.StatusInternalServerError)
		return
	}

	h := md5.New()
	h.Write(tempFileBytes)
	md5Str := hex.EncodeToString(h.Sum(nil))

	tempFilePath := filepath.Join(tempDir, md5Str+ext)
	err = os.WriteFile(tempFilePath, tempFileBytes, 0644)
	if err != nil {
		rm.ErrorJSON(w, "Failed to write temp file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempFilePath) // Cleanup temp file at the end

	// 2. Extract metadata
	bookMeta := &books.BookMeta{
		Title:   strings.TrimSuffix(header.Filename, ext),
		Authors: []string{"Unknown"},
	}

	if ext == ".epub" || ext == ".kepub" {
		zipReader, err := zip.NewReader(file, int64(len(tempFileBytes)))
		if err == nil {
			extractedMeta, err := books.ExtractEpubMetadata(zipReader)
			if err == nil {
				bookMeta = extractedMeta
				// fallback title
				if bookMeta.Title == "" || bookMeta.Title == "Unknown" {
					bookMeta.Title = strings.TrimSuffix(header.Filename, ext)
				}
			}
		}
	}

	// 3. Perform database inserts using Transaction
	tx, err := rm.DB.Beginx()
	if err != nil {
		rm.ErrorJSON(w, "Failed to start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Title Sort
	titleSort := helper.GetSortedAuthor(bookMeta.Title) // simple fallback or leading article UDF
	authorPrimary := bookMeta.Authors[0]
	authorSortPrimary := helper.GetSortedAuthor(authorPrimary)

	hasCoverVal := 0
	if len(bookMeta.CoverData) > 0 {
		hasCoverVal = 1
	}

	res, err := tx.Exec(`
		INSERT INTO calibre.books (
			title, sort, author_sort, timestamp, pubdate, series_index, last_modified, path, has_cover, uuid
		) VALUES (?, ?, ?, ?, ?, '1.0', ?, '', ?, ?)`,
		bookMeta.Title,
		titleSort,
		authorSortPrimary,
		time.Now(),
		time.Date(101, 1, 1, 0, 0, 0, 0, time.UTC), // year 101 sentinel
		time.Now(),
		hasCoverVal,
		auth.GenerateRandomToken(16), // uuid token
	)
	if err != nil {
		rm.ErrorJSON(w, "Failed to insert book row", http.StatusInternalServerError)
		return
	}

	newBookID, _ := res.LastInsertId()
	bookID := int(newBookID)

	// Create Authors and link them
	var authorSorts []string
	for _, name := range bookMeta.Authors {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		var authorID int
		err = tx.Get(&authorID, "SELECT id FROM calibre.authors WHERE lower(name) = lower(?) LIMIT 1", name)
		if err != nil {
			sortName := helper.GetSortedAuthor(name)
			res, err := tx.Exec("INSERT INTO calibre.authors (name, sort, link) VALUES (?, ?, '')", name, sortName)
			if err == nil {
				id, _ := res.LastInsertId()
				authorID = int(id)
			}
		}
		_, _ = tx.Exec("INSERT INTO calibre.books_authors_link (book, author) VALUES (?, ?)", bookID, authorID)

		var sortName string
		_ = tx.Get(&sortName, "SELECT sort FROM calibre.authors WHERE id = ?", authorID)
		authorSorts = append(authorSorts, sortName)
	}

	// Update author sort string on books
	authorSortStr := strings.Join(authorSorts, " & ")
	_, _ = tx.Exec("UPDATE calibre.books SET author_sort = ? WHERE id = ?", authorSortStr, bookID)

	// Add comments if present
	if bookMeta.Description != "" {
		_, _ = tx.Exec("INSERT INTO calibre.comments (book, text) VALUES (?, ?)", bookID, bookMeta.Description)
	}

	// Add publisher if present
	if bookMeta.Publisher != "" {
		var pubID int
		err = tx.Get(&pubID, "SELECT id FROM calibre.publishers WHERE lower(name) = lower(?) LIMIT 1", bookMeta.Publisher)
		if err != nil {
			res, err := tx.Exec("INSERT INTO calibre.publishers (name, sort) VALUES (?, ?)", bookMeta.Publisher, bookMeta.Publisher)
			if err == nil {
				id, _ := res.LastInsertId()
				pubID = int(id)
			}
		}
		_, _ = tx.Exec("INSERT INTO calibre.books_publishers_link (book, publisher) VALUES (?, ?)", bookID, pubID)
	}

	// Add Language if present
	if bookMeta.Language != "" {
		langCode := strings.ToLower(bookMeta.Language)
		// simple 2 or 3 char code
		if len(langCode) > 3 {
			langCode = langCode[:3]
		}
		var langID int
		err = tx.Get(&langID, "SELECT id FROM calibre.languages WHERE lower(lang_code) = lower(?) LIMIT 1", langCode)
		if err != nil {
			res, err := tx.Exec("INSERT INTO calibre.languages (lang_code) VALUES (?)", langCode)
			if err == nil {
				id, _ := res.LastInsertId()
				langID = int(id)
			}
		}
		_, _ = tx.Exec("INSERT INTO calibre.books_languages_link (book, lang_code) VALUES (?, ?)", bookID, langID)
	}

	// Add ISBN Identifier if present
	if bookMeta.ISBN != "" {
		_, _ = tx.Exec("INSERT INTO calibre.identifiers (type, val, book) VALUES ('isbn', ?, ?)", bookMeta.ISBN, bookID)
	}

	// Add Data format row
	cleanAuthor, _ := helper.GetValidFilename(authorPrimary, true, 96, false, cfg.ConfigUnicodeFilename)
	cleanTitle, _ := helper.GetValidFilename(bookMeta.Title, true, 96, false, cfg.ConfigUnicodeFilename)
	newTitleDir := fmt.Sprintf("%s (%d)", cleanTitle, bookID)
	relBookPath := filepath.Join(cleanAuthor, newTitleDir)
	relBookPath = strings.ReplaceAll(relBookPath, "\\", "/")

	cleanAuthorShort, _ := helper.GetValidFilename(authorPrimary, true, 42, false, cfg.ConfigUnicodeFilename)
	cleanTitleShort, _ := helper.GetValidFilename(bookMeta.Title, true, 42, false, cfg.ConfigUnicodeFilename)
	dbFilename := fmt.Sprintf("%s - %s", cleanTitleShort, cleanAuthorShort)

	_, err = tx.Exec(`
		INSERT INTO calibre.data (book, format, uncompressed_size, name)
		VALUES (?, ?, ?, ?)`,
		bookID,
		strings.TrimPrefix(ext, "."),
		int64(len(tempFileBytes)),
		dbFilename,
	)
	if err != nil {
		rm.ErrorJSON(w, "Failed to insert format data", http.StatusInternalServerError)
		return
	}

	// Update book path
	_, _ = tx.Exec("UPDATE calibre.books SET path = ? WHERE id = ?", relBookPath, bookID)

	// Commit Transaction so paths exist in database
	err = tx.Commit()
	if err != nil {
		rm.ErrorJSON(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	// 4. Move file to target directory structure on disk
	calibreDir := rm.Cfg.GetCalibreDir()
	targetPath := filepath.Join(calibreDir, relBookPath)
	_ = os.MkdirAll(targetPath, 0755)

	targetFilePath := filepath.Join(targetPath, dbFilename+ext)
	err = os.WriteFile(targetFilePath, tempFileBytes, 0644)
	if err != nil {
		// remove book from DB on failure
		_, _ = rm.DB.Exec("DELETE FROM calibre.books WHERE id = ?", bookID)
		rm.ErrorJSON(w, "Failed to write book file to library", http.StatusInternalServerError)
		return
	}

	// 5. Write Cover if extracted
	if len(bookMeta.CoverData) > 0 {
		coverFilePath := filepath.Join(targetPath, "cover.jpg")
		_ = os.WriteFile(coverFilePath, bookMeta.CoverData, 0644)
	}

	// Enqueue metadata backup
	_, _ = rm.DB.Exec("INSERT OR IGNORE INTO calibre.metadata_dirtied (book) VALUES (?)", bookID)

	rm.WriteJSON(w, map[string]interface{}{"success": true})
}
