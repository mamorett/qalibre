package routes

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/qalibre/qalibre/internal/appdb"
	"github.com/qalibre/qalibre/internal/calibredb"
	"github.com/qalibre/qalibre/internal/helper"
)

// ServeCover handles GET /cover/{id} and GET /cover/{id}/{res}
func (rm *RouteManager) ServeCover(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Redirect(w, r, "/static/generic_cover.jpg", http.StatusFound)
		return
	}

	resStr := strings.ToLower(chi.URLParam(r, "res")) // sm, md, lg, or empty (og)

	// Fetch book
	var book calibredb.Book
	err = rm.DB.Get(&book, "SELECT * FROM calibre.books WHERE id = ? LIMIT 1", id)
	if err != nil {
		http.Redirect(w, r, "/static/generic_cover.jpg", http.StatusFound)
		return
	}

	calibreDir := rm.Cfg.GetCalibreDir()
	if calibreDir == "" {
		http.Redirect(w, r, "/static/generic_cover.jpg", http.StatusFound)
		return
	}

	coverPath := filepath.Join(calibreDir, book.Path, "cover.jpg")
	coverExists := false
	if _, err := os.Stat(coverPath); err == nil {
		coverExists = true
	}

	if !coverExists {
		http.Redirect(w, r, "/static/generic_cover.jpg", http.StatusFound)
		return
	}

	// Try serving from cached thumbnails if a specific resolution is requested
	if resStr == "sm" || resStr == "md" || resStr == "lg" {
		resVal := 1 // sm
		if resStr == "md" {
			resVal = 2
		} else if resStr == "lg" {
			resVal = 4
		}

		var thumb appdb.Thumbnail
		err = rm.DB.Get(&thumb, `
			SELECT * FROM thumbnail 
			WHERE entity_id = ? AND resolution = ? AND type = 1
			AND (expiration IS NULL OR expiration > ?) LIMIT 1`, id, resVal, time.Now())
		if err == nil {
			// Locate cached file
			configDir := helper.GetConfigDir()
			thumbPath := filepath.Join(configDir, "cache", "thumbnails", thumb.Filename)
			if _, err := os.Stat(thumbPath); err == nil {
				w.Header().Set("Content-Type", "image/jpeg")
				w.Header().Set("Cache-Control", "public, max-age=2592000") // 30 days
				http.ServeFile(w, r, thumbPath)
				return
			}
		}
	}

	// Serve original cover
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=86400") // 1 day
	http.ServeFile(w, r, coverPath)

	// Final fallback
	http.Redirect(w, r, "/static/generic_cover.jpg", http.StatusFound)
}
