package auth

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/replacearr/replacearr/internal/domain"
)

func (s *Service) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/v1/auth/logout", s.handleLogout)
	mux.HandleFunc("GET /api/v1/auth/me", s.RequirePermission("", s.handleMe))

	mux.HandleFunc("GET /api/v1/users", s.RequirePermission(domain.PermUsersRead, s.handleListUsers))
	mux.HandleFunc("POST /api/v1/users", s.RequirePermission(domain.PermUsersManage, s.handleCreateUser))
	mux.HandleFunc("PUT /api/v1/users/{id}", s.RequirePermission(domain.PermUsersManage, s.handleUpdateUser))
	mux.HandleFunc("DELETE /api/v1/users/{id}", s.RequirePermission(domain.PermUsersManage, s.handleDeleteUser))

	mux.HandleFunc("GET /api/v1/roles", s.RequirePermission(domain.PermUsersRead, s.handleListRoles))
	mux.HandleFunc("POST /api/v1/roles", s.RequirePermission(domain.PermUsersManage, s.handleCreateRole))
	mux.HandleFunc("PUT /api/v1/roles/{id}", s.RequirePermission(domain.PermUsersManage, s.handleUpdateRole))

	mux.HandleFunc("GET /api/v1/apikeys", s.RequirePermission(domain.PermUsersRead, s.handleListAPIKeys))
	mux.HandleFunc("POST /api/v1/apikeys", s.RequirePermission(domain.PermUsersManage, s.handleCreateAPIKey))
	mux.HandleFunc("DELETE /api/v1/apikeys/{id}", s.RequirePermission(domain.PermUsersManage, s.handleDeleteAPIKey))

	mux.HandleFunc("GET /api/v1/permissions", s.RequirePermission("", s.handleListPermissions))
}

func (s *Service) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	user, token, err := s.Authenticate(req.Username, req.Password)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "replacearr_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})

	masterKey := ""
	if user.RoleName == "admin" || s.HasPermission(user, nil, domain.PermAdminAll) {
		masterKey = s.GetMasterAPIKey()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"token":        token,
		"user":         user,
		"masterApiKey": masterKey,
	})
}

func (s *Service) handleLogout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "replacearr_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Service) handleMe(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	apiKey := APIKeyFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	masterKey := ""
	if user.RoleName == "admin" || s.HasPermission(user, apiKey, domain.PermAdminAll) {
		masterKey = s.GetMasterAPIKey()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":           user.ID,
		"username":     user.Username,
		"email":        user.Email,
		"roleId":       user.RoleID,
		"roleName":     user.RoleName,
		"isActive":     user.IsActive,
		"user":         user,
		"role":         user.RoleName,
		"permissions":  user.Permissions,
		"masterApiKey": masterKey,
	})
}

func (s *Service) handleListUsers(w http.ResponseWriter, _ *http.Request) {
	users, err := s.store.ListUsers()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if users == nil {
		users = []domain.User{}
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Service) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
		RoleID   int64  `json:"roleId"`
		RoleName string `json:"roleName"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username and password are required"})
		return
	}

	roleID := req.RoleID
	if roleID <= 0 {
		roleName := strings.TrimSpace(req.RoleName)
		if roleName == "" {
			roleName = strings.TrimSpace(req.Role)
		}
		if roleName == "" {
			roleName = "viewer"
		}
		role, err := s.store.GetRoleByName(roleName)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid role"})
			return
		}
		roleID = role.ID
	}

	hash, err := s.HashPassword(req.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to hash password"})
		return
	}

	u, err := s.store.CreateUser(req.Username, req.Email, hash, roleID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, u)
}

func (s *Service) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}

	existing, err := s.store.GetUserByID(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}

	var req struct {
		Email    *string `json:"email"`
		RoleID   *int64  `json:"roleId"`
		RoleName string  `json:"roleName"`
		Role     string  `json:"role"`
		IsActive *bool   `json:"isActive"`
		Password string  `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	email := existing.Email
	if req.Email != nil {
		email = *req.Email
	}

	roleID := existing.RoleID
	if req.RoleID != nil && *req.RoleID > 0 {
		roleID = *req.RoleID
	} else if rName := strings.TrimSpace(req.RoleName); rName != "" {
		if role, err := s.store.GetRoleByName(rName); err == nil {
			roleID = role.ID
		}
	} else if rName := strings.TrimSpace(req.Role); rName != "" {
		if role, err := s.store.GetRoleByName(rName); err == nil {
			roleID = role.ID
		}
	}

	isActive := existing.IsActive
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	passHash := ""
	if req.Password != "" {
		h, err := s.HashPassword(req.Password)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to hash password"})
			return
		}
		passHash = h
	}

	updated, err := s.store.UpdateUser(id, email, roleID, isActive, passHash)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Service) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid user id"})
		return
	}
	if err := s.store.DeleteUser(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted", "id": id})
}

func (s *Service) handleListRoles(w http.ResponseWriter, _ *http.Request) {
	roles, err := s.store.ListRoles()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if roles == nil {
		roles = []domain.Role{}
	}
	writeJSON(w, http.StatusOK, roles)
}

func (s *Service) handleCreateRole(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Permissions []string `json:"permissions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "role name is required"})
		return
	}

	role, err := s.store.CreateRole(req.Name, req.Description, req.Permissions)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, role)
}

func (s *Service) handleUpdateRole(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid role id"})
		return
	}

	var req struct {
		Description string   `json:"description"`
		Permissions []string `json:"permissions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	role, err := s.store.UpdateRole(id, req.Description, req.Permissions)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, role)
}

func (s *Service) handleListAPIKeys(w http.ResponseWriter, _ *http.Request) {
	keys, err := s.store.ListAPIKeys()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if keys == nil {
		keys = []domain.APIKey{}
	}
	writeJSON(w, http.StatusOK, keys)
}

func (s *Service) handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string   `json:"name"`
		UserID      int64    `json:"userId"`
		Scope       string   `json:"scope"`
		Permissions []string `json:"permissions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	userID := req.UserID
	if userID <= 0 {
		if u := UserFromContext(r.Context()); u != nil {
			userID = u.ID
		} else {
			userID = 1
		}
	}

	apiKey, err := s.GenerateAPIKey(req.Name, userID, req.Scope, req.Permissions)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, apiKey)
}

func (s *Service) handleDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid api key id"})
		return
	}
	if err := s.store.DeleteAPIKey(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "deleted", "id": id})
}

func (s *Service) handleListPermissions(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, domain.AllPermissions)
}
