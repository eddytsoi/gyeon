package forms

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/respond"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// PublicRoutes are accessible without authentication. The form definition
// returned here strips admin-only fields (mail templates, secrets).
func (h *Handler) PublicRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{slug}", h.publicGet)
	r.Post("/{slug}/submit", h.submit)
	return r
}

func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Get("/{id}", h.adminGet)
	r.Put("/{id}", h.update)
	r.Delete("/{id}", h.delete)

	r.Get("/{id}/submissions", h.listSubmissions)
	r.Get("/{id}/submissions.csv", h.exportSubmissionsCSV)
	r.Get("/submissions/{sid}", h.getSubmission)
	r.Delete("/submissions/{sid}", h.deleteSubmission)
	return r
}

// ─────────── public ───────────

func (h *Handler) publicGet(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	f, err := h.svc.GetBySlug(r.Context(), slug)
	if errors.Is(err, ErrNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, f.Public())
}

func (h *Handler) submit(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	var req SubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	if req.Data == nil {
		req.Data = map[string]string{}
	}
	sub, form, err := h.svc.Submit(r.Context(), slug, clientIP(r), r.UserAgent(), req)
	if errors.Is(err, ErrNotFound) {
		respond.NotFound(w)
		return
	}
	if errors.Is(err, ErrRecaptcha) {
		// 422 here so the frontend can show a friendly "couldn't verify"
		// message — same shape as field-validation errors.
		errMsg := "Verification failed. Please try again."
		if form != nil {
			errMsg = form.ErrorMessage
		}
		respond.JSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error": errMsg, "code": "recaptcha_failed",
		})
		return
	}
	var verrs ValidationErrors
	if errors.As(err, &verrs) {
		respond.JSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error": "validation failed", "fields": verrs,
		})
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, SubmitResponse{
		OK:      true,
		Message: form.SuccessMessage,
	})
	_ = sub // submission id deliberately not returned — keep response minimal.
}

// ─────────── admin ───────────

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	forms, total, err := h.svc.List(r.Context(), limit, offset)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"items": forms,
		"total": total,
	})
}

func (h *Handler) adminGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f, err := h.svc.GetByID(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, f)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req UpsertFormRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	f, parseErrs, err := h.svc.Create(r.Context(), req)
	if errors.Is(err, ErrSlugExists) {
		respond.JSON(w, http.StatusConflict, map[string]string{"error": "slug already exists"})
		return
	}
	if verrs, ok := err.(ValidationErrors); ok {
		respond.JSON(w, http.StatusUnprocessableEntity, map[string]any{"fields": verrs})
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	if len(parseErrs) > 0 {
		respond.JSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":        "form markup contains errors",
			"parse_errors": parseErrs,
		})
		return
	}
	respond.JSON(w, http.StatusCreated, f)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpsertFormRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	f, parseErrs, err := h.svc.Update(r.Context(), id, req)
	if errors.Is(err, ErrNotFound) {
		respond.NotFound(w)
		return
	}
	if errors.Is(err, ErrSlugExists) {
		respond.JSON(w, http.StatusConflict, map[string]string{"error": "slug already exists"})
		return
	}
	if verrs, ok := err.(ValidationErrors); ok {
		respond.JSON(w, http.StatusUnprocessableEntity, map[string]any{"fields": verrs})
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	if len(parseErrs) > 0 {
		respond.JSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":        "form markup contains errors",
			"parse_errors": parseErrs,
		})
		return
	}
	respond.JSON(w, http.StatusOK, f)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.NotFound(w)
			return
		}
		respond.InternalError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listSubmissions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	page, err := h.svc.ListSubmissions(r.Context(), id, limit, offset)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, page)
}

func (h *Handler) exportSubmissionsCSV(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	form, err := h.svc.GetByID(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	// Cap CSV export at 5000 rows — admins exporting larger sets should hit
	// the DB directly. Mirrors how analytics top-N caps are applied.
	page, err := h.svc.ListSubmissions(r.Context(), id, 5000, 0)
	if err != nil {
		respond.InternalError(w)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s-submissions.csv"`, form.Slug))

	cw := csv.NewWriter(w)

	// Columns: created_at, ip, [field1, field2, ...]. Field order comes from
	// the form spec to keep CSVs consistent across exports.
	header := []string{"created_at", "ip"}
	fieldOrder := make([]string, 0, len(form.Fields))
	for _, f := range form.Fields {
		if f.Type == FieldSubmit {
			continue
		}
		fieldOrder = append(fieldOrder, f.Name)
	}
	// In case submissions contain stale fields (renamed/removed since), still
	// include those at the end for traceability.
	extra := map[string]bool{}
	for _, s := range page.Items {
		for k := range s.Data {
			if !containsString(fieldOrder, k) && !extra[k] {
				extra[k] = true
			}
		}
	}
	if len(extra) > 0 {
		more := make([]string, 0, len(extra))
		for k := range extra {
			more = append(more, k)
		}
		sort.Strings(more)
		fieldOrder = append(fieldOrder, more...)
	}
	header = append(header, fieldOrder...)
	_ = cw.Write(header)

	for _, s := range page.Items {
		row := []string{s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"), s.IP}
		for _, name := range fieldOrder {
			row = append(row, s.Data[name])
		}
		_ = cw.Write(row)
	}
	cw.Flush()
}

func (h *Handler) getSubmission(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "sid")
	sub, err := h.svc.GetSubmission(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, sub)
}

func (h *Handler) deleteSubmission(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "sid")
	if err := h.svc.DeleteSubmission(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			respond.NotFound(w)
			return
		}
		respond.InternalError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─────────── helpers ───────────

func clientIP(r *http.Request) string {
	// chi's middleware.RealIP rewrites RemoteAddr from X-Forwarded-For when
	// present, so the value here is already the trusted client IP. Strip the
	// port so the DB INET column accepts it.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}

func containsString(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
