package routes

import (
	"database/sql"
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
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/qalibre/qalibre/internal/appdb"
	"github.com/qalibre/qalibre/internal/auth"
	"github.com/qalibre/qalibre/internal/calibredb"
	"github.com/qalibre/qalibre/internal/tasks"
	"github.com/qalibre/qalibre/internal/worker"
)

type MetadataItem struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	ValueType string `json:"value_type"`
}

type DatasetSummary struct {
	ID           int    `json:"id"`
	UUID         string `json:"uuid"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	BookCount    int    `json:"book_count"`
	Created      string `json:"created"`
	LastModified string `json:"last_modified"`
}

type DatasetDetail struct {
	DatasetSummary
	Metadata        []MetadataItem `json:"metadata"`
	ExportDirectory string         `json:"export_directory"`
}

type CreateDatasetRequest struct {
	Name            string         `json:"name"`
	Description     string         `json:"description"`
	Metadata        []MetadataItem `json:"metadata"`
	ExportDirectory string         `json:"export_directory"`
}

type UpdateDatasetRequest struct {
	Name            *string         `json:"name,omitempty"`
	Description     *string         `json:"description,omitempty"`
	Metadata        *[]MetadataItem `json:"metadata,omitempty"` // nil = leave as-is
	ExportDirectory *string         `json:"export_directory,omitempty"`
}

type AddBooksRequest struct {
	BookIDs []int `json:"book_ids"`
}

type ExportRequest struct {
	Path string `json:"path"`
}

type ExportChunkedRequest struct {
	Path         string `json:"path"`
	ChunkSize    int    `json:"chunk_size"`
	ChunkOverlap *int   `json:"chunk_overlap"`
}

// ListDatasets handles GET /api/v1/datasets
func (rm *RouteManager) ListDatasets(w http.ResponseWriter, r *http.Request) {
	search := strings.TrimSpace(r.URL.Query().Get("search"))

	type dbDatasetSummary struct {
		ID           int       `db:"id"`
		UUID         string    `db:"uuid"`
		Name         string    `db:"name"`
		Description  string    `db:"description"`
		BookCount    int       `db:"book_count"`
		Created      time.Time `db:"created"`
		LastModified time.Time `db:"last_modified"`
	}

	var summaries []dbDatasetSummary
	var err error
	if search != "" {
		err = rm.DB.Select(&summaries, `
			SELECT d.id, d.uuid, d.name, d.description, d.created, d.last_modified,
				(SELECT COUNT(*) FROM dataset_book db WHERE db.dataset_id = d.id) AS book_count
			FROM dataset d
			WHERE d.name LIKE ? OR d.description LIKE ?
			ORDER BY d.last_modified DESC`, "%"+search+"%", "%"+search+"%")
	} else {
		err = rm.DB.Select(&summaries, `
			SELECT d.id, d.uuid, d.name, d.description, d.created, d.last_modified,
				(SELECT COUNT(*) FROM dataset_book db WHERE db.dataset_id = d.id) AS book_count
			FROM dataset d
			ORDER BY d.last_modified DESC`)
	}
	if err != nil {
		slog.Error("ListDatasets: failed to query datasets", "err", err)
		rm.ErrorJSON(w, "Failed to retrieve datasets", http.StatusInternalServerError)
		return
	}

	result := make([]DatasetSummary, len(summaries))
	for i, s := range summaries {
		result[i] = DatasetSummary{
			ID:           s.ID,
			UUID:         s.UUID,
			Name:         s.Name,
			Description:  s.Description,
			BookCount:    s.BookCount,
			Created:      s.Created.Format("2006-01-02 15:04:05"),
			LastModified: s.LastModified.Format("2006-01-02 15:04:05"),
		}
	}
	rm.WriteJSON(w, result)
}

// CreateDataset handles POST /api/v1/datasets
func (rm *RouteManager) CreateDataset(w http.ResponseWriter, r *http.Request) {
	var payload CreateDatasetRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		rm.ErrorJSON(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	payload.Name = strings.TrimSpace(payload.Name)
	if payload.Name == "" {
		rm.ErrorJSON(w, "Dataset name is required", http.StatusBadRequest)
		return
	}

	payload.ExportDirectory = strings.TrimSpace(payload.ExportDirectory)
	if payload.ExportDirectory != "" {
		if err := validateExportDirectory(payload.ExportDirectory); err != nil {
			rm.ErrorJSON(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	user, ok := auth.GetUserFromContext(r)
	var userID sql.NullInt64
	if ok {
		appUser := user.(*appdb.User)
		userID = sql.NullInt64{Int64: int64(appUser.ID), Valid: true}
	}

	uuidStr := uuid.NewString()

	tx, err := rm.DB.Beginx()
	if err != nil {
		rm.ErrorJSON(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	res, err := tx.Exec(`
		INSERT INTO dataset (uuid, name, description, export_directory, user_id, created, last_modified)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		uuidStr, payload.Name, payload.Description, payload.ExportDirectory, userID)
	if err != nil {
		rm.ErrorJSON(w, "Failed to insert dataset", http.StatusInternalServerError)
		return
	}

	datasetID, err := res.LastInsertId()
	if err != nil {
		rm.ErrorJSON(w, "Database error", http.StatusInternalServerError)
		return
	}

	for idx, m := range payload.Metadata {
		m.ValueType = strings.ToLower(strings.TrimSpace(m.ValueType))
		if m.ValueType != "string" && m.ValueType != "number" && m.ValueType != "timestamp" && m.ValueType != "path" {
			rm.ErrorJSON(w, "Invalid metadata value_type: "+m.ValueType, http.StatusBadRequest)
			return
		}

		_, err = tx.Exec(`
			INSERT INTO dataset_metadata (dataset_id, key, value, value_type, sort_order)
			VALUES (?, ?, ?, ?, ?)`,
			datasetID, m.Key, m.Value, m.ValueType, idx)
		if err != nil {
			rm.ErrorJSON(w, "Failed to insert metadata", http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		rm.ErrorJSON(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	rm.sendDatasetDetail(w, int(datasetID))
}

// GetDataset handles GET /api/v1/dataset/{id}
func (rm *RouteManager) GetDataset(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		rm.ErrorJSON(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	rm.sendDatasetDetail(w, id)
}

// UpdateDataset handles PATCH /api/v1/dataset/{id}
func (rm *RouteManager) UpdateDataset(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		rm.ErrorJSON(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	var payload UpdateDatasetRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		rm.ErrorJSON(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	var count int
	err = rm.DB.Get(&count, "SELECT COUNT(*) FROM dataset WHERE id = ?", id)
	if err != nil || count == 0 {
		rm.ErrorJSON(w, "Dataset not found", http.StatusNotFound)
		return
	}

	tx, err := rm.DB.Beginx()
	if err != nil {
		rm.ErrorJSON(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	if payload.Name != nil {
		*payload.Name = strings.TrimSpace(*payload.Name)
		if *payload.Name == "" {
			rm.ErrorJSON(w, "Dataset name cannot be empty", http.StatusBadRequest)
			return
		}
		_, err = tx.Exec("UPDATE dataset SET name = ?, last_modified = CURRENT_TIMESTAMP WHERE id = ?", *payload.Name, id)
		if err != nil {
			rm.ErrorJSON(w, "Failed to update dataset name", http.StatusInternalServerError)
			return
		}
	}

	if payload.Description != nil {
		_, err = tx.Exec("UPDATE dataset SET description = ?, last_modified = CURRENT_TIMESTAMP WHERE id = ?", *payload.Description, id)
		if err != nil {
			rm.ErrorJSON(w, "Failed to update dataset description", http.StatusInternalServerError)
			return
		}
	}

	if payload.ExportDirectory != nil {
		dir := strings.TrimSpace(*payload.ExportDirectory)
		if dir != "" {
			if err := validateExportDirectory(dir); err != nil {
				rm.ErrorJSON(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		_, err = tx.Exec("UPDATE dataset SET export_directory = ?, last_modified = CURRENT_TIMESTAMP WHERE id = ?", dir, id)
		if err != nil {
			rm.ErrorJSON(w, "Failed to update dataset export directory", http.StatusInternalServerError)
			return
		}
	}

	if payload.Metadata != nil {
		_, err = tx.Exec("DELETE FROM dataset_metadata WHERE dataset_id = ?", id)
		if err != nil {
			rm.ErrorJSON(w, "Failed to delete old metadata", http.StatusInternalServerError)
			return
		}

		for idx, m := range *payload.Metadata {
			m.ValueType = strings.ToLower(strings.TrimSpace(m.ValueType))
			if m.ValueType != "string" && m.ValueType != "number" && m.ValueType != "timestamp" && m.ValueType != "path" {
				rm.ErrorJSON(w, "Invalid metadata value_type: "+m.ValueType, http.StatusBadRequest)
				return
			}

			_, err = tx.Exec(`
				INSERT INTO dataset_metadata (dataset_id, key, value, value_type, sort_order)
				VALUES (?, ?, ?, ?, ?)`,
				id, m.Key, m.Value, m.ValueType, idx)
			if err != nil {
				rm.ErrorJSON(w, "Failed to insert metadata", http.StatusInternalServerError)
				return
			}
		}

		_, err = tx.Exec("UPDATE dataset SET last_modified = CURRENT_TIMESTAMP WHERE id = ?", id)
		if err != nil {
			rm.ErrorJSON(w, "Failed to update dataset last_modified timestamp", http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		rm.ErrorJSON(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	rm.sendDatasetDetail(w, id)
}

// DeleteDataset handles DELETE /api/v1/dataset/{id}
func (rm *RouteManager) DeleteDataset(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		rm.ErrorJSON(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	var count int
	err = rm.DB.Get(&count, "SELECT COUNT(*) FROM dataset WHERE id = ?", id)
	if err != nil || count == 0 {
		rm.ErrorJSON(w, "Dataset not found", http.StatusNotFound)
		return
	}

	tx, err := rm.DB.Beginx()
	if err != nil {
		rm.ErrorJSON(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	_, _ = tx.Exec("DELETE FROM dataset_book WHERE dataset_id = ?", id)
	_, _ = tx.Exec("DELETE FROM dataset_metadata WHERE dataset_id = ?", id)
	_, _ = tx.Exec("DELETE FROM dataset WHERE id = ?", id)

	if err := tx.Commit(); err != nil {
		rm.ErrorJSON(w, "Failed to delete dataset", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListDatasetBooks handles GET /api/v1/dataset/{id}/books
func (rm *RouteManager) ListDatasetBooks(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	datasetID, err := strconv.Atoi(idStr)
	if err != nil {
		rm.ErrorJSON(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	user, ok := auth.GetUserFromContext(r)
	if !ok {
		rm.ErrorJSON(w, "Login required", http.StatusUnauthorized)
		return
	}
	u := user.(*appdb.User)

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = rm.Cfg.Get().ConfigBooksPerPage
	}
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	sortCol := strings.TrimSpace(r.URL.Query().Get("sort"))
	order := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("order")))

	allowedSorts := map[string]string{
		"id":           "b.id",
		"title":        "b.title",
		"sort":         "b.sort",
		"author_sort":  "b.author_sort",
		"series_index": "b.series_index",
		"timestamp":    "b.timestamp",
		"pubdate":      "b.pubdate",
	}
	sortSQL := "b.timestamp"
	orderSQL := "DESC"
	if s, ok := allowedSorts[sortCol]; ok {
		sortSQL = s
		orderSQL = "ASC"
		if order == "desc" {
			orderSQL = "DESC"
		}
	} else if order == "asc" {
		orderSQL = "ASC"
	}

	whereClauses := []string{"1=1"}
	var queryArgs []interface{}

	if u.DefaultLanguage != "all" {
		whereClauses = append(whereClauses, `
			b.id IN (
				SELECT l.book FROM calibre.books_languages_link l 
				JOIN calibre.languages lang ON l.lang_code = lang.id 
				WHERE lang.lang_code = ?
			)`)
		queryArgs = append(queryArgs, u.DefaultLanguage)
	}

	if u.AllowedTags != "" {
		tags := strings.Split(u.AllowedTags, ",")
		whereClauses = append(whereClauses, `
			b.id IN (
				SELECT l.book FROM calibre.books_tags_link l 
				JOIN calibre.tags t ON l.tag = t.id 
				WHERE t.name IN (?)
			)`)
		queryArgs = append(queryArgs, tags)
	}

	if u.DeniedTags != "" {
		tags := strings.Split(u.DeniedTags, ",")
		whereClauses = append(whereClauses, `
			b.id NOT IN (
				SELECT l.book FROM calibre.books_tags_link l 
				JOIN calibre.tags t ON l.tag = t.id 
				WHERE t.name IN (?)
			)`)
		queryArgs = append(queryArgs, tags)
	}

	allowShowArchived := sortCol == "state"
	if !allowShowArchived {
		whereClauses = append(whereClauses, `
			b.id NOT IN (
				SELECT book_id FROM archived_book 
				WHERE user_id = ? AND is_archived = 1
			)`)
		queryArgs = append(queryArgs, u.ID)
	}

	if search != "" {
		whereClauses = append(whereClauses, `(
			b.title LIKE ? OR 
			b.author_sort LIKE ? OR
			b.id IN (
				SELECT l.book FROM calibre.books_authors_link l 
				JOIN calibre.authors a ON l.author = a.id 
				WHERE a.name LIKE ?
			) OR
			b.id IN (
				SELECT book FROM calibre.comments WHERE text LIKE ?
			)
		)`)
		term := "%" + search + "%"
		queryArgs = append(queryArgs, term, term, term, term)
	}

	whereClauses = append(whereClauses, "b.id IN (SELECT book_id FROM dataset_book WHERE dataset_id = ?)")
	queryArgs = append(queryArgs, datasetID)

	whereSQL := strings.Join(whereClauses, " AND ")
	var finalQueryArgs []interface{}
	expandedWhereSQL := whereSQL

	if strings.Contains(whereSQL, "(?)") {
		expandedWhereSQL, finalQueryArgs, err = sqlx.In(whereSQL, queryArgs...)
		if err != nil {
			rm.ErrorJSON(w, "Search helper error", http.StatusInternalServerError)
			return
		}
	} else {
		finalQueryArgs = queryArgs
	}

	expandedWhereSQL = rm.DB.Rebind(expandedWhereSQL)

	var totalNotFiltered int
	_ = rm.DB.Get(&totalNotFiltered, "SELECT COUNT(*) FROM dataset_book WHERE dataset_id = ?", datasetID)

	var total int
	err = rm.DB.Get(&total, "SELECT COUNT(*) FROM calibre.books b WHERE "+expandedWhereSQL, finalQueryArgs...)
	if err != nil {
		slog.Error("ListDatasetBooks count failed", "err", err)
		rm.WriteJSON(w, map[string]interface{}{"totalNotFiltered": totalNotFiltered, "total": 0, "rows": []interface{}{}})
		return
	}

	limitOffsetSQL := fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
	booksQuery := fmt.Sprintf("SELECT b.* FROM calibre.books b WHERE %s ORDER BY %s %s %s", expandedWhereSQL, sortSQL, orderSQL, limitOffsetSQL)

	var booksList []calibredb.Book
	err = rm.DB.Select(&booksList, booksQuery, finalQueryArgs...)
	if err != nil {
		slog.Error("ListDatasetBooks select failed", "err", err)
		rm.ErrorJSON(w, "Failed to retrieve books", http.StatusInternalServerError)
		return
	}

	rows := make([]BookResponse, len(booksList))
	for i, b := range booksList {
		rows[i], _ = rm.buildBookResponse(b, u)
	}

	rm.WriteJSON(w, map[string]interface{}{
		"totalNotFiltered": totalNotFiltered,
		"total":            total,
		"rows":             rows,
	})
}

// ListAvailableBooks handles GET /api/v1/dataset/{id}/available-books
func (rm *RouteManager) ListAvailableBooks(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	datasetID, err := strconv.Atoi(idStr)
	if err != nil {
		rm.ErrorJSON(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	user, ok := auth.GetUserFromContext(r)
	if !ok {
		rm.ErrorJSON(w, "Login required", http.StatusUnauthorized)
		return
	}
	u := user.(*appdb.User)

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = rm.Cfg.Get().ConfigBooksPerPage
	}
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	sortCol := strings.TrimSpace(r.URL.Query().Get("sort"))
	order := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("order")))

	allowedSorts := map[string]string{
		"id":           "b.id",
		"title":        "b.title",
		"sort":         "b.sort",
		"author_sort":  "b.author_sort",
		"series_index": "b.series_index",
		"timestamp":    "b.timestamp",
		"pubdate":      "b.pubdate",
	}
	sortSQL := "b.timestamp"
	orderSQL := "DESC"
	if s, ok := allowedSorts[sortCol]; ok {
		sortSQL = s
		orderSQL = "ASC"
		if order == "desc" {
			orderSQL = "DESC"
		}
	} else if order == "asc" {
		orderSQL = "ASC"
	}

	whereClauses := []string{"1=1"}
	var queryArgs []interface{}

	if u.DefaultLanguage != "all" {
		whereClauses = append(whereClauses, `
			b.id IN (
				SELECT l.book FROM calibre.books_languages_link l 
				JOIN calibre.languages lang ON l.lang_code = lang.id 
				WHERE lang.lang_code = ?
			)`)
		queryArgs = append(queryArgs, u.DefaultLanguage)
	}

	if u.AllowedTags != "" {
		tags := strings.Split(u.AllowedTags, ",")
		whereClauses = append(whereClauses, `
			b.id IN (
				SELECT l.book FROM calibre.books_tags_link l 
				JOIN calibre.tags t ON l.tag = t.id 
				WHERE t.name IN (?)
			)`)
		queryArgs = append(queryArgs, tags)
	}

	if u.DeniedTags != "" {
		tags := strings.Split(u.DeniedTags, ",")
		whereClauses = append(whereClauses, `
			b.id NOT IN (
				SELECT l.book FROM calibre.books_tags_link l 
				JOIN calibre.tags t ON l.tag = t.id 
				WHERE t.name IN (?)
			)`)
		queryArgs = append(queryArgs, tags)
	}

	allowShowArchived := sortCol == "state"
	if !allowShowArchived {
		whereClauses = append(whereClauses, `
			b.id NOT IN (
				SELECT book_id FROM archived_book 
				WHERE user_id = ? AND is_archived = 1
			)`)
		queryArgs = append(queryArgs, u.ID)
	}

	if search != "" {
		whereClauses = append(whereClauses, `(
			b.title LIKE ? OR 
			b.author_sort LIKE ? OR
			b.id IN (
				SELECT l.book FROM calibre.books_authors_link l 
				JOIN calibre.authors a ON l.author = a.id 
				WHERE a.name LIKE ?
			) OR
			b.id IN (
				SELECT book FROM calibre.comments WHERE text LIKE ?
			)
		)`)
		term := "%" + search + "%"
		queryArgs = append(queryArgs, term, term, term, term)
	}

	whereClauses = append(whereClauses, "b.id NOT IN (SELECT book_id FROM dataset_book WHERE dataset_id = ?)")
	queryArgs = append(queryArgs, datasetID)

	whereSQL := strings.Join(whereClauses, " AND ")
	var finalQueryArgs []interface{}
	expandedWhereSQL := whereSQL

	if strings.Contains(whereSQL, "(?)") {
		expandedWhereSQL, finalQueryArgs, err = sqlx.In(whereSQL, queryArgs...)
		if err != nil {
			rm.ErrorJSON(w, "Search helper error", http.StatusInternalServerError)
			return
		}
	} else {
		finalQueryArgs = queryArgs
	}

	expandedWhereSQL = rm.DB.Rebind(expandedWhereSQL)

	var totalNotFiltered int
	_ = rm.DB.Get(&totalNotFiltered, "SELECT COUNT(*) FROM calibre.books WHERE id NOT IN (SELECT book_id FROM dataset_book WHERE dataset_id = ?)", datasetID)

	var total int
	err = rm.DB.Get(&total, "SELECT COUNT(*) FROM calibre.books b WHERE "+expandedWhereSQL, finalQueryArgs...)
	if err != nil {
		slog.Error("ListAvailableBooks count failed", "err", err)
		rm.WriteJSON(w, map[string]interface{}{"totalNotFiltered": totalNotFiltered, "total": 0, "rows": []interface{}{}})
		return
	}

	limitOffsetSQL := fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
	booksQuery := fmt.Sprintf("SELECT b.* FROM calibre.books b WHERE %s ORDER BY %s %s %s", expandedWhereSQL, sortSQL, orderSQL, limitOffsetSQL)

	var booksList []calibredb.Book
	err = rm.DB.Select(&booksList, booksQuery, finalQueryArgs...)
	if err != nil {
		slog.Error("ListAvailableBooks select failed", "err", err)
		rm.ErrorJSON(w, "Failed to retrieve books", http.StatusInternalServerError)
		return
	}

	rows := make([]BookResponse, len(booksList))
	for i, b := range booksList {
		rows[i], _ = rm.buildBookResponse(b, u)
	}

	rm.WriteJSON(w, map[string]interface{}{
		"totalNotFiltered": totalNotFiltered,
		"total":            total,
		"rows":             rows,
	})
}

// AddBooksToDataset handles POST /api/v1/dataset/{id}/books
func (rm *RouteManager) AddBooksToDataset(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	datasetID, err := strconv.Atoi(idStr)
	if err != nil {
		rm.ErrorJSON(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	var dCount int
	err = rm.DB.Get(&dCount, "SELECT COUNT(*) FROM dataset WHERE id = ?", datasetID)
	if err != nil || dCount == 0 {
		rm.ErrorJSON(w, "Dataset not found", http.StatusNotFound)
		return
	}

	var payload AddBooksRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		rm.ErrorJSON(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	added := 0
	skipped := 0

	for _, bookID := range payload.BookIDs {
		var bCount int
		err = rm.DB.Get(&bCount, "SELECT COUNT(*) FROM calibre.books WHERE id = ?", bookID)
		if err != nil || bCount == 0 {
			skipped++
			continue
		}

		res, err := rm.DB.Exec(`
			INSERT OR IGNORE INTO dataset_book (dataset_id, book_id, sort_order)
			VALUES (?, ?, (SELECT COALESCE(MAX(sort_order), 0) + 1 FROM dataset_book WHERE dataset_id = ?))`,
			datasetID, bookID, datasetID)
		if err != nil {
			skipped++
			continue
		}

		rowsAffected, err := res.RowsAffected()
		if err == nil && rowsAffected > 0 {
			added++
		} else {
			skipped++
		}
	}

	rm.WriteJSON(w, map[string]int{
		"added":   added,
		"skipped": skipped,
	})
}

// RemoveBookFromDataset handles DELETE /api/v1/dataset/{id}/books/{bookId}
func (rm *RouteManager) RemoveBookFromDataset(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	datasetID, err := strconv.Atoi(idStr)
	if err != nil {
		rm.ErrorJSON(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	bookIDStr := chi.URLParam(r, "bookId")
	bookID, err := strconv.Atoi(bookIDStr)
	if err != nil {
		rm.ErrorJSON(w, "Invalid book ID format", http.StatusBadRequest)
		return
	}

	_, err = rm.DB.Exec("DELETE FROM dataset_book WHERE dataset_id = ? AND book_id = ?", datasetID, bookID)
	if err != nil {
		rm.ErrorJSON(w, "Failed to remove book", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ExportDataset handles POST /api/v1/dataset/{id}/export
func (rm *RouteManager) ExportDataset(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	datasetID, err := strconv.Atoi(idStr)
	if err != nil {
		rm.ErrorJSON(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	var dCount int
	err = rm.DB.Get(&dCount, "SELECT COUNT(*) FROM dataset WHERE id = ?", datasetID)
	if err != nil || dCount == 0 {
		rm.ErrorJSON(w, "Dataset not found", http.StatusNotFound)
		return
	}

	var payload ExportRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		rm.ErrorJSON(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	payload.Path = strings.TrimSpace(payload.Path)
	if payload.Path == "" {
		rm.ErrorJSON(w, "Export path is required", http.StatusBadRequest)
		return
	}

	if !filepath.IsAbs(payload.Path) {
		rm.ErrorJSON(w, "Export path must be an absolute path", http.StatusBadRequest)
		return
	}

	err = os.MkdirAll(payload.Path, 0755)
	if err != nil {
		rm.ErrorJSON(w, fmt.Sprintf("Failed to create export path: %v", err), http.StatusBadRequest)
		return
	}

	testFile := filepath.Join(payload.Path, fmt.Sprintf(".export_test_%d", time.Now().UnixNano()))
	err = os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		rm.ErrorJSON(w, fmt.Sprintf("Export path is not writable: %v", err), http.StatusBadRequest)
		return
	}
	_ = os.Remove(testFile)

	user, ok := auth.GetUserFromContext(r)
	var username string
	var userID int
	if ok {
		appUser := user.(*appdb.User)
		username = appUser.Name
		userID = appUser.ID
	}

	task := &tasks.TaskExportDataset{
		DatasetID:  datasetID,
		ExportPath: payload.Path,
		UserID:     userID,
		Cfg:        rm.Cfg,
	}

	taskID := worker.GetInstance(rm.DB).AddTask(username, task)
	w.WriteHeader(http.StatusAccepted)
	rm.WriteJSON(w, map[string]string{"task_id": taskID})
}

func (rm *RouteManager) sendDatasetDetail(w http.ResponseWriter, datasetID int) {
	var d appdb.Dataset
	err := rm.DB.Get(&d, "SELECT * FROM dataset WHERE id = ?", datasetID)
	if err != nil {
		rm.ErrorJSON(w, "Dataset not found", http.StatusNotFound)
		return
	}

	var metadata []appdb.DatasetMetadata
	err = rm.DB.Select(&metadata, "SELECT * FROM dataset_metadata WHERE dataset_id = ? ORDER BY sort_order", datasetID)
	if err != nil {
		slog.Error("sendDatasetDetail: failed to load metadata", "err", err)
	}

	var bookCount int
	_ = rm.DB.Get(&bookCount, "SELECT COUNT(*) FROM dataset_book WHERE dataset_id = ?", datasetID)

	metaItems := make([]MetadataItem, len(metadata))
	for i, m := range metadata {
		metaItems[i] = MetadataItem{
			Key:       m.Key,
			Value:     m.Value,
			ValueType: m.ValueType,
		}
	}

	detail := DatasetDetail{
		DatasetSummary: DatasetSummary{
			ID:           d.ID,
			UUID:         d.UUID,
			Name:         d.Name,
			Description:  d.Description,
			BookCount:    bookCount,
			Created:      d.Created.Format("2006-01-02 15:04:05"),
			LastModified: d.LastModified.Format("2006-01-02 15:04:05"),
		},
		Metadata:        metaItems,
		ExportDirectory: d.ExportDirectory,
	}

	rm.WriteJSON(w, detail)
}

func validateExportDirectory(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("export directory must be an absolute path")
	}
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}
	testFile := filepath.Join(path, fmt.Sprintf(".export_test_%d", time.Now().UnixNano()))
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("export directory is not writable: %w", err)
	}
	_ = os.Remove(testFile)
	return nil
}

// ExportDatasetChunked handles POST /api/v1/dataset/{id}/export-chunked
func (rm *RouteManager) ExportDatasetChunked(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	datasetID, err := strconv.Atoi(idStr)
	if err != nil {
		rm.ErrorJSON(w, "Invalid ID format", http.StatusBadRequest)
		return
	}

	var d appdb.Dataset
	err = rm.DB.Get(&d, "SELECT * FROM dataset WHERE id = ?", datasetID)
	if err != nil {
		rm.ErrorJSON(w, "Dataset not found", http.StatusNotFound)
		return
	}

	var payload ExportChunkedRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		rm.ErrorJSON(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	path := strings.TrimSpace(payload.Path)
	if path == "" {
		path = strings.TrimSpace(d.ExportDirectory)
	}
	if path == "" {
		rm.ErrorJSON(w, "No export directory configured: set export_directory on the dataset or provide path in the request", http.StatusBadRequest)
		return
	}

	if err := validateExportDirectory(path); err != nil {
		rm.ErrorJSON(w, err.Error(), http.StatusBadRequest)
		return
	}

	chunkSize := payload.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 768
	} else if chunkSize > 100000 {
		chunkSize = 100000
	}

	chunkOverlap := 80
	if payload.ChunkOverlap != nil {
		chunkOverlap = *payload.ChunkOverlap
	}
	if chunkOverlap < 0 {
		chunkOverlap = 0
	}

	if chunkOverlap >= chunkSize {
		rm.ErrorJSON(w, "chunk_overlap must be less than chunk_size", http.StatusBadRequest)
		return
	}

	user, ok := auth.GetUserFromContext(r)
	var username string
	var userID int
	if ok {
		appUser := user.(*appdb.User)
		username = appUser.Name
		userID = appUser.ID
	}

	task := &tasks.TaskExportDatasetChunked{
		DatasetID:    datasetID,
		ExportPath:   path,
		ChunkSize:    chunkSize,
		ChunkOverlap: chunkOverlap,
		UserID:       userID,
		Cfg:          rm.Cfg,
	}

	taskID := worker.GetInstance(rm.DB).AddTask(username, task)
	w.WriteHeader(http.StatusAccepted)
	rm.WriteJSON(w, map[string]string{"task_id": taskID})
}
