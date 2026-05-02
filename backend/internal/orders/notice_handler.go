package orders

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"gyeon/backend/internal/auth"
	"gyeon/backend/internal/email"
	"gyeon/backend/internal/respond"
)

type NoticeHandler struct {
	svc       *NoticeService
	emailSvc  *email.Service
	jwtSecret string // admin JWT secret — used to extract admin user id (token Subject)
}

func NewNoticeHandler(svc *NoticeService, emailSvc *email.Service, adminJWTSecret string) *NoticeHandler {
	return &NoticeHandler{svc: svc, emailSvc: emailSvc, jwtSecret: adminJWTSecret}
}

// CustomerRoutes registers customer-side notice routes. Mount this inside a
// chi group that already applies auth.CustomerMiddleware.
func (h *NoticeHandler) CustomerRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/unread-counts", h.unreadCountsCustomer)
	r.Get("/{orderID}", h.listCustomer)
	r.Post("/{orderID}", h.createCustomer)
	r.Post("/{orderID}/read", h.markReadCustomer)
	return r
}

// AdminRoutes registers admin-side notice routes. Mount inside the existing
// admin auth group.
func (h *NoticeHandler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/unread-counts", h.unreadCountsAdmin)
	r.Get("/{orderID}", h.listAdmin)
	r.Post("/{orderID}", h.createAdmin)
	r.Post("/{orderID}/read", h.markReadAdmin)
	return r
}

// ---------- customer-side ----------

func (h *NoticeHandler) listCustomer(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	customerID := auth.CustomerIDFromContext(r.Context())
	if customerID == "" {
		respond.Error(w, http.StatusUnauthorized, "missing customer")
		return
	}
	owned, err := h.svc.OrderOwnedByCustomer(r.Context(), orderID, customerID)
	if err != nil {
		respond.InternalError(w)
		return
	}
	if !owned {
		respond.NotFound(w)
		return
	}
	notices, err := h.svc.List(r.Context(), orderID, true)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, notices)
}

func (h *NoticeHandler) createCustomer(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	customerID := auth.CustomerIDFromContext(r.Context())
	if customerID == "" {
		respond.Error(w, http.StatusUnauthorized, "missing customer")
		return
	}
	owned, err := h.svc.OrderOwnedByCustomer(r.Context(), orderID, customerID)
	if err != nil {
		respond.InternalError(w)
		return
	}
	if !owned {
		respond.NotFound(w)
		return
	}
	var body struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	body.Body = strings.TrimSpace(body.Body)
	if body.Body == "" {
		respond.BadRequest(w, "body is required")
		return
	}
	notice, err := h.svc.CreateCustomerMessage(r.Context(), orderID, customerID, body.Body)
	if err != nil {
		if errors.Is(err, ErrNoticeBodyEmpty) {
			respond.BadRequest(w, "body is required")
			return
		}
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusCreated, notice)
}

func (h *NoticeHandler) markReadCustomer(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	customerID := auth.CustomerIDFromContext(r.Context())
	if customerID == "" {
		respond.Error(w, http.StatusUnauthorized, "missing customer")
		return
	}
	owned, err := h.svc.OrderOwnedByCustomer(r.Context(), orderID, customerID)
	if err != nil {
		respond.InternalError(w)
		return
	}
	if !owned {
		respond.NotFound(w)
		return
	}
	if err := h.svc.MarkAdminNoticesRead(r.Context(), orderID); err != nil {
		respond.InternalError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NoticeHandler) unreadCountsCustomer(w http.ResponseWriter, r *http.Request) {
	customerID := auth.CustomerIDFromContext(r.Context())
	if customerID == "" {
		respond.Error(w, http.StatusUnauthorized, "missing customer")
		return
	}
	counts, err := h.svc.UnreadCountsForCustomer(r.Context(), customerID)
	if err != nil {
		respond.InternalError(w)
		return
	}
	if counts == nil {
		counts = map[string]int{}
	}
	respond.JSON(w, http.StatusOK, counts)
}

// ---------- admin-side ----------

func (h *NoticeHandler) listAdmin(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	notices, err := h.svc.List(r.Context(), orderID, false)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, notices)
}

func (h *NoticeHandler) createAdmin(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	adminID := h.adminIDFromRequest(r)

	var req struct {
		Role string `json:"role"`
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	req.Body = strings.TrimSpace(req.Body)
	if req.Body == "" {
		respond.BadRequest(w, "body is required")
		return
	}

	var notice *Notice
	var err error
	switch NoticeRole(req.Role) {
	case NoticeRoleSystem:
		notice, err = h.svc.CreateSystemNotice(r.Context(), orderID, req.Body)
	case NoticeRoleAdmin:
		if adminID == "" {
			respond.Error(w, http.StatusUnauthorized, "missing admin")
			return
		}
		notice, err = h.svc.CreateAdminMessage(r.Context(), orderID, adminID, req.Body)
		if err == nil {
			go h.sendAdminMessageEmail(orderID, req.Body)
		}
	default:
		respond.BadRequest(w, "role must be 'system' or 'admin'")
		return
	}
	if err != nil {
		if errors.Is(err, ErrNoticeBodyEmpty) {
			respond.BadRequest(w, "body is required")
			return
		}
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusCreated, notice)
}

func (h *NoticeHandler) markReadAdmin(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")
	if err := h.svc.MarkCustomerNoticesRead(r.Context(), orderID); err != nil {
		respond.InternalError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NoticeHandler) unreadCountsAdmin(w http.ResponseWriter, r *http.Request) {
	counts, err := h.svc.UnreadCountsForAdmin(r.Context())
	if err != nil {
		respond.InternalError(w)
		return
	}
	if counts == nil {
		counts = map[string]int{}
	}
	respond.JSON(w, http.StatusOK, counts)
}

// adminIDFromRequest pulls the admin user id (token Subject) out of the
// Authorization bearer header. Returns "" if absent or invalid; the caller
// decides whether that's fatal — author_id is nullable so we can record a
// system notice without it.
func (h *NoticeHandler) adminIDFromRequest(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	tokenStr := strings.TrimPrefix(header, "Bearer ")
	claims, err := auth.ValidateToken(tokenStr, h.jwtSecret)
	if err != nil {
		return ""
	}
	// GenerateAdminToken stores the admin user id in RegisteredClaims.Subject
	return claims.Subject
}

// sendAdminMessageEmail fires off the customer notification email. Best-effort:
// runs in a goroutine and logs failures rather than bubbling them up to the
// HTTP response.
func (h *NoticeHandler) sendAdminMessageEmail(orderID, body string) {
	if h.emailSvc == nil {
		return
	}
	ctx := context.Background()
	customerEmail, customerName, orderNumber, err := h.svc.CustomerEmailForOrder(ctx, orderID)
	if err != nil || customerEmail == "" {
		if err != nil {
			log.Printf("notice email: lookup customer for order %s: %v", orderID, err)
		}
		return
	}
	base := h.emailSvc.PublicBaseURL(ctx)
	orderURL := fmt.Sprintf("%s/account/orders/%s", base, orderID)

	if err := h.emailSvc.SendAdminMessageNotification(ctx, email.AdminMessageParams{
		To:           customerEmail,
		CustomerName: customerName,
		OrderNumber:  orderNumber,
		OrderURL:     orderURL,
		Body:         body,
	}); err != nil {
		log.Printf("notice email: send admin message for order %s: %v", orderID, err)
	}
}
