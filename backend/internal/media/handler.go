package media

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"gyeon/backend/internal/respond"
	"gyeon/backend/internal/settings"
)

const uploadsDir = "./uploads"

type MediaRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

type MediaFile struct {
	ID                 string     `json:"id"`
	Filename           string     `json:"filename"`
	OriginalName       string     `json:"original_name"`
	MimeType           string     `json:"mime_type"`
	SizeBytes          int64      `json:"size_bytes"`
	URL                string     `json:"url"`
	CreatedAt          string     `json:"created_at"`
	Refs               []MediaRef `json:"refs"`
	WebpURL            *string    `json:"webp_url"`
	WebpSizeBytes      *int64     `json:"webp_size_bytes"`
	ThumbnailURL       *string    `json:"thumbnail_url,omitempty"`
	ThumbnailSizeBytes *int64     `json:"thumbnail_size_bytes,omitempty"`
}

type Handler struct {
	db       *sql.DB
	baseURL  string
	settings *settings.Service
	svc      *Service
}

func NewHandler(db *sql.DB, baseURL string, settingsSvc *settings.Service, svc *Service) *Handler {
	os.MkdirAll(uploadsDir, 0755)
	return &Handler{db: db, baseURL: baseURL, settings: settingsSvc, svc: svc}
}

// uploadLimits reads configurable size limits from site_settings with fallbacks.
func (h *Handler) uploadLimits(ctx context.Context) (imageMB, videoMB int64) {
	imageMB, videoMB = 1, 10
	if h.settings == nil {
		return
	}
	if st, err := h.settings.Get(ctx, "upload_max_image_mb"); err == nil {
		if n, err := strconv.ParseInt(st.Value, 10, 64); err == nil && n > 0 {
			imageMB = n
		}
	}
	if st, err := h.settings.Get(ctx, "upload_max_video_mb"); err == nil {
		if n, err := strconv.ParseInt(st.Value, 10, 64); err == nil && n > 0 {
			videoMB = n
		}
	}
	return
}

func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Post("/upload", h.upload)
	r.Post("/link", h.addLink)
	r.Get("/{id}", h.get)
	r.Patch("/{id}", h.update)
	r.Delete("/{id}", h.delete)
	return r
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	h.getByID(w, r, chi.URLParam(r, "id"))
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT mf.id, mf.filename, mf.original_name, mf.mime_type,
		       mf.size_bytes, mf.url, mf.created_at,
		       mf.webp_url, mf.webp_size_bytes,
		       mf.thumbnail_url, mf.thumbnail_size_bytes,
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
		         mf.size_bytes, mf.url, mf.created_at, mf.webp_url, mf.webp_size_bytes,
		         mf.thumbnail_url, mf.thumbnail_size_bytes
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
		var webpURL, thumbURL sql.NullString
		var webpSizeBytes, thumbSizeBytes sql.NullInt64
		if err := rows.Scan(&f.ID, &f.Filename, &f.OriginalName, &f.MimeType,
			&f.SizeBytes, &f.URL, &f.CreatedAt, &webpURL, &webpSizeBytes,
			&thumbURL, &thumbSizeBytes, &refsJSON); err != nil {
			respond.InternalError(w)
			return
		}
		if webpURL.Valid {
			f.WebpURL = &webpURL.String
		}
		if webpSizeBytes.Valid {
			f.WebpSizeBytes = &webpSizeBytes.Int64
		}
		if thumbURL.Valid {
			f.ThumbnailURL = &thumbURL.String
		}
		if thumbSizeBytes.Valid {
			f.ThumbnailSizeBytes = &thumbSizeBytes.Int64
		}
		if err := json.Unmarshal(refsJSON, &f.Refs); err != nil {
			f.Refs = []MediaRef{}
		}
		files = append(files, f)
	}
	// Lazy-backfill thumbnails for any video missing one (legacy uploads,
	// or rows whose thumbnail was cleared after a regeneration trigger).
	for _, f := range files {
		if strings.HasPrefix(f.MimeType, "video/") && (f.ThumbnailURL == nil || *f.ThumbnailURL == "") {
			id := f.ID
			go h.EnsureVideoThumbnail(context.Background(), id)
		}
	}
	respond.JSON(w, http.StatusOK, files)
}

