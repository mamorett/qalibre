package routes

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	"github.com/qalibre/qalibre/internal/appdb"
	"github.com/qalibre/qalibre/internal/auth"
	"github.com/qalibre/qalibre/internal/calibredb"
	"github.com/qalibre/qalibre/internal/config"
)

// BookResponse format matching the AlchemyEncoder serialization contract byte-for-byte.
type BookResponse struct {
	ID            int                       `json:"id"`
	Title         string                    `json:"title"`
	Sort          string                    `json:"sort"`
	AuthorSort    string                    `json:"author_sort"`
	Timestamp     string                    `json:"timestamp"`
	Pubdate       string                    `json:"pubdate"`
	SeriesIndex   string                    `json:"series_index"`
	LastModified  string                    `json:"last_modified"`
	Path          string                    `json:"path"`
	HasCover      bool                      `json:"has_cover"`
	UUID          string                    `json:"uuid"`
	Authors       string                    `json:"authors"`
	AuthorsList   []string                  `json:"authors_list"`
	Tags          string                    `json:"tags"`
	TagsList      []string                  `json:"tags_list"`
	Series        *string                   `json:"series"`
	Publisher     *string                   `json:"publisher"`
	Comments      string                    `json:"comments"`
	Formats       []FormatResponse          `json:"formats"`
	Identifiers   []IdentifierResponse      `json:"identifiers"`
	IsArchived    bool                      `json:"is_archived"`
	ReadStatus    bool                      `json:"read_status"`
	Shelves       []int                     `json:"shelves"`
	CustomColumns []CustomColumnValResponse `json:"custom_columns"`
}

type FormatResponse struct {
	ID     int    `json:"id"`
	Format string `json:"format"`
	Size   int64  `json:"size"`
	Name   string `json:"name"`
}

type IdentifierResponse struct {
	Type string `json:"type"`
	Val  string `json:"val"`
}

type CustomColumnValResponse struct {
	ID         int         `json:"id"`
	Label      string      `json:"label"`
	Name       string      `json:"name"`
	DataType   string      `json:"datatype"`
	IsMultiple bool        `json:"is_multiple"`
	Value      interface{} `json:"value"`
}

