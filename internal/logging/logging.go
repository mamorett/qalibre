// Package logging sets up structured logging with log rotation.
// It replaces cps/logger.py (rotating qalibre.log + access.log).
package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	DefaultLogFile    = "qalibre.log"
	DefaultAccessLog  = "access.log"
	DefaultLogLevel   = slog.LevelInfo
)

// Setup initialises the default slog handler writing to both a rotating
// file and stderr.  level is an slog level string (DEBUG|INFO|WARN|ERROR).
func Setup(logfile string, levelStr string) {
	level := parseLevel(levelStr)

	var writers []io.Writer
	writers = append(writers, os.Stderr)

	if logfile != "" {
		writers = append(writers, &lumberjack.Logger{
			Filename:   logfile,
			MaxSize:    10, // MB
			MaxBackups: 3,
			Compress:   true,
		})
	}

	w := io.MultiWriter(writers...)
	h := slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(h))
}

// SetupAccess creates a separate access-log writer suitable for chi's
// middleware.Logger.  Returns an io.Writer backed by a rotating file.
func SetupAccess(logfile string, enabled bool) io.Writer {
	if !enabled || logfile == "" {
		return io.Discard
	}
	return &lumberjack.Logger{
		Filename:   logfile,
		MaxSize:    50,
		MaxBackups: 5,
		Compress:   true,
	}
}

func parseLevel(s string) slog.Level {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARNING", "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
