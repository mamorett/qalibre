package routes

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/qalibre/qalibre/internal/appdb"
	"github.com/qalibre/qalibre/internal/auth"
	"github.com/qalibre/qalibre/internal/config"
	"github.com/qalibre/qalibre/internal/dbutil"
)

// GetConfig handles GET /api/v1/config
func (rm *RouteManager) GetConfig(w http.ResponseWriter, r *http.Request) {
	cfg := rm.Cfg.Get()

	calibreDir := ""
	if cfg.ConfigCalibreDir != nil {
		calibreDir = *cfg.ConfigCalibreDir
	}

	rm.WriteJSON(w, map[string]interface{}{
		"config_calibre_dir":       calibreDir,
		"config_books_per_page":    cfg.ConfigBooksPerPage,
		"config_calibre_web_title": cfg.ConfigCalibreWebTitle,
		"config_public_reg":        cfg.ConfigPublicReg == 1,
		"config_uploading":         cfg.ConfigUploading == 1,
		"config_anonbrowse":        cfg.ConfigAnonBrowse == 1,
	})
}

// PostConfig handles POST /api/v1/config
func (rm *RouteManager) PostConfig(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rm.ErrorJSON(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := rm.Cfg.Update(func(s *config.Settings) {
		if dirRaw, ok := req["config_calibre_dir"]; ok {
			dir := strings.TrimSpace(dirRaw.(string))
			s.ConfigCalibreDir = &dir
		}
		if bppRaw, ok := req["config_books_per_page"]; ok {
			// Float64 is default number type in JSON unmarshal
			if val, ok := bppRaw.(float64); ok {
				s.ConfigBooksPerPage = int(val)
			}
		}
		if titleRaw, ok := req["config_calibre_web_title"]; ok {
			s.ConfigCalibreWebTitle = strings.TrimSpace(titleRaw.(string))
		}
		if regRaw, ok := req["config_public_reg"]; ok {
			if val, ok := regRaw.(bool); ok {
				if val {
					s.ConfigPublicReg = 1
				} else {
					s.ConfigPublicReg = 0
				}
			}
		}
		if uploadRaw, ok := req["config_uploading"]; ok {
			if val, ok := uploadRaw.(bool); ok {
				if val {
					s.ConfigUploading = 1
				} else {
					s.ConfigUploading = 0
				}
			}
		}
		if anonRaw, ok := req["config_anonbrowse"]; ok {
			if val, ok := anonRaw.(bool); ok {
				if val {
					s.ConfigAnonBrowse = 1
				} else {
					s.ConfigAnonBrowse = 0
				}
			}
		}
	})

	if err != nil {
		rm.ErrorJSON(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	// Re-ATTACH calibre DB if configured calibre dir changed
	calibreDir := rm.Cfg.GetCalibreDir()
	if calibreDir != "" {
		metaDBPath := filepath.Join(calibreDir, "metadata.db")
		if _, err := os.Stat(metaDBPath); err == nil {
			_ = dbutil.Reconnect(rm.DB, metaDBPath)
		}
	}

	rm.WriteJSON(w, map[string]interface{}{"success": true})
}

// GetUsers handles GET /api/v1/admin/users
func (rm *RouteManager) GetUsers(w http.ResponseWriter, r *http.Request) {
	var users []appdb.User
	err := rm.DB.Select(&users, "SELECT * FROM user ORDER BY name")
	if err != nil {
		rm.ErrorJSON(w, "Failed to retrieve users", http.StatusInternalServerError)
		return
	}

	type userResponse struct {
		ID           int    `json:"id"`
		Name         string `json:"name"`
		Email        string `json:"email"`
		RoleAdmin    bool   `json:"role_admin"`
		RoleEdit     bool   `json:"role_edit"`
		RoleDownload bool   `json:"role_download"`
		RoleUpload   bool   `json:"role_upload"`
		Locale       string `json:"locale"`
	}

	resp := make([]userResponse, len(users))
	for i, u := range users {
		resp[i] = userResponse{
			ID:           u.ID,
			Name:         u.Name,
			Email:        u.Email,
			RoleAdmin:    auth.HasRole(u.Role, config.RoleAdmin),
			RoleEdit:     auth.HasRole(u.Role, config.RoleEdit),
			RoleDownload: auth.HasRole(u.Role, config.RoleDownload),
			RoleUpload:   auth.HasRole(u.Role, config.RoleUpload),
			Locale:       u.Locale,
		}
	}

	rm.WriteJSON(w, resp)
}

// CreateUser handles POST /api/v1/admin/users
func (rm *RouteManager) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rm.ErrorJSON(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	username := strings.TrimSpace(req.Username)
	email := strings.TrimSpace(req.Email)
	password := strings.TrimSpace(req.Password)

	if username == "" || password == "" {
		rm.ErrorJSON(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	// Check if already exists
	var count int
	_ = rm.DB.Get(&count, "SELECT COUNT(*) FROM user WHERE lower(name) = ?", strings.ToLower(username))
	if count > 0 {
		rm.ErrorJSON(w, "User already exists", http.StatusBadRequest)
		return
	}

	hashedPassword, err := auth.GeneratePasswordHash(password)
	if err != nil {
		rm.ErrorJSON(w, "Password hashing failed", http.StatusInternalServerError)
		return
	}

	cfg := rm.Cfg.Get()

	res, err := rm.DB.Exec(`
		INSERT INTO user (
			name, email, password, role, locale, sidebar_view, default_language,
			denied_tags, allowed_tags, denied_column_value, allowed_column_value
		) VALUES (?, ?, ?, ?, ?, ?, 'all', ?, ?, ?, ?)`,
		username,
		email,
		hashedPassword,
		cfg.ConfigDefaultRole,
		cfg.ConfigDefaultLocale,
		cfg.ConfigDefaultShow,
		cfg.ConfigDeniedTags,
		cfg.ConfigAllowedTags,
		cfg.ConfigDeniedColumnValue,
		cfg.ConfigAllowedColumnValue,
	)
	if err != nil {
		slog.Error("failed to create user", "err", err)
		rm.ErrorJSON(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	newID, _ := res.LastInsertId()
	rm.WriteJSON(w, map[string]interface{}{"success": true, "user_id": newID})
}

// DeleteUser handles DELETE /api/v1/admin/users/{id}
func (rm *RouteManager) DeleteUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	userID, err := strconv.Atoi(idStr)
	if err != nil {
		rm.ErrorJSON(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	currentUser, ok := auth.GetUserFromContext(r)
	if !ok {
		rm.ErrorJSON(w, "Login required", http.StatusUnauthorized)
		return
	}

	if userID == currentUser.(*appdb.User).ID {
		rm.ErrorJSON(w, "Cannot delete yourself", http.StatusBadRequest)
		return
	}

	var userCount int
	err = rm.DB.Get(&userCount, "SELECT COUNT(*) FROM user WHERE id = ?", userID)
	if err != nil || userCount == 0 {
		rm.ErrorJSON(w, "User not found", http.StatusNotFound)
		return
	}

	_, err = rm.DB.Exec("DELETE FROM user WHERE id = ?", userID)
	if err != nil {
		slog.Error("failed to delete user", "id", userID, "err", err)
		rm.ErrorJSON(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	rm.WriteJSON(w, map[string]interface{}{"success": true})
}
