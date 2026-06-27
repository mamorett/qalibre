package books

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/qalibre/qalibre/internal/calibredb"
	"github.com/qalibre/qalibre/internal/helper"
)

// RenameBookFiles moves/renames the physical files on disk when title or author changes.
// Returns the new relative book path.
func RenameBookFiles(db *sqlx.DB, calibrePath string, bookID int, newTitle string, newAuthor string, unicodeFilename bool) (string, error) {
	// Fetch book path and formats
	var book calibredb.Book
	err := db.Get(&book, "SELECT * FROM calibre.books WHERE id = ? LIMIT 1", bookID)
	if err != nil {
		return "", err
	}

	var formats []calibredb.Data
	_ = db.Select(&formats, "SELECT * FROM calibre.data WHERE book = ?", bookID)

	// Determine new directory paths
	cleanAuthor, _ := helper.GetValidFilename(newAuthor, true, 96, false, unicodeFilename)
	cleanTitle, _ := helper.GetValidFilename(newTitle, true, 96, false, unicodeFilename)
	newTitleDir := fmt.Sprintf("%s (%d)", cleanTitle, bookID)

	newRelPath := filepath.Join(cleanAuthor, newTitleDir)
	newRelPath = strings.ReplaceAll(newRelPath, "\\", "/")

	if book.Path == newRelPath || book.Path == "" {
		return book.Path, nil
	}

	oldPath := filepath.Join(calibrePath, book.Path)
	newPath := filepath.Join(calibrePath, newRelPath)

	slog.Info("renaming book files", "old", oldPath, "new", newPath)

	if _, err := os.Stat(oldPath); err == nil {
		// Create parent directory for new author if needed
		if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
			return "", err
		}

		// Move folder
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			err = os.Rename(oldPath, newPath)
			if err != nil {
				// try copy/delete
				err = copyDir(oldPath, newPath)
				if err != nil {
					return "", fmt.Errorf("failed to move folder: %w", err)
				}
				_ = os.RemoveAll(oldPath)
			}
		} else {
			// Destination exists, merge files
			files, _ := os.ReadDir(oldPath)
			for _, file := range files {
				oldFile := filepath.Join(oldPath, file.Name())
				newFile := filepath.Join(newPath, file.Name())
				_ = os.Rename(oldFile, newFile)
			}
			_ = os.RemoveAll(oldPath)
		}

		// Clean up old empty author directory
		oldAuthorPath := filepath.Dir(oldPath)
		if files, err := os.ReadDir(oldAuthorPath); err == nil && len(files) == 0 {
			_ = os.Remove(oldAuthorPath)
		}
	}

	// Rename formats files inside the new folder to match "Title - Author.ext"
	cleanAuthorShort, _ := helper.GetValidFilename(newAuthor, true, 42, false, unicodeFilename)
	cleanTitleShort, _ := helper.GetValidFilename(newTitle, true, 42, false, unicodeFilename)
	newFormatBaseName := fmt.Sprintf("%s - %s", cleanTitleShort, cleanAuthorShort)

	for _, format := range formats {
		oldFormatFile := filepath.Join(newPath, format.Name+"."+strings.ToLower(format.Format))
		newFormatFile := filepath.Join(newPath, newFormatBaseName+"."+strings.ToLower(format.Format))

		if _, err := os.Stat(oldFormatFile); err == nil {
			_ = os.Rename(oldFormatFile, newFormatFile)
		}

		// Update database format name
		_, _ = db.Exec("UPDATE calibre.data SET name = ? WHERE id = ?", newFormatBaseName, format.ID)
	}

	// Update books path in database
	_, err = db.Exec("UPDATE calibre.books SET path = ? WHERE id = ?", newRelPath, bookID)
	if err != nil {
		return "", err
	}

	return newRelPath, nil
}

