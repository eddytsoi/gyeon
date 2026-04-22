package auth

import (
	"encoding/json"
	"net/http"

	"gyeon/backend/internal/respond"
)

type Handler struct {
	secret   string
	password string
}

func NewHandler(secret, password string) *Handler {
	return &Handler{secret: secret, password: password}
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	if body.Password != h.password {
		respond.Error(w, http.StatusUnauthorized, "invalid password")
		return
	}
	token, err := GenerateToken(h.secret)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"token": token})
}
