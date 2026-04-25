package media

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
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
	ID            string     `json:"id"`
	Filename      string     `json:"filename"`
	OriginalName  string     `json:"original_name"`
	MimeType      string     `json:"mime_type"`
	SizeBytes     int64      `json:"size_bytes"`
	URL           string     `json:"url"`
	CreatedAt     string     `json:"created_at"`
	Refs          []MediaRef `json:"refs"`
	WebpURL       *string    `json:"webp_url"`
	WebpSizeBytes *int64     `json:"webp_size_bytes"`
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
		       mf.webp_url, mf.webp_size_bytes,
		       COALESCE(json_agg(DISTINCT jsonb_build_object(
		           'type', refs.entity_type,
		           'id',   refs.entity_id,
		           'name', refs.entity_name
		       )) FILTER (WHERE refs.entity_type IS NOT NULL), '[]') AS refs
		FROM media_files mf
		LEFT JOIN (
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
		    SELECT cp.cover_media_file_id, 'post',
		           cp.id::text, cp.title
		    FROM cms_posts cp WHERE cp.cover_media_file_id IS NOT NULL
		    UNION ALL
		    SELECT mf2.id, 'post', cp.id::text, cp.title
		    FROM cms_posts cp JOIN media_files mf2 ON mf2.url = cp.cover_image_url
		    WHERE cp.cover_media_file_id IS NULL AND cp.cover_image_url IS NOT NULL
		) refs ON refs.mf_id = mf.id
		GROUP BY mf.id, mf.filename, mf.original_name, mf.mime_type,
		         mf.size_bytes, mf.url, mf.created_at, mf.webp_url, mf.webp_size_bytes
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
		var webpURL sql.NullString
		var webpSizeBytes sql.NullInt64
		if err := rows.Scan(&f.ID, &f.Filename, &f.OriginalName, &f.MimeType,
			&f.SizeBytes, &f.URL, &f.CreatedAt, &webpURL, &webpSizeBytes, &refsJSON); err != nil {
			respond.InternalError(w)
			return
		}
		if webpURL.Valid {
			f.WebpURL = &webpURL.String
		}
		if webpSizeBytes.Valid {
			f.WebpSizeBytes = &webpSizeBytes.Int64
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
	size, copyErr := io.Copy(dst, file)
	dst.Close()
	if copyErr != nil {
		os.Remove(destPath)
		respond.InternalError(w)
		return
	}

	fileURL := strings.TrimRight(h.baseURL, "/") + "/uploads/" + filename

	var webpFilenameDB, webpURLDB sql.NullString
	var webpSizeBytesDB sql.NullInt64
	if isConvertibleToWebP(mimeType) {
		wfn, wurl, wsize, werr := generateWebP(destPath, filename, h.baseURL)
		if werr == nil {
			webpFilenameDB = sql.NullString{String: wfn, Valid: true}
			webpURLDB = sql.NullString{String: wurl, Valid: true}
			webpSizeBytesDB = sql.NullInt64{Int64: wsize, Valid: true}
		}
		// WebP failure is non-fatal: original upload succeeds regardless
	}

	var f MediaFile
	var webpURL sql.NullString
	var webpSizeBytes sql.NullInt64
	err = h.db.QueryRowContext(r.Context(),
		`INSERT INTO media_files
		     (filename, original_name, mime_type, size_bytes, url,
		      webp_filename, webp_url, webp_size_bytes)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 RETURNING id, filename, original_name, mime_type, size_bytes, url, created_at,
		           webp_url, webp_size_bytes`,
		filename, header.Filename, mimeType, size, fileURL,
		webpFilenameDB, webpURLDB, webpSizeBytesDB).
		Scan(&f.ID, &f.Filename, &f.OriginalName, &f.MimeType, &f.SizeBytes, &f.URL, &f.CreatedAt,
			&webpURL, &webpSizeBytes)
	if err != nil {
		os.Remove(destPath)
		if webpFilenameDB.Valid {
			os.Remove(filepath.Join(uploadsDir, webpFilenameDB.String))
		}
		respond.InternalError(w)
		return
	}
	if webpURL.Valid {
		f.WebpURL = &webpURL.String
	}
	if webpSizeBytes.Valid {
		f.WebpSizeBytes = &webpSizeBytes.Int64
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
	var webpFilename sql.NullString
	err := h.db.QueryRowContext(r.Context(),
		`DELETE FROM media_files WHERE id=$1 RETURNING filename, webp_filename`, id).
		Scan(&filename, &webpFilename)
	if err == sql.ErrNoRows {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	os.Remove(filepath.Join(uploadsDir, filename))
	if webpFilename.Valid && webpFilename.String != "" {
		os.Remove(filepath.Join(uploadsDir, webpFilename.String))
	}
	w.WriteHeader(http.StatusNoContent)
}

// isConvertibleToWebP returns true for JPEG and PNG, which cwebp supports as input.
// GIF, SVG, PDF, and videos are excluded.
func isConvertibleToWebP(mimeType string) bool {
	return mimeType == "image/jpeg" || mimeType == "image/png"
}

// generateWebP calls the system cwebp binary to produce a WebP copy of srcPath.
// Returns the WebP filename, public URL, and byte size.
// Requires cwebp to be installed (apt-get install webp / brew install webp).
func generateWebP(srcPath, srcFilename, baseURL string) (webpFilename, webpURL string, webpSize int64, err error) {
	webpFilename = strings.TrimSuffix(srcFilename, filepath.Ext(srcFilename)) + ".webp"
	webpPath := filepath.Join(uploadsDir, webpFilename)

	if err = exec.Command("cwebp", "-q", "82", srcPath, "-o", webpPath).Run(); err != nil {
		return
	}

	info, statErr := os.Stat(webpPath)
	if statErr != nil {
		os.Remove(webpPath)
		err = statErr
		return
	}

	webpSize = info.Size()
	webpURL = strings.TrimRight(baseURL, "/") + "/uploads/" + webpFilename
	return
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
