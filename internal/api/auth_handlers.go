package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"walrus/internal/apperr"
	"walrus/internal/auth"
	"walrus/internal/db"
)

func (s *Server) handleAuthStatus(c *gin.Context) {
	count, err := s.store.CountUsers()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"initialized":  count > 0,
		"runtime_mode": s.driver.Mode(),
	})
}

type setupRequest struct {
	Username            string `json:"username" binding:"required,min=3,max=32"`
	Password            string `json:"password" binding:"required,min=6,max=128"`
	DefaultClusterToken string `json:"default_cluster_token"`
	Locale              string `json:"locale"`
}

func (s *Server) handleSetup(c *gin.Context) {
	count, err := s.store.CountUsers()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	if count > 0 {
		respondCodeError(c, http.StatusConflict, apperr.CodeSystemAlreadyInitialized, "system is already initialized")
		return
	}

	var req setupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBindError(c, err)
		return
	}

	hash, err := s.authSvc.HashPassword(req.Password)
	if err != nil {
		respondCodeError(c, http.StatusInternalServerError, apperr.CodePasswordHashFailed, "failed to hash password")
		return
	}
	u, err := s.store.CreateFirstAdmin(strings.TrimSpace(req.Username), hash, req.Locale)
	if errors.Is(err, db.ErrAlreadyInitialized) {
		respondCodeError(c, http.StatusConflict, apperr.CodeSystemAlreadyInitialized, "system is already initialized")
		return
	}
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.DefaultClusterToken) != "" {
		_ = s.store.SetSetting("default_cluster_token", strings.TrimSpace(req.DefaultClusterToken))
	}

	tok, err := s.authSvc.IssueToken(u)
	if err != nil {
		respondCodeError(c, http.StatusInternalServerError, apperr.CodeTokenIssueFailed, "failed to issue token")
		return
	}
	s.authSvc.SetAuthCookie(c, tok)
	s.store.AddAuditLog(u.Username, "system.setup", "admin", "Initial admin account created", c.ClientIP())

	c.JSON(http.StatusCreated, gin.H{
		"user":  u,
		"token": tok,
	})
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (s *Server) handleLogin(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBindError(c, err)
		return
	}

	u, tok, err := s.authSvc.Login(req.Username, req.Password, c.ClientIP())
	if err != nil {
		status := http.StatusUnauthorized
		if errors.Is(err, auth.ErrRateLimited) {
			status = http.StatusTooManyRequests
		} else if errors.Is(err, auth.ErrAccountDisabled) {
			status = http.StatusForbidden
		}
		respondError(c, status, err)
		return
	}

	s.authSvc.SetAuthCookie(c, tok)
	s.store.AddAuditLog(u.Username, "auth.login", u.Username, "User logged in", c.ClientIP())
	c.JSON(http.StatusOK, gin.H{
		"user":  u,
		"token": tok,
	})
}

func (s *Server) handleLogout(c *gin.Context) {
	s.authSvc.ClearAuthCookie(c)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleMe(c *gin.Context) {
	u := auth.CurrentUser(c)
	perms, _ := s.store.GetUserClusterPermissions(u.ID)
	if perms == nil {
		perms = []db.UserClusterPermission{}
	}
	c.JSON(http.StatusOK, gin.H{
		"user":        u,
		"permissions": perms,
	})
}

// --- User Management (Admin only) ---

type userWithPerms struct {
	db.User
	Permissions []db.UserClusterPermission `json:"permissions"`
}

func (s *Server) handleListUsers(c *gin.Context) {
	users, err := s.store.ListUsers()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	out := make([]userWithPerms, 0, len(users))
	for _, u := range users {
		perms, _ := s.store.GetUserClusterPermissions(u.ID)
		if perms == nil {
			perms = []db.UserClusterPermission{}
		}
		out = append(out, userWithPerms{User: u, Permissions: perms})
	}
	c.JSON(http.StatusOK, gin.H{"users": out})
}

type createUserRequest struct {
	Username    string                     `json:"username" binding:"required,min=3,max=32"`
	Password    string                     `json:"password" binding:"required,min=6,max=128"`
	Role        db.Role                    `json:"role" binding:"required"`
	Locale      string                     `json:"locale"`
	Permissions []db.UserClusterPermission `json:"permissions"`
}

func (s *Server) handleCreateUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBindError(c, err)
		return
	}
	if !req.Role.Valid() {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeInvalidRole, "invalid role")
		return
	}
	hash, err := s.authSvc.HashPassword(req.Password)
	if err != nil {
		respondCodeError(c, http.StatusInternalServerError, apperr.CodePasswordHashFailed, "password hashing failed")
		return
	}
	u, err := s.store.CreateUser(strings.TrimSpace(req.Username), hash, req.Role, req.Locale)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	if len(req.Permissions) > 0 {
		_ = s.store.SetUserClusterPermissions(u.ID, req.Permissions)
	}
	actor := auth.CurrentUser(c)
	s.store.AddAuditLog(actor.Username, "user.create", u.Username, "Role="+string(u.Role), c.ClientIP())
	c.JSON(http.StatusCreated, gin.H{"user": u})
}

type updateUserRequest struct {
	Role        db.Role                     `json:"role" binding:"required"`
	Disabled    bool                        `json:"disabled"`
	Password    string                      `json:"password"`
	Locale      string                      `json:"locale"`
	Permissions *[]db.UserClusterPermission `json:"permissions"`
}

func (s *Server) handleUpdateUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("uid"), 10, 64)
	if err != nil {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeInvalidUserID, "invalid user id")
		return
	}
	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBindError(c, err)
		return
	}
	if !req.Role.Valid() {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeInvalidRole, "invalid role")
		return
	}

	actor := auth.CurrentUser(c)
	if actor.ID == id && (req.Disabled || req.Role != db.RoleAdmin) {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeCannotDisableSelf, "cannot disable or demote your own admin account")
		return
	}

	var hash string
	if strings.TrimSpace(req.Password) != "" {
		if len(req.Password) < 6 {
			respondCodeError(c, http.StatusBadRequest, apperr.CodePasswordTooShort, "password must be at least 6 characters")
			return
		}
		hash, err = s.authSvc.HashPassword(req.Password)
		if err != nil {
			respondCodeError(c, http.StatusInternalServerError, apperr.CodePasswordHashFailed, "password hashing failed")
			return
		}
	}

	if err := s.store.UpdateUser(id, req.Role, req.Disabled, hash, req.Locale); err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	if req.Permissions != nil {
		_ = s.store.SetUserClusterPermissions(id, *req.Permissions)
	}
	target, _ := s.store.GetUserByID(id)
	targetName := strconv.FormatInt(id, 10)
	if target != nil {
		targetName = target.Username
	}
	s.store.AddAuditLog(actor.Username, "user.update", targetName, "Role="+string(req.Role), c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"user": target})
}

func (s *Server) handleDeleteUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("uid"), 10, 64)
	if err != nil {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeInvalidUserID, "invalid user id")
		return
	}
	actor := auth.CurrentUser(c)
	if actor.ID == id {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeCannotDeleteSelf, "cannot delete your own account")
		return
	}
	target, _ := s.store.GetUserByID(id)
	if err := s.store.DeleteUser(id); err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	targetName := strconv.FormatInt(id, 10)
	if target != nil {
		targetName = target.Username
	}
	s.store.AddAuditLog(actor.Username, "user.delete", targetName, "User deleted", c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
