package media

import (
	"database/sql"
	"encoding/json"
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

const maxImageSize = 1 << 20  // 1 MB
const maxVideoSize = 10 << 20 // 10 MB
const uploadsDir = "./uploads"

type MediaRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

type MediaFile struct {
	ID           string     `json:"id"`
	Filename     string     `json:"filename"`
	OriginalName string     `json:"original_name"`
	MimeType     string     `json:"mime_type"`
	SizeBytes    int64      `json:"size_bytes"`
	URL          string     `json:"url"`
	CreatedAt    string     `json:"created_at"`
	Refs         []MediaRef `json:"refs"`
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
	r.Post("/link", h.addLink)
	r.Delete("/{id}", h.delete)
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT mf.id, mf.filename, mf.original_name, mf.mime_type,
		       mf.size_bytes, mf.url, mf.created_at,
		       COALESCE(json_agg(DISTINCT jsonb_build_object(
		           'type', refs.entity_type,
		           'id',   refs.entity_id,
		           'name', refs.entity_name
		       )) FILTER (WHERE refs.entity_type IS NOT NULL), '[]') AS refs
		FROM media_files mf
		LEFT JOIN (
		    -- product images: FK takes priority, URL as fallback for old data
		    SELECT pi.media_file_id AS mf_id, 'product' AS entity_type,
		           p.id::text AS entity_id, p.name AS entity_name
		    FROM product_images pi JOIN products p ON p.id = pi.product_id
		    WHERE pi.media_file_id IS NOT NULL
		    UNION ALL
		    SELECT mf2.id, 'product', p.id::text, p.name
		    FROM product_images pi JOIN products p ON p.id = pi.product_id
		    JOIN media_files mf2 ON mf2.url = pi.url
		    WHERE pi.media_file_id IS NULL AND pi.url IS NOT NULL
		    UNION ALL
		    -- post cover: FK takes priority, URL as fallback
		    SELECT cp.cover_media_file_id, 'post',
		           cp.id::text, cp.title
		    FROM cms_posts cp WHERE cp.cover_media_file_id IS NOT NULL
		    UNION ALL
		    SELECT mf2.id, 'post', cp.id::text, cp.title
		    FROM cms_posts cp JOIN media_files mf2 ON mf2.url = cp.cover_image_url
		    WHERE cp.cover_media_file_id IS NULL AND cp.cover_image_url IS NOT NULL
		) refs ON refs.mf_id = mf.id
		GROUP BY mf.id, mf.filename, mf.original_name, mf.mime_type,
		         mf.size_bytes, mf.url, mf.created_at
		ORDER BY mf.created_at DESC
		LIMIT 200`)
	if err != nil {
		respond.InternalError(w)
		return
	}
	defer rows.Close()

	files := make([]MediaFile, 0)
	for rows.Next() {
		var f MediaFile
		var refsJSON []byte
		if err := rows.Scan(&f.ID, &f.Filename, &f.OriginalName, &f.MimeType,
			&f.SizeBytes, &f.URL, &f.CreatedAt, &refsJSON); err != nil {
			respond.InternalError(w)
			return
		}
		if err := json.Unmarshal(refsJSON, &f.Refs); err != nil {
			f.Refs = []MediaRef{}
		}
		files = append(files, f)
	}
	respond.JSON(w, http.StatusOK, files)
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxVideoSize)
	if err := r.ParseMultipartForm(maxVideoSize); err != nil {
		respond.BadRequest(w, "file too large (max 10 MB)")
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

	// Per-type size limit
	var sizeLimit int64
	if strings.HasPrefix(mimeType, "video/") {
		sizeLimit = maxVideoSize
	} else {
		sizeLimit = maxImageSize
	}
	if header.Size > sizeLimit {
		if strings.HasPrefix(mimeType, "video/") {
			respond.BadRequest(w, "video too large (max 10 MB)")
		} else {
			respond.BadRequest(w, "image too large (max 1 MB)")
		}
		return
	}

	exts, _ := mime.ExtensionsByType(mimeType)
	ext := filepath.Ext(header.Filename)
	if ext == "" && len(exts) > 0 {
		ext = exts[0]
	}

	sanitized := sanitizeExt(ext)
	if sanitized == "" {
		respond.BadRequest(w, "unsupported file type")
		return
	}

	baseName := sanitizeName(strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename)))
	filename := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), baseName, sanitized)
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
	f.Refs = []MediaRef{}
	respond.JSON(w, http.StatusCreated, f)
}

type addLinkRequest struct {
	URL  string `json:"url"`
	Name string `json:"name"`
}

func (h *Handler) addLink(w http.ResponseWriter, r *http.Request) {
	var req addLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.URL) == "" {
		respond.BadRequest(w, "url is required")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = req.URL
	}
	var f MediaFile
	err := h.db.QueryRowContext(r.Context(),
		`INSERT INTO media_files (filename, original_name, mime_type, size_bytes, url)
		 VALUES ($1,$2,$3,$4,$5)
		 RETURNING id, filename, original_name, mime_type, size_bytes, url, created_at`,
		req.URL, name, "link", 0, req.URL).
		Scan(&f.ID, &f.Filename, &f.OriginalName, &f.MimeType, &f.SizeBytes, &f.URL, &f.CreatedAt)
	if err != nil {
		respond.InternalError(w)
		return
	}
	f.Refs = []MediaRef{}
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

// sanitizeName makes a filename safe for disk: lowercased, spaces → hyphens,
// only alphanumeric/hyphen/underscore/dot allowed, max 80 chars.
func sanitizeName(name string) string {
	name = strings.ToLower(name)
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('-')
		}
	}
	result := b.String()
	if len(result) > 80 {
		result = result[:80]
	}
	if result == "" {
		result = "file"
	}
	return result
}

func sanitizeExt(ext string) string {
	allowed := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".webp": true, ".svg": true, ".pdf": true,
		".mp4": true, ".webm": true, ".mov": true,
	}
	ext = strings.ToLower(ext)
	if allowed[ext] {
		return ext
	}
	return ""
}
