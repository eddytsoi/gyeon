package smtplog

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/queue"
	"gyeon/backend/internal/respond"
)

// QueueEnqueuer is the minimal slice of queue.Service the handler needs to
// schedule a raw resend job. Declared as an interface so tests can stub it.
type QueueEnqueuer interface {
	Enqueue(ctx context.Context, jobType string, payload []byte, opts ...queue.EnqueueOptions) (string, error)
}

type Handler struct {
	store    *Store
	enqueuer QueueEnqueuer
}

func NewHandler(store *Store, enqueuer QueueEnqueuer) *Handler {
	return &Handler{store: store, enqueuer: enqueuer}
}

func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Get("/{id}", h.get)
	r.Post("/{id}/resend", h.resend)
	return r
}

type listResponse struct {
	Items []Row `json:"items"`
	Total int   `json:"total"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	rows, total, err := h.store.List(r.Context(), ListFilter{
		Status:           q.Get("status"),
		TemplateKey:      q.Get("template_key"),
		TriggerCondition: q.Get("trigger_condition"),
		Recipient:        q.Get("recipient"),
		From:             q.Get("from"),
		To:               q.Get("to"),
		Limit:            limit,
		Offset:           offset,
	})
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, listResponse{Items: rows, Total: total})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	row, err := h.store.Get(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, row)
}

// SendEmailRawJob mirrors the payload consumed by the email package's raw
// send handler. Kept here as a local copy so smtplog doesn't pull in email.
type sendEmailRawJob struct {
	LogID             string `json:"log_id"`
	Recipient         string `json:"recipient"`
	Subject           string `json:"subject"`
	BodyHTML          string `json:"body_html"`
	BodyText          string `json:"body_text"`
	ReplyTo           string `json:"reply_to,omitempty"`
	TriggerCondition  string `json:"trigger_condition"`
	RelatedEntityType string `json:"related_entity_type,omitempty"`
	RelatedEntityID   string `json:"related_entity_id,omitempty"`
}

func (h *Handler) resend(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	row, err := h.store.Get(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}

	payload := sendEmailRawJob{
		LogID:            row.ID,
		Recipient:        row.Recipient,
		Subject:          row.Subject,
		BodyHTML:         row.BodyHTML,
		BodyText:         row.BodyText,
		TriggerCondition: "admin.resend",
	}
	if row.ReplyTo != nil {
		payload.ReplyTo = *row.ReplyTo
	}
	if row.RelatedEntityType != nil {
		payload.RelatedEntityType = *row.RelatedEntityType
	}
	if row.RelatedEntityID != nil {
		payload.RelatedEntityID = *row.RelatedEntityID
	}

	b, err := json.Marshal(payload)
	if err != nil {
		respond.InternalError(w)
		return
	}
	jobID, err := h.enqueuer.Enqueue(r.Context(), queue.JobTypeSendEmailRaw, b)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusAccepted, map[string]string{"queue_job_id": jobID, "smtp_log_id": id})
}