// GetBook handles GET /api/v1/book/{id}
func (rm *RouteManager) GetBook(w http.ResponseWriter, r *http.Request) {
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

	// Fetch book
	var book calibredb.Book
	err = rm.DB.Get(&book, "SELECT * FROM calibre.books WHERE id = ? LIMIT 1", id)
	if err != nil {
		rm.ErrorJSON(w, "Book not found", http.StatusNotFound)
		return
	}

	// Verify allowed tags or other restrictions (common filters check)
	allowed, err := rm.checkBookAllowed(book.ID, u)
	if err != nil || !allowed {
		rm.ErrorJSON(w, "Book not found", http.StatusNotFound)
		return
	}

	// Build the response
	resp, err := rm.buildBookResponse(book, u)
	if err != nil {
		rm.ErrorJSON(w, "Internal server error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	rm.WriteJSON(w, resp)
}

// checkBookAllowed verifies if a book is allowed based on user blocklists/allowed tags
func (rm *RouteManager) checkBookAllowed(bookID int, u *appdb.User) (bool, error) {
	// If admin, all books allowed
	if auth.HasRole(u.Role, config.RoleAdmin) {
		return true, nil
	}

	cfg := rm.Cfg.Get()

	// Check Archived
	var isArchived int
	_ = rm.DB.Get(&isArchived, "SELECT COUNT(*) FROM archived_book WHERE user_id = ? AND book_id = ? AND is_archived = 1", u.ID, bookID)
	if isArchived > 0 {
		return false, nil
	}

	// Check Language
	if u.DefaultLanguage != "all" {
		var langCount int
		_ = rm.DB.Get(&langCount, `
			SELECT COUNT(*) FROM calibre.books_languages_link l 
			JOIN calibre.languages lang ON l.lang_code = lang.id 
			WHERE l.book = ? AND lang.lang_code = ?`, bookID, u.DefaultLanguage)
		if langCount == 0 {
			return false, nil
		}
	}

	// Allowed Tags Check
	if u.AllowedTags != "" {
		tags := strings.Split(u.AllowedTags, ",")
		var allowedTagCount int
		query, args, _ := sqlx.In(`
			SELECT COUNT(*) FROM calibre.books_tags_link l 
			JOIN calibre.tags t ON l.tag = t.id 
			WHERE l.book = ? AND t.name IN (?)`, bookID, tags)
		query = rm.DB.Rebind(query)
		_ = rm.DB.Get(&allowedTagCount, query, args...)
		if allowedTagCount == 0 {
			return false, nil
		}
	}

	// Denied Tags Check
	if u.DeniedTags != "" {
		tags := strings.Split(u.DeniedTags, ",")
		var deniedTagCount int
		query, args, _ := sqlx.In(`
			SELECT COUNT(*) FROM calibre.books_tags_link l 
			JOIN calibre.tags t ON l.tag = t.id 
			WHERE l.book = ? AND t.name IN (?)`, bookID, tags)
		query = rm.DB.Rebind(query)
		_ = rm.DB.Get(&deniedTagCount, query, args...)
		if deniedTagCount > 0 {
			return false, nil
		}
	}

	// Custom Column checks (restricted column)
	if cfg.ConfigRestrictedColumn > 0 {
		colID := cfg.ConfigRestrictedColumn
		// Allowed Column Values
		if u.AllowedColumnValue != "" {
			vals := strings.Split(u.AllowedColumnValue, ",")
			allowedVal, _ := rm.checkCustomColumnInValues(bookID, colID, vals)
			if !allowedVal {
				return false, nil
			}
		}
		// Denied Column Values
		if u.DeniedColumnValue != "" {
			vals := strings.Split(u.DeniedColumnValue, ",")
			deniedVal, _ := rm.checkCustomColumnInValues(bookID, colID, vals)
			if deniedVal {
				return false, nil
			}
		}
	}

	return true, nil
}

func (rm *RouteManager) checkCustomColumnInValues(bookID int, colID int, vals []string) (bool, error) {
	// Query custom_column_X table
	// We need to resolve column type. Let's assume text or similar
	var datatype string
	_ = rm.DB.Get(&datatype, "SELECT datatype FROM calibre.custom_columns WHERE id = ? LIMIT 1", colID)

	var count int
	var query string
	var args []interface{}
	var err error

	if datatype == "series" || datatype == "text" {
		// linked tables or multiple values
		// We'll check both scalar table and links
		query, args, err = sqlx.In(fmt.Sprintf(`
			SELECT COUNT(*) FROM calibre.books_custom_column_%d_link l
			JOIN calibre.custom_column_%d c ON l.value = c.id
			WHERE l.book = ? AND c.value IN (?)`, colID, colID), bookID, vals)
	} else {
		query, args, err = sqlx.In(fmt.Sprintf(`
			SELECT COUNT(*) FROM calibre.custom_column_%d 
			WHERE book = ? AND value IN (?)`, colID), bookID, vals)
	}

	if err != nil {
		return false, err
	}

	query = rm.DB.Rebind(query)
	err = rm.DB.Get(&count, query, args...)
	return count > 0, err
}

// buildBookResponse constructs a complete BookResponse for a single book.
func (rm *RouteManager) buildBookResponse(book calibredb.Book, u *appdb.User) (BookResponse, error) {
	resp := BookResponse{
		ID:            book.ID,
		Title:         book.Title,
		Sort:          book.Sort,
		AuthorSort:    book.AuthorSort,
		Timestamp:     book.Timestamp.Format("2006-01-02 15:04:05"),
		Pubdate:       book.Pubdate.Format("2006-01-02 15:04:05"),
		SeriesIndex:   book.SeriesIndex,
		LastModified:  book.LastModified.Format("2006-01-02 15:04:05"),
		Path:          book.Path,
		HasCover:      book.HasCover == 1,
		UUID:          book.UUID,
		AuthorsList:   []string{},
		TagsList:      []string{},
		Formats:       []FormatResponse{},
		Identifiers:   []IdentifierResponse{},
		Shelves:       []int{},
		CustomColumns: []CustomColumnValResponse{},
	}

	// Fetch authors
	var authors []string
	_ = rm.DB.Select(&authors, `
		SELECT a.name FROM calibre.books_authors_link l 
		JOIN calibre.authors a ON l.author = a.id 
		WHERE l.book = ? 
		ORDER BY l.id`, book.ID)
	resp.AuthorsList = authors
	resp.Authors = strings.Join(authors, " & ")

	// Fetch tags
	var tags []string
	_ = rm.DB.Select(&tags, `
		SELECT t.name FROM calibre.books_tags_link l 
		JOIN calibre.tags t ON l.tag = t.id 
		WHERE l.book = ? 
		ORDER BY t.name`, book.ID)
	resp.TagsList = tags
	resp.Tags = strings.Join(tags, ",")

	// Fetch series
	var series []string
	_ = rm.DB.Select(&series, `
		SELECT s.name FROM calibre.books_series_link l 
		JOIN calibre.series s ON l.series = s.id 
		WHERE l.book = ? LIMIT 1`, book.ID)
	if len(series) > 0 {
		resp.Series = &series[0]
	}

	// Fetch publisher
	var publishers []string
	_ = rm.DB.Select(&publishers, `
		SELECT p.name FROM calibre.books_publishers_link l 
		JOIN calibre.publishers p ON l.publisher = p.id 
		WHERE l.book = ? LIMIT 1`, book.ID)
	if len(publishers) > 0 {
		resp.Publisher = &publishers[0]
	}

	// Fetch comments
	var comment string
	_ = rm.DB.Get(&comment, "SELECT text FROM calibre.comments WHERE book = ? LIMIT 1", book.ID)
	resp.Comments = comment

	// Fetch formats
	var formats []calibredb.Data
	_ = rm.DB.Select(&formats, "SELECT * FROM calibre.data WHERE book = ?", book.ID)
	for _, f := range formats {
		resp.Formats = append(resp.Formats, FormatResponse{
			ID:     f.ID,
			Format: f.Format,
			Size:   f.UncompressedSize,
			Name:   f.Name,
		})
	}

	// Fetch identifiers
	var identifiers []calibredb.Identifier
	_ = rm.DB.Select(&identifiers, "SELECT * FROM calibre.identifiers WHERE book = ?", book.ID)
	for _, id := range identifiers {
		resp.Identifiers = append(resp.Identifiers, IdentifierResponse{
			Type: id.Type,
			Val:  id.Val,
		})
	}

	// Fetch Archived status
	var archCount int
	_ = rm.DB.Get(&archCount, "SELECT COUNT(*) FROM archived_book WHERE user_id = ? AND book_id = ? AND is_archived = 1", u.ID, book.ID)
	resp.IsArchived = archCount > 0

	// Fetch Read Status
	var readStatus int
	_ = rm.DB.Get(&readStatus, "SELECT read_status FROM book_read_link WHERE user_id = ? AND book_id = ? LIMIT 1", u.ID, book.ID)
	resp.ReadStatus = readStatus == appdb.ReadStatusFinished

	// Fetch shelves
	_ = rm.DB.Select(&resp.Shelves, "SELECT shelf FROM book_shelf_link l JOIN shelf s ON l.shelf = s.id WHERE l.book_id = ? AND (s.user_id = ? OR s.is_public = 1)", book.ID, u.ID)
	if resp.Shelves == nil {
		resp.Shelves = []int{}
	}

	// Fetch custom columns values
	resp.CustomColumns, _ = rm.getCustomColumnValues(book.ID)

	return resp, nil
}

// getCustomColumnValues retrieves all custom columns values for a book
func (rm *RouteManager) getCustomColumnValues(bookID int) ([]CustomColumnValResponse, error) {
	var cols []calibredb.CustomColumnDef
	err := rm.DB.Select(&cols, "SELECT id, label, name, datatype, is_multiple, normalized, display FROM calibre.custom_columns ORDER BY id")
	if err != nil {
		return []CustomColumnValResponse{}, err
	}

	var results []CustomColumnValResponse
	for _, c := range cols {
		if c.DataType == "composite" {
			continue // Skip composites
		}

		var val interface{} = nil

		if c.DataType == "series" || c.DataType == "enumeration" || (c.DataType == "text" && c.IsMultiple) {
			// Linked table values (M:N)
			var vals []string
			err = rm.DB.Select(&vals, fmt.Sprintf(`
				SELECT val.value FROM calibre.books_custom_column_%d_link l
				JOIN calibre.custom_column_%d val ON l.value = val.id
				WHERE l.book = ?`, c.ID, c.ID), bookID)
			if err == nil && len(vals) > 0 {
				if c.IsMultiple {
					val = strings.Join(vals, ", ")
				} else {
					val = vals[0]
				}
			}
		} else {
			// Scalar table (1:1 / 1:N)
			var scalarVal string
			err = rm.DB.Get(&scalarVal, fmt.Sprintf("SELECT value FROM calibre.custom_column_%d WHERE book = ? LIMIT 1", c.ID), bookID)
			if err == nil {
				// Parse based on datatype
				switch c.DataType {
				case "bool":
					val = scalarVal == "1" || strings.ToLower(scalarVal) == "true"
				case "int":
					val, _ = strconv.Atoi(scalarVal)
				case "float":
					val, _ = strconv.ParseFloat(scalarVal, 64)
				default:
					val = scalarVal
				}
			}
		}

		results = append(results, CustomColumnValResponse{
			ID:         c.ID,
			Label:      c.Label,
			Name:       c.Name,
			DataType:   c.DataType,
			IsMultiple: c.IsMultiple,
			Value:      val,
		})
	}

	return results, nil
}

// ListBooks handles GET /ajax/listbooks
func (rm *RouteManager) ListBooks(w http.ResponseWriter, r *http.Request) {
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

	// Validate sort column
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

	// 1. Build Query base with common filters
	whereClauses := []string{"1=1"}
	var queryArgs []interface{}

	// Language filter
	if u.DefaultLanguage != "all" {
		whereClauses = append(whereClauses, `
			b.id IN (
				SELECT l.book FROM calibre.books_languages_link l 
				JOIN calibre.languages lang ON l.lang_code = lang.id 
				WHERE lang.lang_code = ?
			)`)
		queryArgs = append(queryArgs, u.DefaultLanguage)
	}

	// Allowed tags
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

	// Denied tags
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

	// Archived books (do not show archived unless allow_show_archived is requested, e.g. state/archive sort)
	allowShowArchived := sortCol == "state"
	if !allowShowArchived {
		whereClauses = append(whereClauses, `
			b.id NOT IN (
				SELECT book_id FROM archived_book 
				WHERE user_id = ? AND is_archived = 1
			)`)
		queryArgs = append(queryArgs, u.ID)
	}

	// Search filter
	if search != "" {
		// FTS or simple LIKE checks
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

	whereSQL := strings.Join(whereClauses, " AND ")

	// Expand SQL In variables
	var err error
	var finalQueryArgs []interface{}
	expandedWhereSQL := whereSQL

	// Perform sqlx.In replacement for IN clauses if tags/restricted columns are used
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

	// Fetch Totals
	var totalNotFiltered int
	_ = rm.DB.Get(&totalNotFiltered, "SELECT COUNT(*) FROM calibre.books")

	var total int
	err = rm.DB.Get(&total, "SELECT COUNT(*) FROM calibre.books b WHERE "+expandedWhereSQL, finalQueryArgs...)
	if err != nil {
		slog.Error("listbooks count failed", "err", err)
		rm.WriteJSON(w, map[string]interface{}{"totalNotFiltered": totalNotFiltered, "total": 0, "rows": []interface{}{}})
		return
	}

	// Fetch page books
	limitOffsetSQL := fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
	booksQuery := fmt.Sprintf("SELECT b.* FROM calibre.books b WHERE %s ORDER BY %s %s %s", expandedWhereSQL, sortSQL, orderSQL, limitOffsetSQL)

	var books []calibredb.Book
	err = rm.DB.Select(&books, booksQuery, finalQueryArgs...)
	if err != nil {
		slog.Error("listbooks select failed", "err", err)
		rm.ErrorJSON(w, "Failed to retrieve books", http.StatusInternalServerError)
		return
	}

	rows := make([]BookResponse, len(books))
	for i, b := range books {
		rows[i], _ = rm.buildBookResponse(b, u)
	}

	rm.WriteJSON(w, map[string]interface{}{
		"totalNotFiltered": totalNotFiltered,
		"total":            total,
		"rows":             rows,
	})
}
