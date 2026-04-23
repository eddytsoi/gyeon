package media

import (
	"database/sql"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/respond"
)

const maxUploadSize = 20 << 20 // 20 MB
const uploadsDir = "./uploads"

type MediaFile struct {
	ID           string `json:"id"`
	Filename     string `json:"filename"`
	OriginalName string `json:"original_name"`
	MimeType     string `json:"mime_type"`
	SizeBytes    int64  `json:"size_bytes"`
	URL          string `json:"url"`
	CreatedAt    string `json:"created_at"`
}

type Handler struct {
	db      *sql.DB
	baseURL string
}

func NewHandler(db *sql.DB, baseURL string) *Handler {
	os.MkdirAll(uploadsDir, 0755)
	return &Handler{db: db, baseURL: baseURL}
}

func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Post("/upload", h.upload)
	r.Delete("/{id}", h.delete)
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(),
		`SELECT id, filename, original_name, mime_type, size_bytes, url, created_at
		 FROM media_files ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		respond.InternalError(w)
		return
	}
	defer rows.Close()

	files := make([]MediaFile, 0)
	for rows.Next() {
		var f MediaFile
		if err := rows.Scan(&f.ID, &f.Filename, &f.OriginalName, &f.MimeType,
			&f.SizeBytes, &f.URL, &f.CreatedAt); err != nil {
			respond.InternalError(w)
			return
		}
		files = append(files, f)
	}
	respond.JSON(w, http.StatusOK, files)
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		respond.BadRequest(w, "file too large (max 20 MB)")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respond.BadRequest(w, "file field is required")
		return
	}
	defer file.Close()

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	exts, _ := mime.ExtensionsByType(mimeType)
	ext := filepath.Ext(header.Filename)
	if ext == "" && len(exts) > 0 {
		ext = exts[0]
	}

	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), sanitizeExt(ext))
	destPath := filepath.Join(uploadsDir, filename)

	dst, err := os.Create(destPath)
	if err != nil {
		respond.InternalError(w)
		return
	}
	defer dst.Close()

	size, err := io.Copy(dst, file)
	if err != nil {
		os.Remove(destPath)
		respond.InternalError(w)
		return
	}

	url := strings.TrimRight(h.baseURL, "/") + "/uploads/" + filename

	var f MediaFile
	err = h.db.QueryRowContext(r.Context(),
		`INSERT INTO media_files (filename, original_name, mime_type, size_bytes, url)
		 VALUES ($1,$2,$3,$4,$5)
		 RETURNING id, filename, original_name, mime_type, size_bytes, url, created_at`,
		filename, header.Filename, mimeType, size, url).
		Scan(&f.ID, &f.Filename, &f.OriginalName, &f.MimeType, &f.SizeBytes, &f.URL, &f.CreatedAt)
	if err != nil {
		os.Remove(destPath)
		respond.InternalError(w)
		return
	}
	respond.JSON(w, http.StatusCreated, f)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var filename string
	err := h.db.QueryRowContext(r.Context(),
		`DELETE FROM media_files WHERE id=$1 RETURNING filename`, id).Scan(&filename)
	if err == sql.ErrNoRows {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	os.Remove(filepath.Join(uploadsDir, filename))
	w.WriteHeader(http.StatusNoContent)
}

func sanitizeExt(ext string) string {
	allowed := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".webp": true, ".svg": true, ".pdf": true,
	}
	ext = strings.ToLower(ext)
	if allowed[ext] {
		return ext
	}
	return ""
}