func (h *Handler) getByID(w http.ResponseWriter, r *http.Request, id string) {
	var f MediaFile
	var refsJSON []byte
	var webpURL, thumbURL sql.NullString
	var webpSizeBytes, thumbSizeBytes sql.NullInt64
	err := h.db.QueryRowContext(r.Context(), `
		SELECT mf.id, mf.filename, mf.original_name, mf.mime_type,
		       mf.size_bytes, mf.url, mf.created_at,
		       mf.webp_url, mf.webp_size_bytes,
		       mf.thumbnail_url, mf.thumbnail_size_bytes,
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
		WHERE mf.id = $1
		GROUP BY mf.id, mf.filename, mf.original_name, mf.mime_type,
		         mf.size_bytes, mf.url, mf.created_at, mf.webp_url, mf.webp_size_bytes,
		         mf.thumbnail_url, mf.thumbnail_size_bytes`,
		id).Scan(&f.ID, &f.Filename, &f.OriginalName, &f.MimeType,
		&f.SizeBytes, &f.URL, &f.CreatedAt, &webpURL, &webpSizeBytes,
		&thumbURL, &thumbSizeBytes, &refsJSON)
	if err == sql.ErrNoRows {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	if webpURL.Valid {
		f.WebpURL = &webpURL.String
	}
	if webpSizeBytes.Valid {
		f.WebpSizeBytes = &webpSizeBytes.Int64
	}
	if thumbURL.Valid {
		f.ThumbnailURL = &thumbURL.String
	}
	if thumbSizeBytes.Valid {
		f.ThumbnailSizeBytes = &thumbSizeBytes.Int64
	}
	if err := json.Unmarshal(refsJSON, &f.Refs); err != nil {
		f.Refs = []MediaRef{}
	}
	respond.JSON(w, http.StatusOK, f)
}

type updateMediaRequest struct {
	OriginalName *string `json:"original_name"`
	URL          *string `json:"url"`
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req updateMediaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "invalid request body")
		return
	}

	var mimeType string
	if err := h.db.QueryRowContext(r.Context(), `SELECT mime_type FROM media_files WHERE id=$1`, id).Scan(&mimeType); err == sql.ErrNoRows {
		respond.NotFound(w)
		return
	} else if err != nil {
		respond.InternalError(w)
		return
	}

	setClauses := make([]string, 0, 3)
	args := make([]any, 0, 3)
	n := 1

	if req.OriginalName != nil {
		name := strings.TrimSpace(*req.OriginalName)
		if name == "" {
			respond.BadRequest(w, "original_name cannot be empty")
			return
		}
		setClauses = append(setClauses, fmt.Sprintf("original_name=$%d", n))
		args = append(args, name)
		n++
	}

	if req.URL != nil && mimeType == "link" {
		u := strings.TrimSpace(*req.URL)
		if u == "" {
			respond.BadRequest(w, "url cannot be empty")
			return
		}
		setClauses = append(setClauses, fmt.Sprintf("url=$%d", n), fmt.Sprintf("filename=$%d", n+1))
		args = append(args, u, u)
		n += 2
	}

	if len(setClauses) == 0 {
		respond.BadRequest(w, "nothing to update")
		return
	}

	args = append(args, id)
	if _, err := h.db.ExecContext(r.Context(),
		fmt.Sprintf("UPDATE media_files SET %s WHERE id=$%d", strings.Join(setClauses, ", "), n),
		args...); err != nil {
		respond.InternalError(w)
		return
	}

	h.getByID(w, r, id)
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request) {
	imageMB, videoMB := h.uploadLimits(r.Context())
	maxImageBytes := imageMB * 1024 * 1024
	maxVideoBytes := videoMB * 1024 * 1024

	r.Body = http.MaxBytesReader(w, r.Body, maxVideoBytes)
	if err := r.ParseMultipartForm(maxVideoBytes); err != nil {
		respond.BadRequest(w, fmt.Sprintf("file too large (max %d MB)", videoMB))
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

	if strings.HasPrefix(mimeType, "video/") && mimeType != "video/mp4" && mimeType != "video/webm" {
		respond.BadRequest(w, "only mp4 and webm video formats are accepted")
		return
	}

	var sizeLimit int64
	if strings.HasPrefix(mimeType, "video/") {
		sizeLimit = maxVideoBytes
	} else {
		sizeLimit = maxImageBytes
	}
	if header.Size > sizeLimit {
		if strings.HasPrefix(mimeType, "video/") {
			respond.BadRequest(w, fmt.Sprintf("video too large (max %d MB)", videoMB))
		} else {
			respond.BadRequest(w, fmt.Sprintf("image too large (max %d MB)", imageMB))
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

	var thumbFilenameDB, thumbURLDB sql.NullString
	var thumbSizeBytesDB sql.NullInt64
	if strings.HasPrefix(mimeType, "video/") {
		tfn, turl, tsize, terr := generateVideoThumbnail(destPath, filename, h.baseURL)
		if terr == nil {
			thumbFilenameDB = sql.NullString{String: tfn, Valid: true}
			thumbURLDB = sql.NullString{String: turl, Valid: true}
			thumbSizeBytesDB = sql.NullInt64{Int64: tsize, Valid: true}
		} else {
			log.Printf("media upload: thumbnail generation failed for %q: %v", filename, terr)
		}
		// Thumbnail failure is non-fatal: original upload succeeds regardless
	}

	var f MediaFile
	var webpURL, thumbURL sql.NullString
	var webpSizeBytes, thumbSizeBytes sql.NullInt64
	err = h.db.QueryRowContext(r.Context(),
		`INSERT INTO media_files
		     (filename, original_name, mime_type, size_bytes, url,
		      webp_filename, webp_url, webp_size_bytes,
		      thumbnail_filename, thumbnail_url, thumbnail_size_bytes)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		 RETURNING id, filename, original_name, mime_type, size_bytes, url, created_at,
		           webp_url, webp_size_bytes, thumbnail_url, thumbnail_size_bytes`,
		filename, header.Filename, mimeType, size, fileURL,
		webpFilenameDB, webpURLDB, webpSizeBytesDB,
		thumbFilenameDB, thumbURLDB, thumbSizeBytesDB).
		Scan(&f.ID, &f.Filename, &f.OriginalName, &f.MimeType, &f.SizeBytes, &f.URL, &f.CreatedAt,
			&webpURL, &webpSizeBytes, &thumbURL, &thumbSizeBytes)
	if err != nil {
		os.Remove(destPath)
		if webpFilenameDB.Valid {
			os.Remove(filepath.Join(uploadsDir, webpFilenameDB.String))
		}
		if thumbFilenameDB.Valid {
			os.Remove(filepath.Join(uploadsDir, thumbFilenameDB.String))
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
	if thumbURL.Valid {
		f.ThumbnailURL = &thumbURL.String
	}
	if thumbSizeBytes.Valid {
		f.ThumbnailSizeBytes = &thumbSizeBytes.Int64
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.BadRequest(w, "url is required")
		return
	}
	rawURL := strings.TrimSpace(req.URL)
	if rawURL == "" {
		respond.BadRequest(w, "url is required")
		return
	}
	name := strings.TrimSpace(req.Name)

	if provider, videoID, ok := DetectStreamingVideo(rawURL); ok {
		h.addStreamingVideoLink(w, r, rawURL, name, provider, videoID)
		return
	}

	if name == "" {
		name = rawURL
	}
	var f MediaFile
	err := h.db.QueryRowContext(r.Context(),
		`INSERT INTO media_files (filename, original_name, mime_type, size_bytes, url)
		 VALUES ($1,$2,$3,$4,$5)
		 RETURNING id, filename, original_name, mime_type, size_bytes, url, created_at`,
		rawURL, name, "link", 0, rawURL).
		Scan(&f.ID, &f.Filename, &f.OriginalName, &f.MimeType, &f.SizeBytes, &f.URL, &f.CreatedAt)
	if err != nil {
		respond.InternalError(w)
		return
	}
	f.Refs = []MediaRef{}
	respond.JSON(w, http.StatusCreated, f)
}

// addStreamingVideoLink stores a YouTube/Vimeo/Wistia URL as a video media row,
// best-effort fetching the platform's title and thumbnail via oEmbed. oEmbed
// failures are non-fatal — the row is still inserted, and the lazy-backfill
// goroutine in list() will retry the thumbnail on the next list call.
func (h *Handler) addStreamingVideoLink(w http.ResponseWriter, r *http.Request, rawURL, name string, provider StreamProvider, videoID string) {
	title, thumbSrcURL, err := FetchStreamingMetadata(r.Context(), provider, videoID, rawURL)
	if err != nil {
		log.Printf("addStreamingVideoLink: oembed %s failed for %q: %v", provider, rawURL, err)
	}

	var thumbFn, thumbURL sql.NullString
	var thumbSize sql.NullInt64
	if thumbSrcURL != "" && h.svc != nil {
		fn, fileURL, sz, _, derr := h.svc.DownloadToUploads(r.Context(), thumbSrcURL)
		if derr != nil {
			log.Printf("addStreamingVideoLink: thumbnail download %q failed: %v", thumbSrcURL, derr)
		} else {
			thumbFn = sql.NullString{String: fn, Valid: true}
			thumbURL = sql.NullString{String: fileURL, Valid: true}
			thumbSize = sql.NullInt64{Int64: sz, Valid: true}
		}
	}

	if name == "" {
		name = title
	}
	if name == "" {
		name = rawURL
	}

	var f MediaFile
	err = h.db.QueryRowContext(r.Context(),
		`INSERT INTO media_files
		     (filename, original_name, mime_type, size_bytes, url,
		      thumbnail_filename, thumbnail_url, thumbnail_size_bytes)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 RETURNING id, filename, original_name, mime_type, size_bytes, url, created_at`,
		rawURL, name, provider.MimeType(), 0, rawURL,
		thumbFn, thumbURL, thumbSize).
		Scan(&f.ID, &f.Filename, &f.OriginalName, &f.MimeType, &f.SizeBytes, &f.URL, &f.CreatedAt)
	if err != nil {
		if thumbFn.Valid {
			os.Remove(filepath.Join(uploadsDir, thumbFn.String))
		}
		respond.InternalError(w)
		return
	}
	if thumbURL.Valid {
		f.ThumbnailURL = &thumbURL.String
	}
	if thumbSize.Valid {
		f.ThumbnailSizeBytes = &thumbSize.Int64
	}
	f.Refs = []MediaRef{}
	respond.JSON(w, http.StatusCreated, f)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var filename, mimeType string
	var webpFilename, thumbFilename sql.NullString
	err := h.db.QueryRowContext(r.Context(),
		`DELETE FROM media_files WHERE id=$1 RETURNING filename, mime_type, webp_filename, thumbnail_filename`, id).
		Scan(&filename, &mimeType, &webpFilename, &thumbFilename)
	if err == sql.ErrNoRows {
		respond.NotFound(w)
		return
	}
	if err != nil {
		respond.InternalError(w)
		return
	}
	// Streaming-video rows store the external URL in `filename`; there's no
	// local file to remove and no CDN-cached uploads URL to purge.
	skipOriginal := IsStreamingMime(mimeType) || mimeType == "link"
	purgeURLs := []string{}
	if !skipOriginal {
		origPath := filepath.Join(uploadsDir, filename)
		if err := os.Remove(origPath); err != nil {
			log.Printf("media delete: remove original %q: %v", origPath, err)
		}
		purgeURLs = append(purgeURLs, h.baseURL+"/uploads/"+filename)
	}
	if webpFilename.Valid && webpFilename.String != "" {
		webpPath := filepath.Join(uploadsDir, webpFilename.String)
		if err := os.Remove(webpPath); err != nil {
			log.Printf("media delete: remove webp %q: %v", webpPath, err)
		}
		purgeURLs = append(purgeURLs, h.baseURL+"/uploads/"+webpFilename.String)
	}
	if thumbFilename.Valid && thumbFilename.String != "" {
		thumbPath := filepath.Join(uploadsDir, thumbFilename.String)
		if err := os.Remove(thumbPath); err != nil {
			log.Printf("media delete: remove thumbnail %q: %v", thumbPath, err)
		}
		purgeURLs = append(purgeURLs, h.baseURL+"/uploads/"+thumbFilename.String)
	}
	h.purgeCloudflare(r.Context(), purgeURLs)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) purgeCloudflare(ctx context.Context, urls []string) {
	zoneSt, err := h.settings.Get(ctx, "cloudflare_zone_id")
	if err != nil || zoneSt.Value == "" {
		return
	}
	tokenSt, err := h.settings.Get(ctx, "cloudflare_api_token")
	if err != nil || tokenSt.Value == "" {
		return
	}

	body, _ := json.Marshal(map[string][]string{"files": urls})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.cloudflare.com/client/v4/zones/"+zoneSt.Value+"/purge_cache",
		bytes.NewReader(body))
	if err != nil {
		log.Printf("cloudflare purge: build request: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+tokenSt.Value)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("cloudflare purge: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("cloudflare purge: unexpected status %d for %v", resp.StatusCode, urls)
	}
}

// isConvertibleToWebP returns true for JPEG and PNG, which cwebp supports as input.
// GIF, SVG, PDF, and videos are excluded.
func isConvertibleToWebP(mimeType string) bool {
	return mimeType == "image/jpeg" || mimeType == "image/png"
}

// generateVideoThumbnail extracts a representative first frame from srcPath
// using the system ffmpeg binary, written as a JPEG sibling file. Returns the
// thumbnail filename, public URL, and byte size.
// Requires ffmpeg to be installed (apt-get install ffmpeg / brew install ffmpeg).
func generateVideoThumbnail(srcPath, srcFilename, baseURL string) (thumbFilename, thumbURL string, thumbSize int64, err error) {
	thumbFilename = strings.TrimSuffix(srcFilename, filepath.Ext(srcFilename)) + "_thumb.jpg"
	thumbPath := filepath.Join(uploadsDir, thumbFilename)

	// -ss 0 before -i seeks to the very first frame (00:00:00) — without it,
	// the default "thumbnail" filter would skip ahead to a representative frame.
	if err = exec.Command("ffmpeg", "-y", "-ss", "0", "-i", srcPath,
		"-frames:v", "1", "-vf", "scale=640:-1", "-q:v", "4",
		thumbPath).Run(); err != nil {
		return
	}

	info, statErr := os.Stat(thumbPath)
	if statErr != nil {
		os.Remove(thumbPath)
		err = statErr
		return
	}

	thumbSize = info.Size()
	thumbURL = strings.TrimRight(baseURL, "/") + "/uploads/" + thumbFilename
	return
}

// EnsureVideoThumbnail generates and persists a thumbnail for a video media
// row that doesn't have one yet. Idempotent and best-effort: no-op for
// non-videos, rows already having thumbnail_url, or rows whose backing file
// is missing. Failures are logged but never returned, so this can be called
// from background goroutines without affecting the originating request.
func (h *Handler) EnsureVideoThumbnail(ctx context.Context, mediaFileID string) {
	var filename, mimeType, urlStr string
	var thumbURL sql.NullString
	err := h.db.QueryRowContext(ctx,
		`SELECT filename, mime_type, url, thumbnail_url FROM media_files WHERE id = $1`,
		mediaFileID).Scan(&filename, &mimeType, &urlStr, &thumbURL)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("ensure thumbnail: lookup %q: %v", mediaFileID, err)
		}
		return
	}
	if !strings.HasPrefix(mimeType, "video/") {
		return
	}
	if thumbURL.Valid && thumbURL.String != "" {
		return
	}

	// Streaming videos: re-fetch via oEmbed instead of running ffmpeg against
	// a non-existent local file. Best-effort; failure is logged and retried
	// on the next list() call.
	if IsStreamingMime(mimeType) {
		h.ensureStreamingThumbnail(ctx, mediaFileID, mimeType, urlStr)
		return
	}

	srcPath := filepath.Join(uploadsDir, filename)
	if _, statErr := os.Stat(srcPath); statErr != nil {
		log.Printf("ensure thumbnail: source missing %q: %v", srcPath, statErr)
		return
	}

	tfn, turl, tsize, terr := generateVideoThumbnail(srcPath, filename, h.baseURL)
	if terr != nil {
		log.Printf("ensure thumbnail: ffmpeg failed for %q: %v", filename, terr)
		return
	}

	if _, err := h.db.ExecContext(ctx,
		`UPDATE media_files
		    SET thumbnail_filename = $2, thumbnail_url = $3, thumbnail_size_bytes = $4
		  WHERE id = $1 AND (thumbnail_url IS NULL OR thumbnail_url = '')`,
		mediaFileID, tfn, turl, tsize); err != nil {
		log.Printf("ensure thumbnail: persist %q: %v", mediaFileID, err)
		os.Remove(filepath.Join(uploadsDir, tfn))
	}
}

// ensureStreamingThumbnail re-runs the oEmbed fetch + thumbnail download for a
// streaming-video row that lost its thumbnail (initial fetch failed, or thumb
// columns were manually cleared). No-op if the platform call still fails.
func (h *Handler) ensureStreamingThumbnail(ctx context.Context, mediaFileID, mimeType, originalURL string) {
	if h.svc == nil {
		return
	}
	provider := StreamProvider(strings.TrimPrefix(mimeType, "video/"))
	_, videoID, ok := DetectStreamingVideo(originalURL)
	if !ok {
		log.Printf("ensure thumbnail: streaming row %q url no longer matches provider", mediaFileID)
		return
	}
	_, thumbSrcURL, err := FetchStreamingMetadata(ctx, provider, videoID, originalURL)
	if err != nil || thumbSrcURL == "" {
		if err != nil {
			log.Printf("ensure thumbnail: oembed %s failed for %q: %v", provider, originalURL, err)
		}
		return
	}
	fn, fileURL, sz, _, derr := h.svc.DownloadToUploads(ctx, thumbSrcURL)
	if derr != nil {
		log.Printf("ensure thumbnail: download %q failed: %v", thumbSrcURL, derr)
		return
	}
	if _, err := h.db.ExecContext(ctx,
		`UPDATE media_files
		    SET thumbnail_filename = $2, thumbnail_url = $3, thumbnail_size_bytes = $4
		  WHERE id = $1 AND (thumbnail_url IS NULL OR thumbnail_url = '')`,
		mediaFileID, fn, fileURL, sz); err != nil {
		log.Printf("ensure thumbnail: persist streaming %q: %v", mediaFileID, err)
		os.Remove(filepath.Join(uploadsDir, fn))
	}
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
		".mp4": true, ".webm": true,
	}
	ext = strings.ToLower(ext)
	if allowed[ext] {
		return ext
	}
	return ""
}
