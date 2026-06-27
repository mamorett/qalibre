package routes

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gorilla/csrf"
	"github.com/qalibre/qalibre/internal/appdb"
	"github.com/qalibre/qalibre/internal/auth"
	"github.com/qalibre/qalibre/internal/calibredb"
	"github.com/qalibre/qalibre/internal/config"
	"github.com/qalibre/qalibre/internal/helper"
)

// GetSession handler for GET /api/v1/session
func (rm *RouteManager) GetSession(w http.ResponseWriter, r *http.Request) {
	var userData interface{} = nil

	u, ok := auth.GetUserFromContext(r)
	if ok && u.(*appdb.User).Name != "Guest" {
		user := u.(*appdb.User)
		userData = map[string]interface{}{
			"id":            user.ID,
			"nickname":      user.Name,
			"email":         user.Email,
			"role_admin":    auth.HasRole(user.Role, config.RoleAdmin),
			"role_edit":     auth.HasRole(user.Role, config.RoleEdit),
			"role_download": auth.HasRole(user.Role, config.RoleDownload),
			"role_upload":   auth.HasRole(user.Role, config.RoleUpload),
			"locale":        user.Locale,
		}
	}

	cfg := rm.Cfg.Get()
	csrfToken := csrf.Token(r)

	// Set csrf_access_token cookie for the client request fallback
	http.SetCookie(w, &http.Cookie{
		Name:     "csrf_access_token",
		Value:    csrfToken,
		Path:     "/",
		HttpOnly: false, // Client needs to read it
		SameSite: http.SameSiteLaxMode,
		Secure:   !rm.IsDev,
	})

	rm.WriteJSON(w, map[string]interface{}{
		"user": userData,
		"config": map[string]interface{}{
			"books_per_page":   cfg.ConfigBooksPerPage,
			"authors_max":      cfg.ConfigAuthorsMax,
			"upload_enabled":   cfg.ConfigUploading == 1,
			"kobo_enabled":     false, // Kobo is dropped
			"anonymous_browse": cfg.ConfigAnonBrowse == 1,
			"public_register":  cfg.ConfigPublicReg == 1,
		},
		"csrf_token": csrfToken,
		"locale":     "en",
	})
}

