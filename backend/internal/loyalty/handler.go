package loyalty

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/auth"
	"gyeon/backend/internal/respond"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// CustomerRoutes mounts under /loyalty (customer-auth required).
func (h *Handler) CustomerRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.myBalance)
	r.Get("/ledger", h.myLedger)
	return r
}

// AdminRoutes mounts under /admin/customers/{id}/loyalty.
func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.adminBalance)
	r.Get("/ledger", h.adminLedger)
	r.Post("/adjust", h.adminAdjust)
	return r
}

type balanceResp struct {
	Points int `json:"points"`
}

func (h *Handler) myBalance(w http.ResponseWriter, r *http.Request) {
	cid := auth.CustomerIDFromContext(r.Context())
	if cid == "" {
		respond.Error(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	p, err := h.svc.GetBalance(r.Context(), cid)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, balanceResp{Points: p})
}

func (h *Handler) myLedger(w http.ResponseWriter, r *http.Request) {
	cid := auth.CustomerIDFromContext(r.Context())
	if cid == "" {
		respond.Error(w, http.StatusUnauthorized, "unauthenticated")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := h.svc.Ledger(r.Context(), cid, limit)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, rows)
}

func (h *Handler) adminBalance(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := h.svc.GetBalance(r.Context(), id)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, balanceResp{Points: p})
}

func (h *Handler) adminLedger(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := h.svc.Ledger(r.Context(), id, limit)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, rows)
}

type adjustReq struct {
	Delta  int    `json:"delta"`
	Reason string `json:"reason"`
	Note   string `json:"note"`
}

func (h *Handler) adminAdjust(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var in adjustReq
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	if in.Delta == 0 {
		respond.BadRequest(w, "delta must be non-zero")
		return
	}
	actor, _ := auth.AdminIDFromContext(r.Context())
	balance, err := h.svc.Adjust(r.Context(), id, in.Delta, in.Reason, in.Note, actor)
	if errors.Is(err, ErrInsufficient) {
		respond.Error(w, http.StatusUnprocessableEntity, "insufficient points")
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, balanceResp{Points: balance})
}
