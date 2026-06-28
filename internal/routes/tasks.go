package routes

import (
	"encoding/json"
	"net/http"

	"github.com/qalibre/qalibre/internal/appdb"
	"github.com/qalibre/qalibre/internal/auth"
	"github.com/qalibre/qalibre/internal/config"
	"github.com/qalibre/qalibre/internal/worker"
)

// GetTasksStatus handles GET /ajax/emailstat
func (rm *RouteManager) GetTasksStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r)
	if !ok {
		rm.ErrorJSON(w, "Login required", http.StatusUnauthorized)
		return
	}
	u := user.(*appdb.User)

	isAdmin := auth.HasRole(u.Role, config.RoleAdmin)
	wMgr := worker.GetInstance(rm.DB)
	list := wMgr.GetTasks(u.Name, isAdmin)

	rm.WriteJSON(w, list)
}

// CancelTask handles POST /ajax/canceltask
func (rm *RouteManager) CancelTask(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r)
	if !ok {
		rm.ErrorJSON(w, "Login required", http.StatusUnauthorized)
		return
	}
	u := user.(*appdb.User)

	var req struct {
		TaskID string `json:"task_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rm.ErrorJSON(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	wMgr := worker.GetInstance(rm.DB)

	// If not admin, check if the task belongs to the current user
	if !auth.HasRole(u.Role, config.RoleAdmin) {
		tasksList := wMgr.GetTasks(u.Name, false)
		found := false
		for _, t := range tasksList {
			if idVal, ok := t["task_id"].(string); ok && idVal == req.TaskID {
				found = true
				break
			}
		}
		if !found {
			rm.ErrorJSON(w, "Access denied: task not found or does not belong to you", http.StatusForbidden)
			return
		}
	}

	wMgr.CancelTask(req.TaskID)

	rm.WriteJSON(w, map[string]interface{}{"success": true})
}
