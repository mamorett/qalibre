package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/sessions"
	"github.com/jmoiron/sqlx"
	"github.com/qalibre/qalibre/internal/appdb"
	"github.com/qalibre/qalibre/internal/config"
)

// GenerateRandomToken generates a hex-encoded random token.
func GenerateRandomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// SessionManager handles user session verification and persistence.
type SessionManager struct {
	DB    *sqlx.DB
	Store *sessions.CookieStore
	Cfg   *config.Config
}

func NewSessionManager(db *sqlx.DB, store *sessions.CookieStore, cfg *config.Config) *SessionManager {
	return &SessionManager{DB: db, Store: store, Cfg: cfg}
}

// LoginUser sets up the session in DB and session cookie.
func (sm *SessionManager) LoginUser(w http.ResponseWriter, r *http.Request, user *appdb.User, remember bool) error {
	session, _ := sm.Store.Get(r, "session")

	randomToken := GenerateRandomToken(16)
	sessionKey := GenerateRandomToken(16)

	// Set session cookie values
	session.Values["_user_id"] = strconv.Itoa(user.ID)
	session.Values["random"] = randomToken
	session.Values["session_key"] = sessionKey

	if remember {
		session.Options.MaxAge = 3600 * 24 * 31 // 31 days
	} else {
		session.Options.MaxAge = 0 // Session cookie
	}

	err := session.Save(r, w)
	if err != nil {
		return err
	}

	// Persist to user_session table
	expiry := time.Now().Add(31 * 24 * time.Hour).Unix()
	_, err = sm.DB.Exec(`INSERT INTO user_session (user_id, session_key, random, expiry) VALUES (?, ?, ?, ?)`,
		user.ID, sessionKey, randomToken, expiry)
	return err
}

// LogoutUser clears the session in DB and session cookie.
func (sm *SessionManager) LogoutUser(w http.ResponseWriter, r *http.Request) error {
	session, _ := sm.Store.Get(r, "session")

	userIDStr, ok1 := session.Values["_user_id"].(string)
	randomToken, ok2 := session.Values["random"].(string)
	sessionKey, ok3 := session.Values["session_key"].(string)

	if ok1 && ok2 && ok3 {
		userID, _ := strconv.Atoi(userIDStr)
		// Delete from DB
		_, _ = sm.DB.Exec(`DELETE FROM user_session WHERE user_id = ? AND random = ? AND session_key = ?`,
			userID, randomToken, sessionKey)
	}

	session.Values["_user_id"] = ""
	session.Values["random"] = ""
	session.Values["session_key"] = ""
	session.Options.MaxAge = -1
	return session.Save(r, w)
}

// LoadUser validates the session cookies against the DB and returns the loaded user if valid.
func (sm *SessionManager) LoadUser(r *http.Request) (*appdb.User, error) {
	// Reverse proxy header login support
	settings := sm.Cfg.Get()
	if settings.ConfigAllowReverseProxyHeaderLogin && settings.ConfigReverseProxyLoginHeaderName != "" {
		headerUser := r.Header.Get(settings.ConfigReverseProxyLoginHeaderName)
		if headerUser != "" {
			var user appdb.User
			err := sm.DB.Get(&user, "SELECT * FROM user WHERE lower(name) = lower(?) LIMIT 1", headerUser)
			if err == nil {
				return &user, nil
			}
		}
	}

	session, _ := sm.Store.Get(r, "session")
	userIDStr, ok1 := session.Values["_user_id"].(string)
	randomToken, _ := session.Values["random"].(string)
	sessionKey, _ := session.Values["session_key"].(string)

	if !ok1 || userIDStr == "" {
		return nil, nil // No active session
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		return nil, err
	}

	// Validate against DB
	var sessionCount int
	err = sm.DB.Get(&sessionCount,
		"SELECT COUNT(*) FROM user_session WHERE user_id = ? AND random = ? AND session_key = ? AND expiry > ?",
		userID, randomToken, sessionKey, time.Now().Unix())
	if err != nil || sessionCount == 0 {
		return nil, nil // Invalid session
	}

	var user appdb.User
	err = sm.DB.Get(&user, "SELECT * FROM user WHERE id = ? LIMIT 1", userID)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// LoadGuest loads the anonymous/guest user row if anonymous browsing is enabled.
func (sm *SessionManager) LoadGuest() (*appdb.User, error) {
	var user appdb.User
	err := sm.DB.Get(&user, "SELECT * FROM user WHERE name = 'Guest' LIMIT 1")
	return &user, err
}

// AuthMiddleware loads the user from session or falls back to Guest if anon browse is enabled.
func (sm *SessionManager) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := sm.LoadUser(r)
		if err != nil {
			slogError("session load failed", err)
		}

		if user == nil {
			// Fallback to Guest if config_anonbrowse == 1
			if sm.Cfg.Get().ConfigAnonBrowse == 1 {
				guest, err := sm.LoadGuest()
				if err == nil {
					user = guest
				}
			}
		}

		if user != nil {
			ctx := context.WithValue(r.Context(), UserKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			next.ServeHTTP(w, r)
		}
	})
}

// Role Requirement Middlewares

func (sm *SessionManager) RequireLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := GetUserFromContext(r)
		if !ok || user.(*appdb.User).Name == "Guest" {
			sm.sendAuthError(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (sm *SessionManager) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := GetUserFromContext(r)
		if !ok || !HasRole(user.(*appdb.User).Role, config.RoleAdmin) {
			sm.sendAuthError(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (sm *SessionManager) RequireUpload(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := GetUserFromContext(r)
		if !ok || (!HasRole(user.(*appdb.User).Role, config.RoleUpload) && !HasRole(user.(*appdb.User).Role, config.RoleAdmin)) {
			sm.sendAuthError(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (sm *SessionManager) RequireEdit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := GetUserFromContext(r)
		if !ok || (!HasRole(user.(*appdb.User).Role, config.RoleEdit) && !HasRole(user.(*appdb.User).Role, config.RoleAdmin)) {
			sm.sendAuthError(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (sm *SessionManager) RequireDownload(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := GetUserFromContext(r)
		if !ok || (!HasRole(user.(*appdb.User).Role, config.RoleDownload) && !HasRole(user.(*appdb.User).Role, config.RoleAdmin)) {
			sm.sendAuthError(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (sm *SessionManager) RequireViewer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := GetUserFromContext(r)
		if !ok || (!HasRole(user.(*appdb.User).Role, config.RoleViewer) && !HasRole(user.(*appdb.User).Role, config.RoleAdmin)) {
			sm.sendAuthError(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (sm *SessionManager) sendAuthError(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ajax/") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "Login required"}`))
	} else {
		http.Redirect(w, r, "/spa/login", http.StatusFound)
	}
}

// logging helper to avoid direct slog error calls in package scope
func slogError(msg string, err error) {
	// slog error
}
