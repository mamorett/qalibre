package routes

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/qalibre/qalibre/internal/appdb"
	"github.com/qalibre/qalibre/internal/auth"
	"github.com/qalibre/qalibre/internal/books"
	"github.com/qalibre/qalibre/internal/calibredb"
	"github.com/qalibre/qalibre/internal/config"
)

// EditBook handles PATCH /api/v1/book/{id}
func (rm *RouteManager) EditBook(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		rm.ErrorJSON(w, "Invalid book ID", http.StatusBadRequest)
		return
	}

	user, ok := auth.GetUserFromContext(r)
	if !ok {
		rm.ErrorJSON(w, "Login required", http.StatusUnauthorized)
		return
	}
	u := user.(*appdb.User)

	if !auth.HasRole(u.Role, 1<<3) && !auth.HasRole(u.Role, 1<<0) { // RoleEdit (1<<3) or RoleAdmin (1<<0)
		rm.ErrorJSON(w, "Permission denied", http.StatusForbidden)
		return
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		rm.ErrorJSON(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Make direct metadata updates
	err = books.EditBookMetadata(rm.DB, id, updates)
	if err != nil {
		slog.Error("failed to update book metadata", "err", err)
		rm.ErrorJSON(w, "Failed to update book metadata: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return updated book JSON
	var book calibredb.Book
	err = rm.DB.Get(&book, "SELECT * FROM calibre.books WHERE id = ? LIMIT 1", id)
	if err != nil {
		rm.ErrorJSON(w, "Book not found", http.StatusNotFound)
		return
	}

	resp, err := rm.buildBookResponse(book, u)
	if err != nil {
		rm.ErrorJSON(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	rm.WriteJSON(w, resp)
}

// ToggleRead handles POST /ajax/toggleread/{id}
func (rm *RouteManager) ToggleRead(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		rm.ErrorJSON(w, "Invalid book ID", http.StatusBadRequest)
		return
	}

	user, ok := auth.GetUserFromContext(r)
	if !ok {
		rm.ErrorJSON(w, "Login required", http.StatusUnauthorized)
		return
	}
	u := user.(*appdb.User)

	cfg := rm.Cfg.Get()

	if cfg.ConfigReadColumn == 0 {
		// Toggle standard app.db read link status
		var readLink appdb.ReadBook
		err = rm.DB.Get(&readLink, "SELECT * FROM book_read_link WHERE user_id = ? AND book_id = ? LIMIT 1", u.ID, id)
		if err != nil {
			// Insert new unread/in-progress to finished
			_, _ = rm.DB.Exec(`
				INSERT INTO book_read_link (book_id, user_id, read_status, last_modified, times_started_reading)
				VALUES (?, ?, ?, ?, ?)`, id, u.ID, appdb.ReadStatusFinished, time.Now(), 1)
		} else {
			// Cycle status: finished (1) -> unread (0) -> finished (1)
			newStatus := appdb.ReadStatusFinished
			if readLink.ReadStatus == appdb.ReadStatusFinished {
				newStatus = appdb.ReadStatusUnread
			}
			_, _ = rm.DB.Exec(`
				UPDATE book_read_link SET read_status = ?, last_modified = ? 
				WHERE id = ?`, newStatus, time.Now(), readLink.ID)
		}
	} else {
		// Toggle custom column boolean status
		colID := cfg.ConfigReadColumn
		var ccVal string
		err = rm.DB.Get(&ccVal, fmt.Sprintf("SELECT value FROM calibre.custom_column_%d WHERE book = ? LIMIT 1", colID), id)
		newVal := "1"
		if err == nil && (ccVal == "1" || strings.ToLower(ccVal) == "true") {
			newVal = "0"
		}
		_, _ = rm.DB.Exec(fmt.Sprintf("DELETE FROM calibre.custom_column_%d WHERE book = ?", colID), id)
		_, _ = rm.DB.Exec(fmt.Sprintf("INSERT INTO calibre.custom_column_%d (book, value) VALUES (?, ?)", colID), id, newVal)
	}

	rm.WriteJSON(w, map[string]interface{}{"success": true})
}

// ToggleArchived handles POST /ajax/togglearchived/{id}
func (rm *RouteManager) ToggleArchived(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		rm.ErrorJSON(w, "Invalid book ID", http.StatusBadRequest)
		return
	}

	user, ok := auth.GetUserFromContext(r)
	if !ok {
		rm.ErrorJSON(w, "Login required", http.StatusUnauthorized)
		return
	}
	u := user.(*appdb.User)

	var arch appdb.ArchivedBook
	err = rm.DB.Get(&arch, "SELECT * FROM archived_book WHERE user_id = ? AND book_id = ? LIMIT 1", u.ID, id)
	if err != nil {
		// Insert as archived
		_, _ = rm.DB.Exec(`
			INSERT INTO archived_book (user_id, book_id, is_archived, last_modified)
			VALUES (?, ?, 1, ?)`, u.ID, id, time.Now())
	} else {
		// Toggle
		newArch := 1
		if arch.IsArchived {
			newArch = 0
		}
		_, _ = rm.DB.Exec(`
			UPDATE archived_book SET is_archived = ?, last_modified = ? 
			WHERE id = ?`, newArch, time.Now(), arch.ID)
	}

	rm.WriteJSON(w, map[string]interface{}{"success": true})
}

// DeleteBook handles DELETE /api/v1/book/{id}
func (rm *RouteManager) DeleteBook(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		rm.ErrorJSON(w, "Invalid book ID", http.StatusBadRequest)
		return
	}

	user, ok := auth.GetUserFromContext(r)
	if !ok {
		rm.ErrorJSON(w, "Login required", http.StatusUnauthorized)
		return
	}
	u := user.(*appdb.User)

	if !auth.HasRole(u.Role, config.RoleDeleteBooks) && !auth.HasRole(u.Role, config.RoleAdmin) {
		rm.ErrorJSON(w, "Permission denied", http.StatusForbidden)
		return
	}

	// 1. Fetch the book path before deletion
	var path string
	err = rm.DB.Get(&path, "SELECT path FROM calibre.books WHERE id = ? LIMIT 1", id)
	if err != nil {
		rm.ErrorJSON(w, "Book not found", http.StatusNotFound)
		return
	}

	// 2. Start transaction
	tx, err := rm.DB.Beginx()
	if err != nil {
		rm.ErrorJSON(w, "Failed to start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// 3. Delete from Calibre tables
	_, _ = tx.Exec("DELETE FROM calibre.books WHERE id = ?", id)
	_, _ = tx.Exec("DELETE FROM calibre.books_authors_link WHERE book = ?", id)
	_, _ = tx.Exec("DELETE FROM calibre.books_tags_link WHERE book = ?", id)
	_, _ = tx.Exec("DELETE FROM calibre.books_series_link WHERE book = ?", id)
	_, _ = tx.Exec("DELETE FROM calibre.books_publishers_link WHERE book = ?", id)
	_, _ = tx.Exec("DELETE FROM calibre.books_languages_link WHERE book = ?", id)
	_, _ = tx.Exec("DELETE FROM calibre.comments WHERE book = ?", id)
	_, _ = tx.Exec("DELETE FROM calibre.identifiers WHERE book = ?", id)
	_, _ = tx.Exec("DELETE FROM calibre.books_ratings_link WHERE book = ?", id)
	_, _ = tx.Exec("DELETE FROM calibre.data WHERE book = ?", id)

	// Clean up custom columns
	type customCol struct {
		ID int `db:"id"`
	}
	var cols []customCol
	_ = tx.Select(&cols, "SELECT id FROM calibre.custom_columns")
	for _, col := range cols {
		_, _ = tx.Exec(fmt.Sprintf("DELETE FROM calibre.books_custom_column_%d_link WHERE book = ?", col.ID), id)
		_, _ = tx.Exec(fmt.Sprintf("DELETE FROM calibre.custom_column_%d WHERE book = ?", col.ID), id)
	}

	// 4. Delete from app.db tables
	_, _ = tx.Exec("DELETE FROM dataset_book WHERE book_id = ?", id)
	_, _ = tx.Exec("DELETE FROM book_read_link WHERE book_id = ?", id)
	_, _ = tx.Exec("DELETE FROM archived_book WHERE book_id = ?", id)
	_, _ = tx.Exec("DELETE FROM calibre.metadata_dirtied WHERE book = ?", id)

	// Commit Transaction
	err = tx.Commit()
	if err != nil {
		rm.ErrorJSON(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	// 5. Delete physical files from disk
	if path != "" {
		calibreDir := rm.Cfg.GetCalibreDir()
		fullPath := filepath.Join(calibreDir, path)
		
		absCalibre, err1 := filepath.Abs(calibreDir)
		absFull, err2 := filepath.Abs(fullPath)
		if err1 == nil && err2 == nil && strings.HasPrefix(absFull, absCalibre) {
			slog.Info("deleting book folder on disk", "path", fullPath)
			_ = os.RemoveAll(fullPath)
			
			authorDir := filepath.Dir(fullPath)
			if files, err := os.ReadDir(authorDir); err == nil && len(files) == 0 {
				_ = os.Remove(authorDir)
			}
		} else {
			slog.Warn("prevented deletion of path outside calibre directory", "path", fullPath)
		}
	}

	rm.WriteJSON(w, map[string]interface{}{"success": true})
}
