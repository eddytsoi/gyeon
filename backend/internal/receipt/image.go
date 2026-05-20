package receipt

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// uploadsURLRe captures an optional scheme+host prefix and the `/uploads/<rest>`
// tail of an image URL, stripping any query/hash so the rewritten resize URL
// stays clean. Matches `frontend/src/lib/image.ts::buildResponsiveAttrs`.
var uploadsURLRe = regexp.MustCompile(`^(.*?)/uploads/([^?#]+)(\?.*)?$`)

// toResizedWebpURL rewrites a same-host /uploads/<filename> URL into the
// resize endpoint's WebP variant at the given width. Pass-through (returns
// the original URL unchanged) for anything that:
//   - is empty
//   - doesn't match the /uploads/<filename> pattern (external CDN, data URI)
//   - already points at /uploads/r/ (already resized)
//   - has a filename containing "/" or ".." (would be rejected by backend anyway)
//
// Width should be one of media.allowedWidths; callers in this package only
// pass 160 (product thumbs) and 320 (logo) so we don't re-validate here.
//
// Forces a `.webp` filename even when no literal WebP sibling exists on disk:
// the backend's resize endpoint (media/resize.go::webpBaseFallback) falls back
// to the base raster (foo.jpg / foo.png) so the rewrite is safe for legacy
// uploads that predated automatic sibling generation. Using the explicit
// `.webp` URL also means Cloudflare's free plan (which ignores Vary: Accept)
// can't accidentally cache a JPEG for WebP-capable clients.
func toResizedWebpURL(rawURL string, width int) string {
	if rawURL == "" {
		return rawURL
	}
	m := uploadsURLRe.FindStringSubmatch(rawURL)
	if m == nil {
		return rawURL
	}
	origin, filename := m[1], m[2]

	// Already pointing at the resize endpoint — don't double-process.
	if strings.HasPrefix(filename, "r/") {
		return rawURL
	}
	// Backend rejects slashes/dotdot in filename, so the rewrite is a no-op
	// for anything that wouldn't be served anyway.
	if strings.ContainsAny(filename, "/\\") || strings.Contains(filename, "..") {
		return rawURL
	}

	served := toWebpFilename(filename)
	return origin + "/uploads/r/" + strconv.Itoa(width) + "/" + served
}

// toWebpFilename swaps a JPEG/PNG extension for `.webp`. Other extensions
// (including `.webp` itself and non-raster formats like `.svg`) pass through
// unchanged. Mirrors frontend/src/lib/image.ts::toWebpFilename.
func toWebpFilename(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg", ".png":
		return filename[:len(filename)-len(ext)] + ".webp"
	}
	return filename
}
