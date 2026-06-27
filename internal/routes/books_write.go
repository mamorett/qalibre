package routes

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/qalibre/qalibre/internal/appdb"
	"github.com/qalibre/qalibre/internal/auth"
	"github.com/qalibre/qalibre/internal/books"
	"github.com/qalibre/qalibre/internal/calibredb"
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
