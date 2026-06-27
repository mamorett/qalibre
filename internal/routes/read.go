package routes

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/qalibre/qalibre/internal/appdb"
	"github.com/qalibre/qalibre/internal/auth"
	"github.com/qalibre/qalibre/internal/calibredb"
	"github.com/qalibre/qalibre/internal/mime"
)

// ReadBook handles GET /read/{id}/{fmt}
func (rm *RouteManager) ReadBook(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		rm.ErrorJSON(w, "Invalid book ID", http.StatusBadRequest)
		return
	}

	format := strings.ToUpper(strings.TrimSpace(chi.URLParam(r, "fmt")))
	if format == "" {
		rm.ErrorJSON(w, "Format not specified", http.StatusBadRequest)
		return
	}

	user, ok := auth.GetUserFromContext(r)
	if !ok {
		rm.ErrorJSON(w, "Login required", http.StatusUnauthorized)
		return
	}
	u := user.(*appdb.User)

	// Fetch book
	var book calibredb.Book
	err = rm.DB.Get(&book, "SELECT * FROM calibre.books WHERE id = ? LIMIT 1", id)
	if err != nil {
		rm.ErrorJSON(w, "Book not found", http.StatusNotFound)
		return
	}

	// Verify allowed tags or other restrictions
	allowed, err := rm.checkBookAllowed(book.ID, u)
	if err != nil || !allowed {
		rm.ErrorJSON(w, "Book not found", http.StatusNotFound)
		return
	}

	// Fetch format data row
	var data calibredb.Data
	err = rm.DB.Get(&data, "SELECT * FROM calibre.data WHERE book = ? AND format = ? LIMIT 1", id, format)
	if err != nil {
		rm.ErrorJSON(w, "Format not found for this book", http.StatusNotFound)
		return
	}

	calibreDir := rm.Cfg.GetCalibreDir()
	if calibreDir == "" {
		rm.ErrorJSON(w, "Calibre library directory is not configured", http.StatusInternalServerError)
		return
	}

	ext := strings.ToLower(data.Format)
	fileName := fmt.Sprintf("%s.%s", data.Name, ext)
	filePath := filepath.Join(calibreDir, book.Path, fileName)

	fileInfo, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		rm.ErrorJSON(w, "File not found on disk", http.StatusNotFound)
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		rm.ErrorJSON(w, "Failed to open book file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Serve inline with Content-Disposition: inline
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", fileName))
	w.Header().Set("Content-Type", mime.ByExtension("."+ext))

	// ServeContent handles HTTP Range request headers automatically!
	http.ServeContent(w, r, fileName, fileInfo.ModTime(), file)
}
