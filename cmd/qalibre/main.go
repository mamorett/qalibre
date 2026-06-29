package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/qalibre/qalibre/internal/appdb"
	"github.com/qalibre/qalibre/internal/auth"
	"github.com/qalibre/qalibre/internal/config"
	"github.com/qalibre/qalibre/internal/dbutil"
	"github.com/qalibre/qalibre/internal/helper"
	"github.com/qalibre/qalibre/internal/logging"
	"github.com/qalibre/qalibre/internal/routes"
	"github.com/qalibre/qalibre/internal/server"
)

func main() {
	// Parse CLI flags
	settingsPathFlag := flag.String("p", "", "path to app.db (settings db)")
	certFlag := flag.String("c", "", "path to TLS certificate")
	keyFlag := flag.String("k", "", "path to TLS private key")
	ipFlag := flag.String("i", "", "IP address to listen on")
	versionFlag := flag.Bool("v", false, "print version and exit")
	flag.BoolVar(versionFlag, "version", false, "print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("Qalibre version: %s\n", config.STABLE_VERSION)
		os.Exit(0)
	}

	// Determine settings path
	configDir := helper.GetConfigDir()
	appDBPath := *settingsPathFlag
	if appDBPath == "" {
		appDBPath = filepath.Join(configDir, "app.db")
	}

	// Ensure config directory exists
	if err := os.MkdirAll(filepath.Dir(appDBPath), 0755); err != nil {
		slog.Error("failed to create config directory", "err", err)
		os.Exit(1)
	}

	// 1. Boot up app.db (first part of Phase 0 scaffolding)
	slog.Info("booting Qalibre Go port...", "app.db", appDBPath)

	// Temporary setup: open app.db to initialize tables & seed if empty
	// We pass temporary empty metadata.db path for attachment initially,
	// then resolve the real calibre dir and reconnect.
	tempMetaDB := filepath.Join(configDir, "metadata.db")
	db, err := dbutil.Open(appDBPath, tempMetaDB)
	if err != nil {
		slog.Error("failed to open database", "err", err)
		os.Exit(1)
	}

	// Init schema tables
	hashedAdmin, err := auth.GeneratePasswordHash("admin123")
	if err != nil {
		slog.Error("failed to hash default admin password", "err", err)
		os.Exit(1)
	}
	if err := appdb.InitSchema(db, hashedAdmin); err != nil {
		slog.Error("failed to initialize schema", "err", err)
		os.Exit(1)
	}

	// Load configuration singleton from settings table
	cliCalibreDir := os.Getenv("CALIBRE_DIR")
	cfg, err := config.Load(db, cliCalibreDir)
	if err != nil {
		slog.Error("failed to load settings from DB", "err", err)
		os.Exit(1)
	}

	// Re-route calibre database attachment to the resolved calibre path
	resolvedCalibreDir := cfg.GetCalibreDir()
	if resolvedCalibreDir != "" {
		realMetaDB := filepath.Join(resolvedCalibreDir, "metadata.db")
		slog.Info("attaching Calibre metadata.db", "path", realMetaDB)
		if err := dbutil.Reconnect(db, realMetaDB); err != nil {
			slog.Error("failed to attach Calibre metadata.db", "path", realMetaDB, "err", err)
			// Let it continue; we might be in initial config setup state
		}
	} else {
		slog.Warn("no Calibre library directory configured yet; setup via UI config required")
	}

	// 2. Setup logging based on settings
	settings := cfg.Get()
	logLevelStr := "INFO"
	switch settings.ConfigLogLevel {
	case 10:
		logLevelStr = "DEBUG"
	case 20:
		logLevelStr = "INFO"
	case 30:
		logLevelStr = "WARNING"
	case 40:
		logLevelStr = "ERROR"
	}
	logging.Setup(settings.ConfigLogFile, logLevelStr)

	// Determine listen address
	port := 8083
	if envPort := os.Getenv("CALIBRE_PORT"); envPort != "" {
		fmt.Sscanf(envPort, "%d", &port)
	} else if settings.ConfigPort > 0 {
		port = settings.ConfigPort
	}
	ip := "0.0.0.0"
	if *ipFlag != "" {
		ip = *ipFlag
	}
	addr := fmt.Sprintf("%s:%d", ip, port)

	// Setup cert/key if flags passed overriding DB settings
	if *certFlag != "" {
		cfg.Update(func(s *config.Settings) {
			s.ConfigCertFile = certFlag
		})
	}
	if *keyFlag != "" {
		cfg.Update(func(s *config.Settings) {
			s.ConfigKeyFile = keyFlag
		})
	}

	// Resolve frontend static files dir
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)
	frontDir := filepath.Join(execDir, "frontend/dist")
	if _, err := os.Stat(frontDir); err != nil {
		// Fallback to workspace path or /gorgon/dev/qalibre/frontend/dist
		frontDir = "/gorgon/dev/qalibre/frontend/dist"
	}

	slog.Info("frontend static assets directory", "path", frontDir)

	// Build and run server
	appServer, err := server.New(db.Unsafe(), cfg, os.Getenv("APP_MODE") != "production", frontDir)
	if err != nil {
		slog.Error("failed to create server", "err", err)
		os.Exit(1)
	}

	// Register API endpoints
	sessionMgr := auth.NewSessionManager(db.Unsafe(), appServer.Store, cfg)
	routeMgr := routes.NewRouteManager(db.Unsafe(), cfg, sessionMgr, os.Getenv("APP_MODE") != "production")
	routeMgr.RegisterRoutes(appServer.Router)

	appServer.ServeSPA()

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := appServer.Start(addr); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	slog.Info("Qalibre server started", "addr", addr)

	sig := <-sigChan
	slog.Info("shutting down gracefully...", "signal", sig)

	// Defer 5 seconds max for graceful shutdown
	_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db.Close()
	slog.Info("Qalibre server stopped")
}