// Copy a directory fallback helper
func copyDir(src string, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := os.MkdirAll(dstPath, 0755); err != nil {
				return err
			}
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			input, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			err = os.WriteFile(dstPath, input, 0644)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// EditBookMetadata modifies Calibre DB metadata rows directly (Phase 3 write).
func EditBookMetadata(db *sqlx.DB, bookID int, updates map[string]interface{}) error {
	tx, err := db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Update basic book fields
	var book calibredb.Book
	err = tx.Get(&book, "SELECT * FROM calibre.books WHERE id = ? LIMIT 1", bookID)
	if err != nil {
		return err
	}

	titleChanged := false
	authorChanged := false
	newTitle := book.Title
	newAuthor := ""

	if title, ok := updates["title"].(string); ok && title != "" {
		newTitle = title
		titleChanged = (title != book.Title)
		book.Title = title
		book.Sort = dbutilTitleSort(title) // Call local title sort UDF equivalent
	}

	if seriesIdx, ok := updates["series_index"].(string); ok {
		book.SeriesIndex = seriesIdx
	}

	// Update timestamp and last modified
	book.LastModified = time.Now()

	_, err = tx.NamedExec(`
		UPDATE calibre.books SET 
			title=:title, sort=:sort, series_index=:series_index, last_modified=:last_modified
		WHERE id=:id`, book)
	if err != nil {
		return err
	}

	// 2. Update comments
	if comment, ok := updates["comments"].(string); ok {
		_, _ = tx.Exec("DELETE FROM calibre.comments WHERE book = ?", bookID)
		if comment != "" {
			_, _ = tx.Exec("INSERT INTO calibre.comments (book, text) VALUES (?, ?)", bookID, comment)
		}
	}

	// 3. Update authors
	if authorsRaw, ok := updates["authors"].([]string); ok {
		authorChanged = true
		newAuthor = strings.Join(authorsRaw, " & ")

		// Remove existing links
		_, _ = tx.Exec("DELETE FROM calibre.books_authors_link WHERE book = ?", bookID)

		// Create/Resolve authors
		for _, name := range authorsRaw {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}

			var authorID int
			err = tx.Get(&authorID, "SELECT id FROM calibre.authors WHERE lower(name) = lower(?) LIMIT 1", name)
			if err != nil {
				// Insert new author
				sortName := helper.GetSortedAuthor(name)
				res, err := tx.Exec("INSERT INTO calibre.authors (name, sort, link) VALUES (?, ?, '')", name, sortName)
				if err == nil {
					id, _ := res.LastInsertId()
					authorID = int(id)
				}
			}

			// Add link
			_, _ = tx.Exec("INSERT INTO calibre.books_authors_link (book, author) VALUES (?, ?)", bookID, authorID)
		}

		// Recalculate author_sort on books table
		var authorSorts []string
		_ = tx.Select(&authorSorts, `
			SELECT a.sort FROM calibre.books_authors_link l 
			JOIN calibre.authors a ON l.author = a.id 
			WHERE l.book = ? 
			ORDER BY l.id`, bookID)
		authorSortStr := strings.Join(authorSorts, " & ")
		_, _ = tx.Exec("UPDATE calibre.books SET author_sort = ? WHERE id = ?", authorSortStr, bookID)
	}

	// 4. Update tags
	if tagsRaw, ok := updates["tags"].([]string); ok {
		_, _ = tx.Exec("DELETE FROM calibre.books_tags_link WHERE book = ?", bookID)
		for _, name := range tagsRaw {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}

			var tagID int
			err = tx.Get(&tagID, "SELECT id FROM calibre.tags WHERE lower(name) = lower(?) LIMIT 1", name)
			if err != nil {
				res, err := tx.Exec("INSERT INTO calibre.tags (name) VALUES (?)", name)
				if err == nil {
					id, _ := res.LastInsertId()
					tagID = int(id)
				}
			}
			_, _ = tx.Exec("INSERT INTO calibre.books_tags_link (book, tag) VALUES (?, ?)", bookID, tagID)
		}
	}

	// 5. Update series
	if seriesName, ok := updates["series"].(string); ok {
		_, _ = tx.Exec("DELETE FROM calibre.books_series_link WHERE book = ?", bookID)
		seriesName = strings.TrimSpace(seriesName)
		if seriesName != "" {
			var seriesID int
			err = tx.Get(&seriesID, "SELECT id FROM calibre.series WHERE lower(name) = lower(?) LIMIT 1", seriesName)
			if err != nil {
				res, err := tx.Exec("INSERT INTO calibre.series (name, sort) VALUES (?, ?)", seriesName, seriesName)
				if err == nil {
					id, _ := res.LastInsertId()
					seriesID = int(id)
				}
			}
			_, _ = tx.Exec("INSERT INTO calibre.books_series_link (book, series) VALUES (?, ?)", bookID, seriesID)
		}
	}

	// 6. Update publisher
	if pubName, ok := updates["publisher"].(string); ok {
		_, _ = tx.Exec("DELETE FROM calibre.books_publishers_link WHERE book = ?", bookID)
		pubName = strings.TrimSpace(pubName)
		if pubName != "" {
			var pubID int
			err = tx.Get(&pubID, "SELECT id FROM calibre.publishers WHERE lower(name) = lower(?) LIMIT 1", pubName)
			if err != nil {
				res, err := tx.Exec("INSERT INTO calibre.publishers (name, sort) VALUES (?, ?)", pubName, pubName)
				if err == nil {
					id, _ := res.LastInsertId()
					pubID = int(id)
				}
			}
			_, _ = tx.Exec("INSERT INTO calibre.books_publishers_link (book, publisher) VALUES (?, ?)", bookID, pubID)
		}
	}

	// 7. Update custom columns
	if ccRaw, ok := updates["custom_columns"].(map[string]interface{}); ok {
		for colIDStr, val := range ccRaw {
			colID, _ := strconv.Atoi(colIDStr)
			if colID <= 0 {
				continue
			}

			// Get datatype
			var datatype string
			err = tx.Get(&datatype, "SELECT datatype FROM calibre.custom_columns WHERE id = ? LIMIT 1", colID)
			if err != nil {
				continue
			}

			if datatype == "series" || datatype == "text" {
				// Linked table values (M:N)
				_, _ = tx.Exec(fmt.Sprintf("DELETE FROM calibre.books_custom_column_%d_link WHERE book = ?", colID), bookID)
				if valStr, ok := val.(string); ok && strings.TrimSpace(valStr) != "" {
					parts := strings.Split(valStr, ",")
					for _, part := range parts {
						part = strings.TrimSpace(part)
						if part == "" {
							continue
						}
						var itemID int
						err = tx.Get(&itemID, fmt.Sprintf("SELECT id FROM calibre.custom_column_%d WHERE lower(value) = lower(?) LIMIT 1", colID), part)
						if err != nil {
							res, err := tx.Exec(fmt.Sprintf("INSERT INTO calibre.custom_column_%d (value) VALUES (?)", colID), part)
							if err == nil {
								id, _ := res.LastInsertId()
								itemID = int(id)
							}
						}
						_, _ = tx.Exec(fmt.Sprintf("INSERT INTO calibre.books_custom_column_%d_link (book, value) VALUES (?, ?)", colID), bookID, itemID)
					}
				}
			} else {
				// Scalar table
				_, _ = tx.Exec(fmt.Sprintf("DELETE FROM calibre.custom_column_%d WHERE book = ?", colID), bookID)
				if val != nil {
					valStr := fmt.Sprintf("%v", val)
					if strings.TrimSpace(valStr) != "" {
						_, _ = tx.Exec(fmt.Sprintf("INSERT INTO calibre.custom_column_%d (book, value) VALUES (?, ?)", colID), bookID, valStr)
					}
				}
			}
		}
	}

	// 8. Enqueue metadata_dirtied
	_, _ = tx.Exec("INSERT OR IGNORE INTO calibre.metadata_dirtied (book) VALUES (?)", bookID)

	err = tx.Commit()
	if err != nil {
		return err
	}

	// If title or author changed, rename files on disk (we pass the tx DB handle to perform updates)
	if titleChanged || authorChanged {
		if newAuthor == "" {
			// Fetch author from DB
			var authors []string
			_ = db.Select(&authors, `
				SELECT a.name FROM calibre.books_authors_link l 
				JOIN calibre.authors a ON l.author = a.id 
				WHERE l.book = ? 
				ORDER BY l.id`, bookID)
			newAuthor = strings.Join(authors, " & ")
		}

		// Get unicode filename config setting
		var unicodeFilename bool
		_ = db.Get(&unicodeFilename, "SELECT config_unicode_filename FROM settings WHERE id = 1 LIMIT 1")

		// Get calibre directory
		var calibreDir string
		_ = db.Get(&calibreDir, "SELECT config_calibre_dir FROM settings WHERE id = 1 LIMIT 1")

		if calibreDir != "" {
			_, _ = RenameBookFiles(db, calibreDir, bookID, newTitle, newAuthor, unicodeFilename)
		}
	}

	return nil
}

// Local helper to match leading article sort logic
func dbutilTitleSort(title string) string {
	leadingArticleRe := regexp.MustCompile(`(?i)^(A|The|An|Der|Die|Das|Den|Ein|Eine|Einen|Dem|Des|Einem|Eines|Le|La|Les|L'|Un|Une)[\s']+`)
	m := leadingArticleRe.FindStringIndex(title)
	if m == nil {
		return title
	}
	article := strings.TrimSpace(title[m[0]:m[1]])
	rest := strings.TrimSpace(title[m[1]:])
	return rest + ", " + article
}
