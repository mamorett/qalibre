package tasks

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/qalibre/qalibre/internal/dbutil"
	"github.com/qalibre/qalibre/internal/helper"
)

// TaskUpload status marker task
type TaskUpload struct{}

func (t *TaskUpload) Name() string        { return "Upload Book" }
func (t *TaskUpload) IsCancellable() bool { return false }
func (t *TaskUpload) Run(ctx context.Context, db interface{}) error {
	// Status-marker only; complete immediately
	return nil
}

// TaskReconnectDatabase
type TaskReconnectDatabase struct {
	MetaDBPath string
}

func (t *TaskReconnectDatabase) Name() string        { return "Reconnect Database" }
func (t *TaskReconnectDatabase) IsCancellable() bool { return false }
func (t *TaskReconnectDatabase) Run(ctx context.Context, db interface{}) error {
	slog.Info("running reconnect database task")
	sqlxDB, ok := db.(*sqlx.DB)
	if ok {
		return dbutil.Reconnect(sqlxDB, t.MetaDBPath)
	}
	return nil
}

// TaskClean deletes temp folder files and expired sessions
type TaskClean struct{}

func (t *TaskClean) Name() string        { return "Clean Cache & Expired Sessions" }
func (t *TaskClean) IsCancellable() bool { return false }
func (t *TaskClean) Run(ctx context.Context, db interface{}) error {
	slog.Info("running clean cache task")
	sqlxDB, ok := db.(*sqlx.DB)
	if ok {
		// Clean expired sessions
		_, _ = sqlxDB.Exec("DELETE FROM user_session WHERE expiry < ?", time.Now().Unix())
	}

	// Clean temp directory
	tempDir := filepath.Join(helper.GetConfigDir(), "temp")
	_ = os.RemoveAll(tempDir)
	_ = os.MkdirAll(tempDir, 0755)

	return nil
}

// TaskBackupMetadata stub
type TaskBackupMetadata struct{}

func (t *TaskBackupMetadata) Name() string        { return "Backup Metadata" }
func (t *TaskBackupMetadata) IsCancellable() bool { return true }
func (t *TaskBackupMetadata) Run(ctx context.Context, db interface{}) error {
	slog.Info("running metadata backup task (stub)")
	return nil
}

// TaskGenerateCoverThumbnails stub
type TaskGenerateCoverThumbnails struct{}

func (t *TaskGenerateCoverThumbnails) Name() string        { return "Generate Cover Thumbnails" }
func (t *TaskGenerateCoverThumbnails) IsCancellable() bool { return true }
func (t *TaskGenerateCoverThumbnails) Run(ctx context.Context, db interface{}) error {
	slog.Info("running cover thumbnail generation task (stub)")
	return nil
}

// TaskGenerateSeriesThumbnails stub
type TaskGenerateSeriesThumbnails struct{}

func (t *TaskGenerateSeriesThumbnails) Name() string        { return "Generate Series Thumbnails" }
func (t *TaskGenerateSeriesThumbnails) IsCancellable() bool { return true }
func (t *TaskGenerateSeriesThumbnails) Run(ctx context.Context, db interface{}) error {
	slog.Info("running series thumbnail generation task (stub)")
	return nil
}

// TaskClearCoverThumbnailCache stub
type TaskClearCoverThumbnailCache struct{}

func (t *TaskClearCoverThumbnailCache) Name() string        { return "Clear Cover Thumbnail Cache" }
func (t *TaskClearCoverThumbnailCache) IsCancellable() bool { return false }
func (t *TaskClearCoverThumbnailCache) Run(ctx context.Context, db interface{}) error {
	slog.Info("running clear cover thumbnail cache task (stub)")
	return nil
}
