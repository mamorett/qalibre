// Package dbutil handles SQLite database setup: opening app.db, ATTACHing
// metadata.db, registering SQLite UDFs, and providing shared helpers.
// It replaces the ATTACH trick in cps/db.py:685-725.
package dbutil

import (
	"database/sql/driver"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"modernc.org/sqlite"
)

func init() {
	// Register lower UDF
	err := sqlite.RegisterFunction("lower", &sqlite.FunctionImpl{
		NArgs:         1,
		Deterministic: true,
		Scalar: func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			if len(args) == 0 || args[0] == nil {
				return nil, nil
			}
			s, ok := args[0].(string)
			if !ok {
				if b, ok := args[0].([]byte); ok {
					s = string(b)
				} else {
					return nil, fmt.Errorf("lower: invalid argument type")
				}
			}
			return strings.ToLower(unidecodeSimple(s)), nil
		},
	})
	if err != nil {
		slog.Warn("dbutil: failed to register lower UDF", "err", err)
	}

	// Register title_sort UDF
	err = sqlite.RegisterFunction("title_sort", &sqlite.FunctionImpl{
		NArgs:         1,
		Deterministic: true,
		Scalar: func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			if len(args) == 0 || args[0] == nil {
				return nil, nil
			}
			s, ok := args[0].(string)
			if !ok {
				if b, ok := args[0].([]byte); ok {
					s = string(b)
				} else {
					return nil, fmt.Errorf("title_sort: invalid argument type")
				}
			}
			return titleSort(s), nil
		},
	})
	if err != nil {
		slog.Warn("dbutil: failed to register title_sort UDF", "err", err)
	}

	// Register uuid4 UDF
	err = sqlite.RegisterFunction("uuid4", &sqlite.FunctionImpl{
		NArgs:         0,
		Deterministic: false,
		Scalar: func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return uuid.NewString(), nil
		},
	})
	if err != nil {
		slog.Warn("dbutil: failed to register uuid4 UDF", "err", err)
	}
}

// Open opens app.db (primary) and returns an *sqlx.DB that automatically
// ATTACHes metadata.db on each new pool connection.  Call this once at
// startup.
func Open(appDBPath, metaDBPath string) (*sqlx.DB, error) {
	// WAL mode + shared cache allow concurrent readers.
	// We use the default modernc.org/sqlite "sqlite" driver.
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&cache=shared", appDBPath)

	db, err := sqlx.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("dbutil: open app.db: %w", err)
	}

	// SQLite is effectively single-writer; limit pool to avoid SQLITE_BUSY.
	db.SetMaxOpenConns(1)

	// Verify connection.
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("dbutil: ping app.db: %w", err)
	}

	// Run setup pragmas + ATTACH on the connection.
	if err := setupConn(db, metaDBPath); err != nil {
		return nil, err
	}

	slog.Info("dbutil: opened databases", "app", appDBPath, "calibre", metaDBPath)
	return db, nil
}

// setupConn runs on a *sqlx.DB (pool size 1) to install pragmas, ATTACH.
func setupConn(db *sqlx.DB, metaDBPath string) error {
	pragmas := []string{
		`PRAGMA cache_size=10000`,
		`PRAGMA foreign_keys=ON`,
		fmt.Sprintf(`ATTACH DATABASE '%s' AS calibre`, metaDBPath),
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return fmt.Errorf("dbutil: %s: %w", p, err)
		}
	}
	return nil
}

// Reconnect re-ATTACHes metadata.db (TaskReconnectDatabase).
func Reconnect(db *sqlx.DB, metaDBPath string) error {
	if _, err := db.Exec(`DETACH DATABASE calibre`); err != nil {
		slog.Warn("dbutil: DETACH calibre (may not exist)", "err", err)
	}
	_, err := db.Exec(fmt.Sprintf(`ATTACH DATABASE '%s' AS calibre`, metaDBPath))
	return err
}

// --- UDF helpers ---

var leadingArticleRe = regexp.MustCompile(
	`(?i)^(A|The|An|Der|Die|Das|Den|Ein|Eine|Einen|Dem|Des|Einem|Eines|Le|La|Les|L'|Un|Une)[\s']+`)

// titleSort applies the default leading-article sort (moves article to end).
func titleSort(title string) string {
	m := leadingArticleRe.FindStringIndex(title)
	if m == nil {
		return title
	}
	article := strings.TrimSpace(title[m[0]:m[1]])
	rest := strings.TrimSpace(title[m[1]:])
	return rest + ", " + article
}

// unidecodeSimple performs a basic ASCII transliteration.
func unidecodeSimple(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 128 {
			b.WriteRune(r)
		} else {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}
