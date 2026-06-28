package routes

import (
	"log/slog"
	"net/http"

	"github.com/qalibre/qalibre/internal/appdb"
	"github.com/qalibre/qalibre/internal/auth"
	"github.com/qalibre/qalibre/internal/config"
)

type FormatStat struct {
	Format string `db:"format" json:"format"`
	Count  int    `db:"count" json:"count"`
	Size   int64  `db:"size" json:"size"`
}

type StatsResponse struct {
	Version         string       `json:"version"`
	TotalBooks      int          `json:"total_books"`
	TotalAuthors    int          `json:"total_authors"`
	TotalSeries     int          `json:"total_series"`
	TotalTags       int          `json:"total_tags"`
	TotalPublishers int          `json:"total_publishers"`
	ReadBooks       int          `json:"read_books"`
	UnreadBooks     int          `json:"unread_books"`
	ArchivedBooks   int          `json:"archived_books"`
	Formats         []FormatStat `json:"formats"`
}

// GetStats handles GET /api/v1/stats
func (rm *RouteManager) GetStats(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r)
	var userID int
	if ok {
		u := user.(*appdb.User)
		userID = u.ID
	}

	var stats StatsResponse
	stats.Version = config.STABLE_VERSION

	// 1. Total books
	err := rm.DB.Get(&stats.TotalBooks, "SELECT COUNT(*) FROM calibre.books")
	if err != nil {
		slog.Error("stats: failed to count books", "err", err)
	}

	// 2. Total authors
	err = rm.DB.Get(&stats.TotalAuthors, "SELECT COUNT(*) FROM calibre.authors")
	if err != nil {
		slog.Error("stats: failed to count authors", "err", err)
	}

	// 3. Total series
	err = rm.DB.Get(&stats.TotalSeries, "SELECT COUNT(*) FROM calibre.series")
	if err != nil {
		slog.Error("stats: failed to count series", "err", err)
	}

	// 4. Total tags
	err = rm.DB.Get(&stats.TotalTags, "SELECT COUNT(*) FROM calibre.tags")
	if err != nil {
		slog.Error("stats: failed to count tags", "err", err)
	}

	// 5. Total publishers
	err = rm.DB.Get(&stats.TotalPublishers, "SELECT COUNT(*) FROM calibre.publishers")
	if err != nil {
		slog.Error("stats: failed to count publishers", "err", err)
	}

	// 6. User-specific stats (if logged in)
	if userID > 0 {
		err = rm.DB.Get(&stats.ReadBooks, "SELECT COUNT(*) FROM book_read_link WHERE user_id = ? AND read_status = 1", userID)
		if err != nil {
			slog.Error("stats: failed to count read books", "err", err)
		}

		err = rm.DB.Get(&stats.ArchivedBooks, "SELECT COUNT(*) FROM archived_book WHERE user_id = ? AND is_archived = 1", userID)
		if err != nil {
			slog.Error("stats: failed to count archived books", "err", err)
		}
	}

	stats.UnreadBooks = stats.TotalBooks - stats.ReadBooks
	if stats.UnreadBooks < 0 {
		stats.UnreadBooks = 0
	}

	// 7. Formats distribution
	var formats []FormatStat
	err = rm.DB.Select(&formats, `
		SELECT format, COUNT(*) AS count, COALESCE(SUM(uncompressed_size), 0) AS size 
		FROM calibre.data 
		GROUP BY format 
		ORDER BY count DESC`)
	if err != nil {
		slog.Error("stats: failed to count formats", "err", err)
		stats.Formats = []FormatStat{}
	} else {
		stats.Formats = formats
	}

	rm.WriteJSON(w, stats)
}
