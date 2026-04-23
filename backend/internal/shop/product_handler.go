package shop

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/respond"
)

type ProductHandler struct {
	svc *ProductService
}

func NewProductHandler(svc *ProductService) *ProductHandler {
	return &ProductHandler{svc: svc}
}

func (h *ProductHandler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/low-stock", h.lowStock)
	return r
}

func (h *ProductHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Get("/{id}", h.getByID)
	r.Put("/{id}", h.update)
	r.Delete("/{id}", h.delete)

	// Variant sub-routes
	r.Get("/{id}/variants", h.listVariants)
	r.Post("/{id}/variants", h.createVariant)
	r.Put("/{id}/variants/{variantID}", h.updateVariant)
	r.Delete("/{id}/variants/{variantID}", h.deleteVariant)
	r.Post("/{id}/variants/{variantID}/stock", h.adjustStock)

	// Image sub-routes
	r.Get("/{id}/images", h.listImages)
	r.Post("/{id}/images", h.addImage)
	r.Put("/{id}/images/{imageID}", h.updateImage)
	r.Delete("/{id}/images/{imageID}", h.deleteImage)

	// Translation sub-routes (admin-only translation management + public read is via ?lang=)
	r.Get("/{id}/translations", h.listTranslations)
	r.Put("/{id}/translations/{locale}", h.upsertTranslation)
	r.Delete("/{id}/translations/{locale}", h.deleteTranslation)
	return r
}

func (h *ProductHandler) list(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	products, err := h.svc.List(r.Context(), r.URL.Query().Get("lang"), limit, offset)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, products)
}

func (h *ProductHandler) getByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	product, err := h.svc.GetByID(r.Context(), id, r.URL.Query().Get("lang"))
	if err != nil {
		respond.NotFound(w)
		return
	}
	respond.JSON(w, http.StatusOK, product)
}

func (h *ProductHandler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	product, err := h.svc.Create(r.Context(), req)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusCreated, product)
}

func (h *ProductHandler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	product, err := h.svc.Update(r.Context(), id, req)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, product)
}

func (h *ProductHandler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		respond.InternalError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProductHandler) listVariants(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	variants, err := h.svc.ListVariants(r.Context(), id)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, variants)
}

func (h *ProductHandler) createVariant(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req CreateVariantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	variant, err := h.svc.CreateVariant(r.Context(), id, req)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusCreated, variant)
}

func (h *ProductHandler) updateVariant(w http.ResponseWriter, r *http.Request) {
	variantID := chi.URLParam(r, "variantID")
	var req UpdateVariantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	variant, err := h.svc.UpdateVariant(r.Context(), variantID, req)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, variant)
}

func (h *ProductHandler) deleteVariant(w http.ResponseWriter, r *http.Request) {
	variantID := chi.URLParam(r, "variantID")
	if err := h.svc.DeleteVariant(r.Context(), variantID); err != nil {
		respond.InternalError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProductHandler) adjustStock(w http.ResponseWriter, r *http.Request) {
	variantID := chi.URLParam(r, "variantID")
	var req AdjustStockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	variant, err := h.svc.AdjustStock(r.Context(), variantID, req)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, variant)
}

func (h *ProductHandler) listImages(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	images, err := h.svc.ListImages(r.Context(), id)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, images)
}

func (h *ProductHandler) addImage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req AddImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	image, err := h.svc.AddImage(r.Context(), id, req)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusCreated, image)
}

func (h *ProductHandler) updateImage(w http.ResponseWriter, r *http.Request) {
	imageID := chi.URLParam(r, "imageID")
	var req UpdateImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	image, err := h.svc.UpdateImage(r.Context(), imageID, req)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, image)
}

func (h *ProductHandler) deleteImage(w http.ResponseWriter, r *http.Request) {
	imageID := chi.URLParam(r, "imageID")
	if err := h.svc.DeleteImage(r.Context(), imageID); err != nil {
		respond.InternalError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProductHandler) lowStock(w http.ResponseWriter, r *http.Request) {
	threshold, _ := strconv.Atoi(r.URL.Query().Get("threshold"))
	if threshold <= 0 {
		threshold = 5
	}
	variants, err := h.svc.LowStock(r.Context(), threshold)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, variants)
}

func (h *ProductHandler) listTranslations(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	translations, err := h.svc.ListTranslations(r.Context(), id)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, translations)
}

func (h *ProductHandler) upsertTranslation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	locale := chi.URLParam(r, "locale")
	var req UpsertProductTranslationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}
	t, err := h.svc.UpsertTranslation(r.Context(), id, locale, req)
	if err != nil {
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusOK, t)
}

func (h *ProductHandler) deleteTranslation(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	locale := chi.URLParam(r, "locale")
	if err := h.svc.DeleteTranslation(r.Context(), id, locale); err != nil {
		if errors.Is(err, errProductNotFound) {
			respond.NotFound(w)
			return
		}
		respond.InternalError(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
