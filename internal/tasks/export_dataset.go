package tasks

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/qalibre/qalibre/internal/appdb"
	"github.com/qalibre/qalibre/internal/books"
	"github.com/qalibre/qalibre/internal/calibredb"
	"github.com/qalibre/qalibre/internal/config"
	"github.com/qalibre/qalibre/internal/worker"
)

type TaskExportDataset struct {
	TaskID     string
	DatasetID  int
	ExportPath string
	Force      bool
	UserID     int
	Cfg        *config.Config
}

func (t *TaskExportDataset) Name() string        { return "Export Dataset to Markdown" }
func (t *TaskExportDataset) IsCancellable() bool { return true }

func (t *TaskExportDataset) SetTaskID(id string) {
	t.TaskID = id
}

func (t *TaskExportDataset) Run(ctx context.Context, db interface{}) (err error) {
	sqlxDB, ok := db.(*sqlx.DB)
	if !ok {
		return fmt.Errorf("export: bad db handle")
	}

	wMgr := worker.GetInstance(sqlxDB)

	// Ensure we recover from any potential panics (e.g. within pdf/epub library parsers)
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("task panicked: %v", r)
			slog.Error("export task panicked", "taskID", t.TaskID, "panic", r)
			wMgr.SetMessage(t.TaskID, fmt.Sprintf("Panicked: %v", r))
			wMgr.SetProgress(t.TaskID, 1.0)
		}
	}()

	slog.Info("export: starting task", "taskID", t.TaskID, "datasetID", t.DatasetID, "exportPath", t.ExportPath)
	wMgr.SetMessage(t.TaskID, "Loading dataset...")
	wMgr.SetProgress(t.TaskID, 0.05)

	// 1. Load dataset
	var dataset appdb.Dataset
	err = sqlxDB.Get(&dataset, "SELECT * FROM dataset WHERE id = ?", t.DatasetID)
	if err != nil {
		return fmt.Errorf("failed to fetch dataset: %w", err)
	}

	s3Enabled := s3Configured(&dataset)
	if s3Enabled {
		defer func() {
			slog.Info("export: cleaning up S3 temporary directory", "path", t.ExportPath)
			_ = os.RemoveAll(t.ExportPath)
		}()
	}

	// Load metadata
	var metadata []appdb.DatasetMetadata
	err = sqlxDB.Select(&metadata, "SELECT * FROM dataset_metadata WHERE dataset_id = ? ORDER BY sort_order", t.DatasetID)
	if err != nil {
		return fmt.Errorf("failed to fetch dataset metadata: %w", err)
	}

	// Load member book IDs
	var bookIDs []int
	err = sqlxDB.Select(&bookIDs, "SELECT book_id FROM dataset_book WHERE dataset_id = ? ORDER BY sort_order", t.DatasetID)
	if err != nil {
		return fmt.Errorf("failed to fetch dataset book IDs: %w", err)
	}

	// 2. Resolve target dir
	sanitizedName := sanitizeFilename(dataset.Name)
	if sanitizedName == "" {
		sanitizedName = fmt.Sprintf("dataset_%d", dataset.ID)
	}
	targetDir := filepath.Join(t.ExportPath, sanitizedName)
	err = os.MkdirAll(targetDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create target directory %s: %w", targetDir, err)
	}

	type bookStatus struct {
		Title        string
		BookID       int
		SourceFormat string
		Status       string
		Details      string
	}
	var statuses []bookStatus

	totalBooks := len(bookIDs)
	slog.Info("export: loaded books", "taskID", t.TaskID, "count", totalBooks)

	// 3. Process books
	for idx, bookID := range bookIDs {
		// Respect ctx.Err() for cancellation between books
		if err := ctx.Err(); err != nil {
			slog.Info("export: task cancelled", "taskID", t.TaskID)
			return err
		}

		// Calculate and update progress
		progress := 0.05 + (float64(idx)/float64(totalBooks))*0.90
		wMgr.SetProgress(t.TaskID, progress)

		// Load calibre Book
		var book calibredb.Book
		err = sqlxDB.Get(&book, "SELECT * FROM calibre.books WHERE id = ?", bookID)
		if err != nil {
			slog.Warn("export: calibre book missing in database", "taskID", t.TaskID, "bookID", bookID, "err", err)
			statuses = append(statuses, bookStatus{
				Title:        fmt.Sprintf("Book #%d", bookID),
				BookID:       bookID,
				SourceFormat: "unknown",
				Status:       "Missing",
				Details:      "Book record not found in calibre database",
			})
			continue
		}

		wMgr.SetMessage(t.TaskID, fmt.Sprintf("Processing (%d/%d): %s", idx+1, totalBooks, book.Title))
		slog.Info("export: processing book", "taskID", t.TaskID, "index", idx+1, "total", totalBooks, "bookID", bookID, "title", book.Title)

		// Fetch authors
		var authors []string
		_ = sqlxDB.Select(&authors, `
			SELECT a.name FROM calibre.books_authors_link l 
			JOIN calibre.authors a ON l.author = a.id 
			WHERE l.book = ? 
			ORDER BY l.id`, bookID)
		authorsStr := strings.Join(authors, " & ")

		// Load calibre Data formats
		var formats []calibredb.Data
		err = sqlxDB.Select(&formats, "SELECT * FROM calibre.data WHERE book = ?", bookID)
		if err != nil || len(formats) == 0 {
			slog.Warn("export: book has no formats", "taskID", t.TaskID, "bookID", bookID, "err", err)
			statuses = append(statuses, bookStatus{
				Title:        book.Title,
				BookID:       bookID,
				SourceFormat: "none",
				Status:       "Skipped",
				Details:      "No formats available for this book",
			})
			continue
		}

		// Choose preferred format
		data, ok := chooseFormat(formats)
		if !ok {
			var fmtNames []string
			for _, f := range formats {
				fmtNames = append(fmtNames, f.Format)
			}
			slog.Info("export: book has no supported formats", "taskID", t.TaskID, "bookID", bookID, "formats", fmtNames)
			statuses = append(statuses, bookStatus{
				Title:        book.Title,
				BookID:       bookID,
				SourceFormat: strings.Join(fmtNames, ","),
				Status:       "Skipped",
				Details:      "Unsupported formats: " + strings.Join(fmtNames, ", "),
			})
			continue
		}

		// Resolve absolute file path
		calibreDir := t.Cfg.GetCalibreDir()
		ext := strings.ToLower(data.Format)
		fileName := fmt.Sprintf("%s.%s", data.Name, ext)
		filePath := filepath.Join(calibreDir, book.Path, fileName)

		// Check if file exists, or check other extension for kepub
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			if ext == "kepub" {
				altFileName := fmt.Sprintf("%s.epub", data.Name)
				altFilePath := filepath.Join(calibreDir, book.Path, altFileName)
				if _, statErr := os.Stat(altFilePath); statErr == nil {
					filePath = altFilePath
					ext = "epub"
				}
			} else if ext == "epub" {
				altFileName := fmt.Sprintf("%s.kepub", data.Name)
				altFilePath := filepath.Join(calibreDir, book.Path, altFileName)
				if _, statErr := os.Stat(altFilePath); statErr == nil {
					filePath = altFilePath
					ext = "kepub"
				}
			}
		}

		// Check if file actually exists now
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			slog.Warn("export: format file missing on disk", "taskID", t.TaskID, "bookID", bookID, "filePath", filePath)
			statuses = append(statuses, bookStatus{
				Title:        book.Title,
				BookID:       bookID,
				SourceFormat: ext,
				Status:       "Missing",
				Details:      "File not found on disk",
			})
			continue
		}

		// Resolve Markdown output file path
		sanitizedTitle := sanitizeFilename(book.Title)
		if sanitizedTitle == "" {
			sanitizedTitle = fmt.Sprintf("book_%d", bookID)
		}
		bookFile := filepath.Join(targetDir, sanitizedTitle+".md")

		// Idempotency: Skip if file already exists (unless force is requested)
		if !t.Force {
			if _, err := os.Stat(bookFile); err == nil {
				slog.Info("export: skipping already exported book", "taskID", t.TaskID, "bookID", bookID, "file", bookFile)
				statuses = append(statuses, bookStatus{
					Title:        book.Title,
					BookID:       bookID,
					SourceFormat: ext,
					Status:       "Skipped",
					Details:      "Already exported (file exists)",
				})
				continue
			}
		}

		slog.Info("export: converting file", "taskID", t.TaskID, "bookID", bookID, "filePath", filePath, "format", ext)

		// Dispatch to conversion
		var mdText string
		var convErr error
		if ext == "kepub" {
			mdText, convErr = books.ToMarkdown(ctx, filePath, "epub")
		} else {
			mdText, convErr = books.ToMarkdown(ctx, filePath, ext)
		}

		if convErr != nil {
			if convErr == books.ErrImageOnlyPDF {
				slog.Info("export: skipped image-only PDF", "taskID", t.TaskID, "bookID", bookID)
				statuses = append(statuses, bookStatus{
					Title:        book.Title,
					BookID:       bookID,
					SourceFormat: ext,
					Status:       "Skipped",
					Details:      "Image-based (scanned) PDF contains no extractable text",
				})
			} else {
				slog.Error("export: conversion failed", "taskID", t.TaskID, "bookID", bookID, "err", convErr)
				statuses = append(statuses, bookStatus{
					Title:        book.Title,
					BookID:       bookID,
					SourceFormat: ext,
					Status:       "Failed",
					Details:      convErr.Error(),
				})
			}
			continue
		}

		escapedTitle := strings.ReplaceAll(book.Title, `"`, `\"`)
		escapedAuthors := strings.ReplaceAll(authorsStr, `"`, `\"`)
		escapedDatasetName := strings.ReplaceAll(dataset.Name, `"`, `\"`)

		frontMatter := fmt.Sprintf(`---
title: "%s"
authors: "%s"
book_id: %d
source_format: "%s"
dataset: "%s"
exported_at: "%s"
---

`, escapedTitle, escapedAuthors, bookID, ext, escapedDatasetName, time.Now().Format(time.RFC3339))

		err = os.WriteFile(bookFile, []byte(frontMatter+mdText), 0644)
		if err != nil {
			slog.Error("export: failed to write markdown file", "taskID", t.TaskID, "file", bookFile, "err", err)
			statuses = append(statuses, bookStatus{
				Title:        book.Title,
				BookID:       bookID,
				SourceFormat: ext,
				Status:       "Failed",
				Details:      fmt.Sprintf("Failed to write to file: %v", err),
			})
			continue
		}

		if s3Configured(&dataset) {
			s3Key := fmt.Sprintf("%s/%s.md", sanitizedName, sanitizedTitle)
			if uploadErr := uploadFileToS3(ctx, &dataset, bookFile, s3Key); uploadErr != nil {
				slog.Error("export plain: failed to upload to S3", "key", s3Key, "err", uploadErr)
				statuses = append(statuses, bookStatus{
					Title:        book.Title,
					BookID:       bookID,
					SourceFormat: ext,
					Status:       "Failed",
					Details:      fmt.Sprintf("Failed to upload to S3: %v", uploadErr),
				})
				continue
			}
		}

		statuses = append(statuses, bookStatus{
			Title:        book.Title,
			BookID:       bookID,
			SourceFormat: ext,
			Status:       "Exported",
			Details:      "-",
		})
	}

	wMgr.SetMessage(t.TaskID, "Writing index file...")
	wMgr.SetProgress(t.TaskID, 0.98)

	// 4. Write dataset.md index
	var indexBuilder strings.Builder
	indexBuilder.WriteString(fmt.Sprintf("# Dataset: %s\n\n", dataset.Name))
	if dataset.Description != "" {
		indexBuilder.WriteString(dataset.Description + "\n\n")
	}

	indexBuilder.WriteString("## Metadata\n\n")
	if len(metadata) == 0 {
		indexBuilder.WriteString("No metadata items defined.\n\n")
	} else {
		indexBuilder.WriteString("| Key | Value | Type |\n| --- | ----- | ---- |\n")
		for _, m := range metadata {
			indexBuilder.WriteString(fmt.Sprintf("| %s | %s | %s |\n", m.Key, m.Value, m.ValueType))
		}
		indexBuilder.WriteString("\n")
	}

	indexBuilder.WriteString("## Books\n\n")
	if len(statuses) == 0 {
		indexBuilder.WriteString("No books in this dataset.\n")
	} else {
		indexBuilder.WriteString("| Title | ID | Source Format | Status | Details |\n| ----- | -- | ------------- | ------ | ------- |\n")
		for _, s := range statuses {
			escapedTitle := strings.ReplaceAll(s.Title, "|", "\\|")
			indexBuilder.WriteString(fmt.Sprintf("| %s | %d | %s | %s | %s |\n", escapedTitle, s.BookID, s.SourceFormat, s.Status, s.Details))
		}
	}

	indexFile := filepath.Join(targetDir, "dataset.md")
	err = os.WriteFile(indexFile, []byte(indexBuilder.String()), 0644)
	if err != nil {
		return fmt.Errorf("failed to write dataset.md index file: %w", err)
	}

	if s3Configured(&dataset) {
		s3Key := fmt.Sprintf("%s/dataset.md", sanitizedName)
		if uploadErr := uploadFileToS3(ctx, &dataset, indexFile, s3Key); uploadErr != nil {
			slog.Error("export plain: failed to upload index to S3", "key", s3Key, "err", uploadErr)
			return fmt.Errorf("failed to upload index to S3: %w", uploadErr)
		}
	}

	wMgr.SetProgress(t.TaskID, 1.0)
	wMgr.SetMessage(t.TaskID, "Finished")
	slog.Info("export: task finished successfully", "taskID", t.TaskID, "datasetID", t.DatasetID)

	return nil
}

func sanitizeFilename(name string) string {
	badChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, c := range badChars {
		name = strings.ReplaceAll(name, c, "_")
	}
	return strings.TrimSpace(name)
}

func chooseFormat(formats []calibredb.Data) (calibredb.Data, bool) {
	priority := []string{"TXT", "EPUB", "PDF", "HTML"}
	for _, p := range priority {
		for _, f := range formats {
			if strings.ToUpper(f.Format) == p {
				return f, true
			}
		}
	}
	// kepub/kepa check
	for _, f := range formats {
		if strings.ToUpper(f.Format) == "KEPUB" {
			return f, true
		}
	}
	return calibredb.Data{}, false
}
