package media

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Service struct {
	db      *sql.DB
	baseURL string
}

func NewService(db *sql.DB, baseURL string) *Service {
	return &Service{db: db, baseURL: baseURL}
}

// FindIDBySourceURL returns the media_files.id for an asset previously
// downloaded from srcURL, or ("", false) if none exists. Used by the
// WooCommerce importer to skip re-downloading images on re-sync.
func (s *Service) FindIDBySourceURL(ctx context.Context, srcURL string) (string, bool) {
	if srcURL == "" {
		return "", false
	}
	var id string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM media_files WHERE source_url = $1 LIMIT 1`, srcURL).Scan(&id)
	if err != nil {
		return "", false
	}
	return id, true
}

// DownloadAndStore fetches srcURL, saves it to the uploads directory, converts
// to WebP when applicable, and inserts a media_files record. Returns the new ID.
// The srcURL is also recorded as media_files.source_url so future imports can
// dedup via FindIDBySourceURL instead of re-downloading.
func (s *Service) DownloadAndStore(ctx context.Context, srcURL, altText string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srcURL, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: status %d", srcURL, resp.StatusCode)
	}

	mimeType := resp.Header.Get("Content-Type")
	if idx := strings.Index(mimeType, ";"); idx != -1 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// Derive extension from URL path first, then fall back to mime type.
	urlPath := strings.Split(srcURL, "?")[0]
	ext := filepath.Ext(urlPath)
	if sanitizeExt(ext) == "" {
		if exts, _ := mime.ExtensionsByType(mimeType); len(exts) > 0 {
			ext = exts[0]
		}
	}
	sanitized := sanitizeExt(ext)
	if sanitized == "" {
		return "", fmt.Errorf("unsupported file type %q from %s", mimeType, srcURL)
	}

	// Use alt text as the human-readable name; fall back to the URL basename.
	originalName := strings.TrimSpace(altText)
	if originalName == "" {
		originalName = filepath.Base(urlPath)
	}
	baseName := sanitizeName(strings.TrimSuffix(filepath.Base(urlPath), filepath.Ext(urlPath)))
	filename := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), baseName, sanitized)
	destPath := filepath.Join(uploadsDir, filename)

	dst, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	size, copyErr := io.Copy(dst, resp.Body)
	dst.Close()
	if copyErr != nil {
		os.Remove(destPath)
		return "", copyErr
	}

	fileURL := strings.TrimRight(s.baseURL, "/") + "/uploads/" + filename

	var webpFilenameDB, webpURLDB sql.NullString
	var webpSizeBytesDB sql.NullInt64
	if isConvertibleToWebP(mimeType) {
		wfn, wurl, wsize, werr := generateWebP(destPath, filename, s.baseURL)
		if werr == nil {
			webpFilenameDB = sql.NullString{String: wfn, Valid: true}
			webpURLDB = sql.NullString{String: wurl, Valid: true}
			webpSizeBytesDB = sql.NullInt64{Int64: wsize, Valid: true}
		}
	}

	var id string
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO media_files
		     (filename, original_name, mime_type, size_bytes, url,
		      webp_filename, webp_url, webp_size_bytes, source_url)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 RETURNING id`,
		filename, originalName, mimeType, size, fileURL,
		webpFilenameDB, webpURLDB, webpSizeBytesDB, srcURL).Scan(&id)
	if err != nil {
		os.Remove(destPath)
		if webpFilenameDB.Valid {
			os.Remove(filepath.Join(uploadsDir, webpFilenameDB.String))
		}
		return "", err
	}
	return id, nil
}
