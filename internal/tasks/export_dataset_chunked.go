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

type TaskExportDatasetChunked struct {
	TaskID       string
	DatasetID    int
	ExportPath   string // already-resolved root directory (absolute)
	ChunkSize    int
	ChunkOverlap int
	UserID       int
	Cfg          *config.Config
}

func (t *TaskExportDatasetChunked) Name() string        { return "Export Chunked Markdown" }
func (t *TaskExportDatasetChunked) IsCancellable() bool { return true }

func (t *TaskExportDatasetChunked) SetTaskID(id string) {
	t.TaskID = id
}

func (t *TaskExportDatasetChunked) Run(ctx context.Context, db interface{}) (err error) {
	sqlxDB, ok := db.(*sqlx.DB)
	if !ok {
		return fmt.Errorf("export chunked: bad db handle")
	}

	wMgr := worker.GetInstance(sqlxDB)

	// Ensure we recover from any potential panics
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("task panicked: %v", r)
			slog.Error("export chunked task panicked", "taskID", t.TaskID, "panic", r)
			wMgr.SetMessage(t.TaskID, fmt.Sprintf("Panicked: %v", r))
			wMgr.SetProgress(t.TaskID, 1.0)
		}
	}()

	slog.Info("export chunked: starting task", "taskID", t.TaskID, "datasetID", t.DatasetID, "exportPath", t.ExportPath)
	wMgr.SetMessage(t.TaskID, "Loading dataset for Chunked Markdown...")
	wMgr.SetProgress(t.TaskID, 0.05)

	// 1. Load dataset
	var dataset appdb.Dataset
	err = sqlxDB.Get(&dataset, "SELECT * FROM dataset WHERE id = ?", t.DatasetID)
	if err != nil {
		return fmt.Errorf("failed to fetch dataset: %w", err)
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

	// 2. Resolve target dirs
	sanitizedName := sanitizeFilename(dataset.Name)
	if sanitizedName == "" {
		sanitizedName = fmt.Sprintf("dataset_%d", dataset.ID)
	}
	root := filepath.Join(t.ExportPath, sanitizedName)
	chunkedDir := filepath.Join(root, "chunked")
	err = os.MkdirAll(chunkedDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create chunked directory %s: %w", chunkedDir, err)
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
	slog.Info("export chunked: loaded books", "taskID", t.TaskID, "count", totalBooks)

	// 3. Process books
	for idx, bookID := range bookIDs {
		// Respect ctx.Err() for cancellation between books
		if err := ctx.Err(); err != nil {
			slog.Info("export chunked: task cancelled", "taskID", t.TaskID)
			return err
		}

		// Calculate and update progress
		progress := 0.05 + (float64(idx)/float64(totalBooks))*0.90
		wMgr.SetProgress(t.TaskID, progress)

		// Load calibre Book
		var book calibredb.Book
		err = sqlxDB.Get(&book, "SELECT * FROM calibre.books WHERE id = ?", bookID)
		if err != nil {
			slog.Warn("export chunked: calibre book missing in database", "taskID", t.TaskID, "bookID", bookID, "err", err)
			statuses = append(statuses, bookStatus{
				Title:        fmt.Sprintf("Book #%d", bookID),
				BookID:       bookID,
				SourceFormat: "unknown",
				Status:       "Missing",
				Details:      "Book record not found in calibre database",
			})
			continue
		}

		wMgr.SetMessage(t.TaskID, fmt.Sprintf("Processing (%d/%d): %s (Chunked)", idx+1, totalBooks, book.Title))
		slog.Info("export chunked: processing book", "taskID", t.TaskID, "index", idx+1, "total", totalBooks, "bookID", bookID, "title", book.Title)

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
			slog.Warn("export chunked: book has no formats", "taskID", t.TaskID, "bookID", bookID, "err", err)
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
			slog.Info("export chunked: book has no supported formats", "taskID", t.TaskID, "bookID", bookID, "formats", fmtNames)
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
			slog.Warn("export chunked: format file missing on disk", "taskID", t.TaskID, "bookID", bookID, "filePath", filePath)
			statuses = append(statuses, bookStatus{
				Title:        book.Title,
				BookID:       bookID,
				SourceFormat: ext,
				Status:       "Missing",
				Details:      "File not found on disk",
			})
			continue
		}

		// Resolve sanitized title
		sanitizedTitle := sanitizeFilename(book.Title)
		if sanitizedTitle == "" {
			sanitizedTitle = fmt.Sprintf("book_%d", bookID)
		}

		slog.Info("export chunked: converting file", "taskID", t.TaskID, "bookID", bookID, "filePath", filePath, "format", ext)

		// If the book is image-only PDF, mark it as Skipped and continue (do NOT try to chunk an empty string)
		// We first check the format. If PDF, convert to Markdown using standard ToMarkdown first to see if it's image-only
		if ext == "pdf" {
			_, checkErr := books.ToMarkdown(filePath, "pdf")
			if checkErr == books.ErrImageOnlyPDF {
				slog.Info("export chunked: skipped image-only PDF", "taskID", t.TaskID, "bookID", bookID)
				statuses = append(statuses, bookStatus{
					Title:        book.Title,
					BookID:       bookID,
					SourceFormat: ext,
					Status:       "Skipped",
					Details:      "Image-based (scanned) PDF contains no extractable text",
				})
				continue
			}
		}

		// Convert and split
		var formatToUse string
		if ext == "kepub" {
			formatToUse = "epub"
		} else {
			formatToUse = ext
		}

		chunks, convErr := books.ToChunkedMarkdown(filePath, formatToUse, t.ChunkSize, t.ChunkOverlap)
		if convErr != nil {
			if convErr == books.ErrImageOnlyPDF {
				slog.Info("export chunked: skipped image-only PDF", "taskID", t.TaskID, "bookID", bookID)
				statuses = append(statuses, bookStatus{
					Title:        book.Title,
					BookID:       bookID,
					SourceFormat: ext,
					Status:       "Skipped",
					Details:      "Image-based (scanned) PDF contains no extractable text",
				})
			} else {
				slog.Error("export chunked: conversion/chunking failed", "taskID", t.TaskID, "bookID", bookID, "err", convErr)
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

		// Write each chunk to its own file
		escapedTitle := strings.ReplaceAll(book.Title, `"`, `\"`)
		escapedAuthors := strings.ReplaceAll(authorsStr, `"`, `\"`)
		escapedDatasetName := strings.ReplaceAll(dataset.Name, `"`, `\"`)

		// Format metadata section
		var metadataSection string
		if len(metadata) > 0 {
			var sb strings.Builder
			sb.WriteString("dataset_metadata:\n")
			for _, m := range metadata {
				// Quote values properly
				escapedKey := strings.ReplaceAll(m.Key, `"`, `\"`)
				escapedVal := strings.ReplaceAll(m.Value, `"`, `\"`)
				sb.WriteString(fmt.Sprintf("  %s: \"%s\"\n", escapedKey, escapedVal))
			}
			metadataSection = sb.String()
		}

		for _, chunk := range chunks {
			chunkFile := filepath.Join(chunkedDir, fmt.Sprintf("%s__chunk_%03d.md", sanitizedTitle, chunk.Index))
			frontMatter := fmt.Sprintf(`---
title: "%s"
authors: "%s"
book_id: %d
source_format: "%s"
dataset: "%s"
exported_at: "%s"
chunk_index: %d
chunk_size: %d
chunk_overlap: %d
`, escapedTitle, escapedAuthors, bookID, ext, escapedDatasetName, time.Now().Format(time.RFC3339), chunk.Index, t.ChunkSize, t.ChunkOverlap)

			if metadataSection != "" {
				frontMatter += metadataSection
			}
			frontMatter += "---\n\n"

			err = os.WriteFile(chunkFile, []byte(frontMatter+chunk.Text), 0644)
			if err != nil {
				slog.Error("export chunked: failed to write chunk file", "taskID", t.TaskID, "file", chunkFile, "err", err)
				statuses = append(statuses, bookStatus{
					Title:        book.Title,
					BookID:       bookID,
					SourceFormat: ext,
					Status:       "Failed",
					Details:      fmt.Sprintf("Failed to write chunk %d: %v", chunk.Index, err),
				})
				// break or continue? Let's just record failed status once and stop writing chunks of this book
				break
			}
		}

		// If no failures occurred for this book, record exported status
		var alreadyRecorded bool
		for _, s := range statuses {
			if s.BookID == bookID {
				alreadyRecorded = true
				break
			}
		}
		if !alreadyRecorded {
			statuses = append(statuses, bookStatus{
				Title:        book.Title,
				BookID:       bookID,
				SourceFormat: ext,
				Status:       "Exported",
				Details:      fmt.Sprintf("%d chunks generated", len(chunks)),
			})
		}
	}

	wMgr.SetMessage(t.TaskID, "Writing index file...")
	wMgr.SetProgress(t.TaskID, 0.98)

	// 4. Write dataset.md index to the dataset root (NOT chunked/)
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

	indexBuilder.WriteString("## Chunked Markdown\n\n")
	if len(statuses) == 0 {
		indexBuilder.WriteString("No books in this dataset.\n")
	} else {
		indexBuilder.WriteString("| Title | ID | Source Format | Status | Details |\n| ----- | -- | ------------- | ------ | ------- |\n")
		for _, s := range statuses {
			escapedTitle := strings.ReplaceAll(s.Title, "|", "\\|")
			indexBuilder.WriteString(fmt.Sprintf("| %s | %d | %s | %s | %s |\n", escapedTitle, s.BookID, s.SourceFormat, s.Status, s.Details))
		}
	}

	indexBuilder.WriteString(fmt.Sprintf("\n---\nChunked via langchain MarkdownTextSplitter (chunk_size=%d, overlap=%d).\n", t.ChunkSize, t.ChunkOverlap))

	indexFile := filepath.Join(root, "dataset.md")
	err = os.WriteFile(indexFile, []byte(indexBuilder.String()), 0644)
	if err != nil {
		return fmt.Errorf("failed to write dataset.md index file: %w", err)
	}

	wMgr.SetProgress(t.TaskID, 1.0)
	wMgr.SetMessage(t.TaskID, "Finished")
	slog.Info("export chunked: task finished successfully", "taskID", t.TaskID, "datasetID", t.DatasetID)

	return nil
}
