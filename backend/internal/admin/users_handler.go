package admin

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/auth"
	"gyeon/backend/internal/respond"
)

type UserHandler struct {
	svc       *UserService
	jwtSecret string
}

func NewUserHandler(svc *UserService, jwtSecret string) *UserHandler {
	return &UserHandler{svc: svc, jwtSecret: jwtSecret}
}

// LoginRoute returns the login handler (public, replaces single-password login)
func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req AdminLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	user, err := h.svc.Login(r.Context(), req)
	if errors.Is(err, ErrInvalidCredentials) {
		respond.Error(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	tv, _ := h.svc.TokenVersion(r.Context(), user.ID)
	token, err := auth.GenerateAdminToken(h.jwtSecret, user.ID, user.Role, tv)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]interface{}{
		"user":  user,
		"token": token,
	})
}

// AdminRoutes — admin user management. The whole subtree must be mounted
// behind a super_admin role gate (see auth.RequireRole) so editor/viewer
// admins can't escalate by creating themselves a new account.
func (h *UserHandler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Put("/{id}", h.update)
	r.Delete("/{id}", h.delete)
	return r
}

// SignOutEverywhere increments the calling admin's token_version, killing
// every previously issued admin JWT for that user (including the current
// one). Mounted at /admin/me/sign-out-everywhere inside the admin auth group.
func (h *UserHandler) SignOutEverywhere(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.AdminIDFromContext(r.Context())
	if !ok || id == "" {
		respond.Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	if _, err := h.svc.IncrementTokenVersion(r.Context(), id); err != nil {
		respond.InternalError(w)
		return
	}
	auth.InvalidateAdminVersion(id)
	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) list(w http.ResponseWriter, r *http.Request) {
	users, err := h.svc.List(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, users)
}

func (h *UserHandler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateAdminUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	user, err := h.svc.Create(r.Context(), req)
	if errors.Is(err, ErrEmailTaken) {
		respond.Error(w, http.StatusConflict, "email already registered")
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusCreated, user)
}

func (h *UserHandler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdateAdminUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	user, err := h.svc.Update(r.Context(), id, req)
	if errors.Is(err, ErrUserNotFound) {
		respond.NotFound(w)
		return
	}
	if errors.Is(err, ErrSelfModification) {
		respond.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, user)
}

func (h *UserHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, ErrUserNotFound) {
			respond.NotFound(w)
			return
		}
		if errors.Is(err, ErrSelfModification) {
			respond.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		respond.InternalError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
