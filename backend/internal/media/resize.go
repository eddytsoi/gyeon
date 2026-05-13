package media

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/go-chi/chi/v5"
	_ "golang.org/x/image/webp" // registers webp decoder for imaging.Open
	"gyeon/backend/internal/respond"
)

const resizeCacheDir = "./uploads/.cache"

// allowedWidths gates the resize endpoint to a fixed bucket set so the on-disk
// cache can't be flooded by arbitrary width probing, and so Cloudflare purge
// can enumerate every derived URL on media delete.
var allowedWidths = []int{160, 320, 480, 640, 768, 960, 1280, 1600, 1920}

func widthAllowed(w int) bool {
	for _, v := range allowedWidths {
		if v == w {
			return true
		}
	}
	return false
}

// resizableExt is the set of raster formats this endpoint handles. SVG/PDF/
// GIF/video fall through to the legacy /uploads/ FileServer.
func resizableExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg", ".png", ".webp":
		return true
	}
	return false
}

func resizeCachePath(filename string, width int, outExt string) string {
	return filepath.Join(resizeCacheDir, filename+".w"+strconv.Itoa(width)+outExt)
}

// ServeResized handles GET /uploads/r/{width}/{filename}. Width must be in
// allowedWidths; otherwise 400. WebP is served when the request's Accept
// header advertises image/webp, else the source format is preserved (or JPEG
// falls back for WebP sources on legacy clients). Successful responses cache
// under uploads/.cache/ keyed by filename + width + output ext.
func (h *Handler) ServeResized(w http.ResponseWriter, r *http.Request) {
	width, err := strconv.Atoi(chi.URLParam(r, "width"))
	if err != nil || !widthAllowed(width) {
		respond.BadRequest(w, "width not allowed")
		return
	}

	filename := chi.URLParam(r, "filename")
	if filename == "" || strings.ContainsAny(filename, "/\\") || strings.Contains(filename, "..") {
		respond.BadRequest(w, "invalid filename")
		return
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if !resizableExt(ext) {
		http.NotFound(w, r)
		return
	}

	srcPath := filepath.Join(uploadsDir, filename)
	if _, err := os.Stat(srcPath); err != nil {
		http.NotFound(w, r)
		return
	}

	wantWebP := strings.Contains(r.Header.Get("Accept"), "image/webp")
	outExt := chooseResizeExt(ext, wantWebP)
	cp := resizeCachePath(filename, width, outExt)

	if _, err := os.Stat(cp); err != nil {
		_ = os.MkdirAll(resizeCacheDir, 0o755)
		if outExt == ".webp" {
			if err := encodeResizedWebP(srcPath, cp, width); err != nil {
				http.Error(w, "resize: webp encode failed", http.StatusInternalServerError)
				return
			}
		} else {
			if err := encodeResizedRaster(srcPath, cp, width, outExt); err != nil {
				http.Error(w, "resize: encode failed", http.StatusInternalServerError)
				return
			}
		}
	}

	setResizedHeaders(w, outExt)
	http.ServeFile(w, r, cp)
}

// chooseResizeExt: WebP when the browser accepts it; otherwise keep PNG for
// PNG sources (transparency) and fall back to JPEG for everything else.
func chooseResizeExt(srcExt string, wantWebP bool) string {
	if wantWebP {
		return ".webp"
	}
	if srcExt == ".png" {
		return ".png"
	}
	return ".jpg"
}

func setResizedHeaders(w http.ResponseWriter, outExt string) {
	switch outExt {
	case ".webp":
		w.Header().Set("Content-Type", "image/webp")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	default:
		w.Header().Set("Content-Type", "image/jpeg")
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Vary", "Accept")
}

// encodeResizedWebP shells out to cwebp (already required by the upload flow
// at handler.go:829) — it can resize and encode in one process via -resize.
// Height 0 preserves aspect ratio.
func encodeResizedWebP(srcPath, dstPath string, width int) error {
	tmp := dstPath + ".tmp"
	defer os.Remove(tmp)
	cmd := exec.Command("cwebp", "-q", "82",
		"-resize", strconv.Itoa(width), "0",
		srcPath, "-o", tmp)
	if err := cmd.Run(); err != nil {
		return err
	}
	return os.Rename(tmp, dstPath)
}

// encodeResizedRaster handles JPEG/PNG output via pure-Go imaging. WebP source
// decode is enabled by the x/image/webp blank import above.
func encodeResizedRaster(srcPath, dstPath string, width int, outExt string) error {
	img, err := imaging.Open(srcPath, imaging.AutoOrientation(true))
	if err != nil {
		return err
	}
	resized := imaging.Resize(img, width, 0, imaging.Lanczos)

	tmp := dstPath + ".tmp"
	defer os.Remove(tmp)
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	format := imaging.JPEG
	opts := []imaging.EncodeOption{imaging.JPEGQuality(85)}
	if outExt == ".png" {
		format = imaging.PNG
		opts = nil
	}
	if err := imaging.Encode(f, resized, format, opts...); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dstPath)
}
