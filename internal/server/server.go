package server

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/sessions"
	"github.com/jmoiron/sqlx"
	"github.com/qalibre/qalibre/internal/config"
	"github.com/qalibre/qalibre/internal/logging"
)

type Server struct {
	Router   *chi.Mux
	Config   *config.Config
	DB       *sqlx.DB
	Store    *sessions.CookieStore
	IsDev    bool
	FrontDir string
}

func New(db *sqlx.DB, cfg *config.Config, isDev bool, frontDir string) (*Server, error) {
	// Load session key from flask_settings table
	var sessionKey []byte
	err := db.Get(&sessionKey, "SELECT flask_session_key FROM flask_settings LIMIT 1")
	if err != nil || len(sessionKey) == 0 {
		// Generate random 32-byte key if missing
		sessionKey = make([]byte, 32)
		for i := 0; i < 32; i++ {
			sessionKey[i] = byte(i)
		}
		_, _ = db.Exec("INSERT OR REPLACE INTO flask_settings (id, flask_session_key) VALUES (1, ?)", sessionKey)
	}

	cookieStore := sessions.NewCookieStore(sessionKey)
	cookieStore.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   3600 * 24 * 31, // 31 days
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   !isDev,
	}

	s := &Server{
		Router:   chi.NewRouter(),
		Config:   cfg,
		DB:       db,
		Store:    cookieStore,
		IsDev:    isDev,
		FrontDir: frontDir,
	}

	s.setupMiddleware()
	return s, nil
}

func (s *Server) setupMiddleware() {
	s.Router.Use(middleware.RequestID)
	s.Router.Use(middleware.RealIP)
	s.Router.Use(middleware.Recoverer)

	// Custom Access log middleware
	accessLogEnabled := s.Config.Get().ConfigAccessLog == 1
	accessLogFile := s.Config.Get().ConfigAccessLogFile
	accessWriter := logging.SetupAccess(accessLogFile, accessLogEnabled)
	if accessLogEnabled && accessWriter != io.Discard {
		s.Router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				start := time.Now()
				ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
				next.ServeHTTP(ww, r)
				duration := time.Since(start)
				date := time.Now().Format("02/Jan/2006:15:04:05 -0700")
				fmt.Fprintf(accessWriter, "%s - - [%s] \"%s %s %s\" %d %d %v\n",
					r.RemoteAddr, date, r.Method, r.URL.RequestURI(), r.Proto,
					ww.Status(), ww.BytesWritten(), duration)
			})
		})
	}

	s.Router.Use(s.reverseProxyMiddleware)
	s.Router.Use(s.spaRedirectMiddleware)
}

func (s *Server) reverseProxyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// X-Script-Name
		scriptName := r.Header.Get("X-Script-Name")
		if scriptName != "" {
			if strings.HasPrefix(r.URL.Path, scriptName) {
				r.URL.Path = r.URL.Path[len(scriptName):]
				if r.URL.Path == "" {
					r.URL.Path = "/"
				}
			}
		}

		// X-Forwarded-Host
		forwardedHost := r.Header.Get("X-Forwarded-Host")
		if forwardedHost != "" {
			r.Host = forwardedHost
		}

		// We can store original headers in request context if needed
		next.ServeHTTP(w, r)
	})
}

func (s *Server) spaRedirectMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		path := r.URL.Path

		// Exemptions
		if strings.HasPrefix(path, "/api/") ||
			strings.HasPrefix(path, "/ajax/") ||
			strings.HasPrefix(path, "/static/") ||
			strings.HasPrefix(path, "/spa") ||
			strings.HasPrefix(path, "/cover/") ||
			strings.HasPrefix(path, "/series_cover/") ||
			strings.HasPrefix(path, "/download/") ||
			strings.HasPrefix(path, "/read/") ||
			strings.HasPrefix(path, "/shelf/") ||
			strings.HasPrefix(path, "/get_") ||
			path == "/favicon.ico" ||
			path == "/robots.txt" ||
			path == "/apple-touch-icon.png" {
			next.ServeHTTP(w, r)
			return
		}

		// Redirects
		if path == "/login" {
			http.Redirect(w, r, "/spa/login", http.StatusFound)
			return
		}
		if path == "/register" {
			http.Redirect(w, r, "/spa/register", http.StatusFound)
			return
		}
		if strings.HasPrefix(path, "/admin/config") || strings.HasPrefix(path, "/admin/dbconfig") {
			http.Redirect(w, r, "/spa/config", http.StatusFound)
			return
		}
		if strings.HasPrefix(path, "/admin") {
			http.Redirect(w, r, "/spa/admin", http.StatusFound)
			return
		}
		if strings.HasPrefix(path, "/edit/") {
			parts := strings.Split(path, "/")
			if len(parts) >= 3 {
				// edit id is parts[2]
				http.Redirect(w, r, "/spa/edit/"+parts[2], http.StatusFound)
				return
			}
			http.Redirect(w, r, "/spa", http.StatusFound)
			return
		}

		http.Redirect(w, r, "/spa", http.StatusFound)
	})
}

// ServeSPA mounts the static file router for the built React frontend.
func (s *Server) ServeSPA() {
	// Serve static public assets
	publicDir := filepath.Join(filepath.Dir(s.FrontDir), "public")
	s.Router.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir(publicDir))))

	// Serve files under /spa from s.FrontDir
	s.Router.HandleFunc("/spa*", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if !strings.HasPrefix(path, "/spa") {
			http.NotFound(w, r)
			return
		}

		// Relative path inside frontend/dist/
		rel := path[len("/spa"):]
		if rel == "" || rel == "/" {
			rel = "index.html"
		} else {
			rel = strings.TrimPrefix(rel, "/")
		}

		targetFile := filepath.Join(s.FrontDir, rel)
		info, err := os.Stat(targetFile)
		if err != nil || info.IsDir() {
			// Fallback to index.html for client-side routing
			targetFile = filepath.Join(s.FrontDir, "index.html")
		}

		http.ServeFile(w, r, targetFile)
	})

	// Also redirect root / to /spa
	s.Router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/spa", http.StatusFound)
	})
}

func (s *Server) Start(addr string) error {
	srv := &http.Server{
		Addr:         addr,
		Handler:      s.Router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Cert setup
	settings := s.Config.Get()
	certFile := ""
	keyFile := ""
	if settings.ConfigCertFile != nil {
		certFile = *settings.ConfigCertFile
	}
	if settings.ConfigKeyFile != nil {
		keyFile = *settings.ConfigKeyFile
	}

	if certFile != "" && keyFile != "" {
		slog.Info("server: listening TLS", "addr", addr)
		return srv.ListenAndServeTLS(certFile, keyFile)
	}

	slog.Info("server: listening HTTP", "addr", addr)
	return srv.ListenAndServe()
}
