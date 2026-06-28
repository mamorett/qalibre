package routes

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	"github.com/qalibre/qalibre/internal/auth"
	"github.com/qalibre/qalibre/internal/config"
)

type RouteManager struct {
	DB    *sqlx.DB
	Cfg   *config.Config
	SM    *auth.SessionManager
	IsDev bool
}

func NewRouteManager(db *sqlx.DB, cfg *config.Config, sm *auth.SessionManager, isDev bool) *RouteManager {
	return &RouteManager{DB: db, Cfg: cfg, SM: sm, IsDev: isDev}
}

// RegisterRoutes registers all Qalibre JSON/AJAX API routes onto the router.
func (rm *RouteManager) RegisterRoutes(r chi.Router) {
	// Public routes (Auth handled inside or optional)
	r.Group(func(pub chi.Router) {
		pub.Use(rm.SM.AuthMiddleware)

		pub.Get("/api/v1/session", rm.GetSession)
		pub.Post("/api/v1/login", rm.Login)
		pub.Post("/api/v1/logout", rm.Logout)
		pub.Post("/api/v1/register", rm.Register)
	})

	// Authenticated routes
	r.Group(func(authGroup chi.Router) {
		authGroup.Use(rm.SM.AuthMiddleware)

		// Optional basic/session auth check
		authGroup.Get("/api/v1/book/{id}", rm.GetBook)
		authGroup.Patch("/api/v1/book/{id}", rm.EditBook)
		authGroup.Get("/api/v1/shelves", rm.GetShelves)
		authGroup.Get("/api/v1/meta/custom-columns", rm.GetCustomColumns)
		authGroup.Get("/api/v1/stats", rm.GetStats)
		authGroup.Get("/ajax/listbooks", rm.ListBooks)

		authGroup.Post("/ajax/toggleread/{id}", rm.ToggleRead)
		authGroup.Post("/ajax/togglearchived/{id}", rm.ToggleArchived)
		authGroup.Get("/ajax/emailstat", rm.GetTasksStatus)
		authGroup.Post("/ajax/canceltask", rm.CancelTask)

		authGroup.Get("/cover/{id}", rm.ServeCover)
		authGroup.Get("/cover/{id}/{res}", rm.ServeCover)
		authGroup.Get("/read/{id}/{fmt}", rm.ReadBook)

		// Download endpoints require download role
		authGroup.Group(func(dl chi.Router) {
			dl.Use(rm.SM.RequireDownload)
			dl.Get("/download/{id}/{fmt}", rm.DownloadBook)
			dl.Get("/download/{id}/{fmt}/{name}", rm.DownloadBook)
		})

		// Upload endpoint requires upload role
		authGroup.Group(func(up chi.Router) {
			up.Use(rm.SM.RequireUpload)
			up.Post("/upload", rm.UploadBook)
		})

		// Admin endpoints require admin role
		authGroup.Group(func(adm chi.Router) {
			adm.Use(rm.SM.RequireAdmin)
			adm.Get("/api/v1/config", rm.GetConfig)
			adm.Post("/api/v1/config", rm.PostConfig)
			adm.Get("/api/v1/admin/users", rm.GetUsers)
			adm.Post("/api/v1/admin/users", rm.CreateUser)
			adm.Delete("/api/v1/admin/users/{id}", rm.DeleteUser)
		})

		// Dataset routes - v1: admins only (see D10)
		authGroup.Group(func(ds chi.Router) {
			ds.Use(rm.SM.RequireAdmin)
			ds.Get("/api/v1/datasets", rm.ListDatasets)
			ds.Post("/api/v1/datasets", rm.CreateDataset)
			ds.Get("/api/v1/dataset/{id}", rm.GetDataset)
			ds.Patch("/api/v1/dataset/{id}", rm.UpdateDataset)
			ds.Delete("/api/v1/dataset/{id}", rm.DeleteDataset)

			ds.Get("/api/v1/dataset/{id}/books", rm.ListDatasetBooks)
			ds.Post("/api/v1/dataset/{id}/books", rm.AddBooksToDataset)
			ds.Delete("/api/v1/dataset/{id}/books/{bookId}", rm.RemoveBookFromDataset)

			ds.Get("/api/v1/dataset/{id}/available-books", rm.ListAvailableBooks)
			ds.Post("/api/v1/dataset/{id}/export", rm.ExportDataset)
			ds.Post("/api/v1/dataset/{id}/export-chunked", rm.ExportDatasetChunked)
		})
	})
}

// Helper to write JSON error
func (rm *RouteManager) ErrorJSON(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// Helper to write JSON success
func (rm *RouteManager) WriteJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}