// Login handler for POST /api/v1/login
func (rm *RouteManager) Login(w http.ResponseWriter, r *http.Request) {
	u, ok := auth.GetUserFromContext(r)
	if ok && u.(*appdb.User).Name != "Guest" {
		user := u.(*appdb.User)
		rm.WriteJSON(w, map[string]interface{}{
			"success": true,
			"user": map[string]interface{}{
				"id":            user.ID,
				"nickname":      user.Name,
				"email":         user.Email,
				"role_admin":    auth.HasRole(user.Role, config.RoleAdmin),
				"role_edit":     auth.HasRole(user.Role, config.RoleEdit),
				"role_download": auth.HasRole(user.Role, config.RoleDownload),
				"role_upload":   auth.HasRole(user.Role, config.RoleUpload),
				"locale":        user.Locale,
			},
		})
		return
	}

	var req struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		RememberMe bool   `json:"remember_me"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rm.ErrorJSON(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	username := strings.TrimSpace(strings.ToLower(req.Username))
	if username == "" || req.Password == "" {
		rm.ErrorJSON(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	var user appdb.User
	err := rm.DB.Get(&user, "SELECT * FROM user WHERE lower(name) = ? LIMIT 1", username)
	if err != nil {
		rm.ErrorJSON(w, "Wrong Username or Password", http.StatusUnauthorized)
		return
	}

	if user.Name == "Guest" {
		rm.ErrorJSON(w, "Wrong Username or Password", http.StatusUnauthorized)
		return
	}

	// Verify standard local password hash
	if !auth.CheckPasswordHash(user.Password, req.Password) {
		rm.ErrorJSON(w, "Wrong Username or Password", http.StatusUnauthorized)
		return
	}

	// Session save
	err = rm.SM.LoginUser(w, r, &user, req.RememberMe)
	if err != nil {
		slog.Error("login session setup failed", "err", err)
		rm.ErrorJSON(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	rm.WriteJSON(w, map[string]interface{}{
		"success": true,
		"user": map[string]interface{}{
			"id":            user.ID,
			"nickname":      user.Name,
			"email":         user.Email,
			"role_admin":    auth.HasRole(user.Role, config.RoleAdmin),
			"role_edit":     auth.HasRole(user.Role, config.RoleEdit),
			"role_download": auth.HasRole(user.Role, config.RoleDownload),
			"role_upload":   auth.HasRole(user.Role, config.RoleUpload),
			"locale":        user.Locale,
		},
	})
}

// Logout handler for POST /api/v1/logout
func (rm *RouteManager) Logout(w http.ResponseWriter, r *http.Request) {
	_ = rm.SM.LogoutUser(w, r)
	rm.WriteJSON(w, map[string]interface{}{
		"success": true,
	})
}

// Register handler for POST /api/v1/register
func (rm *RouteManager) Register(w http.ResponseWriter, r *http.Request) {
	cfg := rm.Cfg.Get()
	if cfg.ConfigPublicReg != 1 {
		rm.ErrorJSON(w, "Public registration is disabled", http.StatusForbidden)
		return
	}

	u, ok := auth.GetUserFromContext(r)
	if ok && u.(*appdb.User).Name != "Guest" {
		rm.ErrorJSON(w, "Already logged in", http.StatusBadRequest)
		return
	}

	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rm.ErrorJSON(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	username := strings.TrimSpace(req.Username)
	email := strings.TrimSpace(req.Email)

	if username == "" || email == "" {
		rm.ErrorJSON(w, "Username and email are required", http.StatusBadRequest)
		return
	}

	if !helper.CheckUsername(username) {
		rm.ErrorJSON(w, "Invalid username format", http.StatusBadRequest)
		return
	}

	if !helper.CheckEmail(email) {
		rm.ErrorJSON(w, "Invalid email format", http.StatusBadRequest)
		return
	}

	// Check domain allowlist
	var allowedDomains []string
	err := rm.DB.Select(&allowedDomains, "SELECT domain FROM registration WHERE allow = 1")
	if err != nil {
		slog.Error("registration: failed to fetch allowed domains", "err", err)
	}
	if len(allowedDomains) > 0 && !helper.CheckValidDomain(email, allowedDomains) {
		rm.ErrorJSON(w, "Your Email domain is not allowed", http.StatusBadRequest)
		return
	}

	// Check if username or email already exists
	var count int
	_ = rm.DB.Get(&count, "SELECT COUNT(*) FROM user WHERE lower(name) = ? OR lower(email) = ?", strings.ToLower(username), strings.ToLower(email))
	if count > 0 {
		rm.ErrorJSON(w, "Username or Email already registered", http.StatusBadRequest)
		return
	}

	// Generate a secure random password since email is dropped
	generatedPassword, err := auth.GenerateRandomSalt(12)
	if err != nil {
		rm.ErrorJSON(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	hashedPassword, err := auth.GeneratePasswordHash(generatedPassword)
	if err != nil {
		rm.ErrorJSON(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	_, err = rm.DB.Exec(`INSERT INTO user (
		name, email, role, password, locale, sidebar_view, default_language,
		denied_tags, allowed_tags, denied_column_value, allowed_column_value
	) VALUES (
		?, ?, ?, ?, ?, ?, 'all', ?, ?, ?, ?
	)`,
		username,
		email,
		cfg.ConfigDefaultRole,
		hashedPassword,
		cfg.ConfigDefaultLocale,
		cfg.ConfigDefaultShow,
		cfg.ConfigDeniedTags,
		cfg.ConfigAllowedTags,
		cfg.ConfigDeniedColumnValue,
		cfg.ConfigAllowedColumnValue,
	)
	if err != nil {
		slog.Error("registration: insert user failed", "err", err)
		rm.ErrorJSON(w, "An error occurred during registration", http.StatusInternalServerError)
		return
	}

	rm.WriteJSON(w, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Registration successful! Your generated password is: %s (Write it down, as email is disabled)", generatedPassword),
	})
}

// GetCustomColumns handles GET /api/v1/meta/custom-columns
func (rm *RouteManager) GetCustomColumns(w http.ResponseWriter, r *http.Request) {
	var cols []calibredb.CustomColumnDef
	err := rm.DB.Select(&cols, "SELECT id, label, name, datatype, is_multiple, normalized, display FROM calibre.custom_columns ORDER BY id")
	if err != nil {
		rm.WriteJSON(w, []interface{}{})
		return
	}

	type colResponse struct {
		ID         int         `json:"id"`
		Label      string      `json:"label"`
		Name       string      `json:"name"`
		DataType   string      `json:"datatype"`
		IsMultiple bool        `json:"is_multiple"`
		Display    interface{} `json:"display"`
	}

	resp := make([]colResponse, len(cols))
	for i, c := range cols {
		var disp map[string]interface{}
		if c.Display != "" {
			_ = json.Unmarshal([]byte(c.Display), &disp)
		}
		if disp == nil {
			disp = make(map[string]interface{})
		}
		resp[i] = colResponse{
			ID:         c.ID,
			Label:      c.Label,
			Name:       c.Name,
			DataType:   c.DataType,
			IsMultiple: c.IsMultiple,
			Display:    disp,
		}
	}

	rm.WriteJSON(w, resp)
}

// GetShelves handles GET /api/v1/shelves
func (rm *RouteManager) GetShelves(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r)
	if !ok {
		rm.ErrorJSON(w, "Login required", http.StatusUnauthorized)
		return
	}

	u := user.(*appdb.User)

	type shelfResponse struct {
		ID       int    `db:"id" json:"id"`
		Name     string `db:"name" json:"name"`
		IsPublic bool   `db:"is_public" json:"is_public"`
		KoboSync bool   `db:"kobo_sync" json:"kobo_sync"`
		Count    int    `db:"count" json:"count"`
	}

	var shelves []shelfResponse
	err := rm.DB.Select(&shelves, `
		SELECT s.id, s.name, s.is_public = 1 AS is_public, s.kobo_sync = 1 AS kobo_sync, COUNT(l.book_id) AS count
		FROM shelf s
		LEFT JOIN book_shelf_link l ON l.shelf = s.id
		WHERE s.user_id = ? OR s.is_public = 1
		GROUP BY s.id
		ORDER BY s.name`, u.ID)
	if err != nil {
		rm.WriteJSON(w, []interface{}{})
		return
	}

	rm.WriteJSON(w, shelves)
}
