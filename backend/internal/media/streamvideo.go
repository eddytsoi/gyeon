package media

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type StreamProvider string

const (
	ProviderYouTube StreamProvider = "youtube"
	ProviderVimeo   StreamProvider = "vimeo"
	ProviderWistia  StreamProvider = "wistia"
)

func (p StreamProvider) MimeType() string {
	return "video/" + string(p)
}

// IsStreamingMime reports whether mime is one of the streaming-video mime types
// produced by this package (video/youtube | video/vimeo | video/wistia).
func IsStreamingMime(mime string) bool {
	switch mime {
	case "video/youtube", "video/vimeo", "video/wistia":
		return true
	}
	return false
}

var (
	ytIDRegex     = regexp.MustCompile(`^[A-Za-z0-9_-]{6,20}$`)
	vimeoIDRegex  = regexp.MustCompile(`^[0-9]+$`)
	vimeoHashReg  = regexp.MustCompile(`^[A-Za-z0-9]+$`)
	wistiaIDRegex = regexp.MustCompile(`^[A-Za-z0-9]{6,20}$`)
)

// DetectStreamingVideo parses rawURL and returns the matched provider plus a
// videoID that uniquely identifies the video on that platform. For Vimeo
// private hashes the videoID has the form "{id}/{hash}" so callers can
// reconstruct the embed URL.
//
// Pure function — no network calls. Returns ok=false for non-matching URLs,
// playlists without a video id, channels, and showcases.
func DetectStreamingVideo(rawURL string) (provider StreamProvider, videoID string, ok bool) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" {
		return "", "", false
	}
	host := strings.ToLower(u.Host)
	path := strings.TrimSuffix(u.Path, "/")

	// ── YouTube ────────────────────────────────────────────────────────────
	if isYouTubeHost(host) {
		// youtu.be/{id}
		if host == "youtu.be" {
			id := strings.TrimPrefix(path, "/")
			if ytIDRegex.MatchString(id) {
				return ProviderYouTube, id, true
			}
			return "", "", false
		}
		// youtube.com / m.youtube.com / www.youtube.com
		// /watch?v={id} (preferred even when ?list=... is present)
		if path == "/watch" {
			id := u.Query().Get("v")
			if ytIDRegex.MatchString(id) {
				return ProviderYouTube, id, true
			}
			return "", "", false
		}
		for _, prefix := range []string{"/embed/", "/shorts/", "/v/"} {
			if strings.HasPrefix(path, prefix) {
				id := strings.SplitN(path[len(prefix):], "/", 2)[0]
				if ytIDRegex.MatchString(id) {
					return ProviderYouTube, id, true
				}
				return "", "", false
			}
		}
		return "", "", false
	}

	// ── Vimeo ──────────────────────────────────────────────────────────────
	if isVimeoHost(host) {
		segs := strings.Split(strings.TrimPrefix(path, "/"), "/")
		// player.vimeo.com/video/{id}
		if host == "player.vimeo.com" && len(segs) >= 2 && segs[0] == "video" && vimeoIDRegex.MatchString(segs[1]) {
			id := segs[1]
			if len(segs) >= 3 && vimeoHashReg.MatchString(segs[2]) {
				return ProviderVimeo, id + "/" + segs[2], true
			}
			// hash sometimes provided as ?h=
			if h := u.Query().Get("h"); h != "" && vimeoHashReg.MatchString(h) {
				return ProviderVimeo, id + "/" + h, true
			}
			return ProviderVimeo, id, true
		}
		// vimeo.com/{id} or vimeo.com/{id}/{hash}
		if len(segs) == 1 && vimeoIDRegex.MatchString(segs[0]) {
			return ProviderVimeo, segs[0], true
		}
		if len(segs) == 2 && vimeoIDRegex.MatchString(segs[0]) && vimeoHashReg.MatchString(segs[1]) {
			return ProviderVimeo, segs[0] + "/" + segs[1], true
		}
		// vimeo.com/channels/{name}/{id}
		if len(segs) == 3 && segs[0] == "channels" && vimeoIDRegex.MatchString(segs[2]) {
			return ProviderVimeo, segs[2], true
		}
		// vimeo.com/groups/{name}/videos/{id}
		if len(segs) == 4 && segs[0] == "groups" && segs[2] == "videos" && vimeoIDRegex.MatchString(segs[3]) {
			return ProviderVimeo, segs[3], true
		}
		// vimeo.com/channels/{name} (no id) → reject
		return "", "", false
	}

	// ── Wistia ─────────────────────────────────────────────────────────────
	if isWistiaHost(host) {
		// /channel/, /showcase/ → reject
		if strings.HasPrefix(path, "/channel/") || strings.HasPrefix(path, "/showcase/") {
			return "", "", false
		}
		segs := strings.Split(strings.TrimPrefix(path, "/"), "/")
		// {vendor}.wistia.com/medias/{id}
		if len(segs) >= 2 && segs[0] == "medias" && wistiaIDRegex.MatchString(segs[1]) {
			return ProviderWistia, segs[1], true
		}
		// fast.wistia.com/embed/iframe/{id}
		if len(segs) >= 3 && segs[0] == "embed" && segs[1] == "iframe" && wistiaIDRegex.MatchString(segs[2]) {
			return ProviderWistia, segs[2], true
		}
		return "", "", false
	}

	return "", "", false
}

func isYouTubeHost(host string) bool {
	switch host {
	case "youtube.com", "www.youtube.com", "m.youtube.com", "youtu.be":
		return true
	}
	return false
}

func isVimeoHost(host string) bool {
	switch host {
	case "vimeo.com", "www.vimeo.com", "player.vimeo.com":
		return true
	}
	return false
}

func isWistiaHost(host string) bool {
	if strings.HasSuffix(host, ".wistia.com") || strings.HasSuffix(host, ".wistia.net") {
		return true
	}
	switch host {
	case "wistia.com", "wistia.net":
		return true
	}
	return false
}

// streamHTTPClient is the default client used by FetchStreamingMetadata.
// Tests can override via FetchStreamingMetadataWithClient.
var streamHTTPClient = &http.Client{Timeout: 10 * time.Second}

// FetchStreamingMetadata calls the platform's oEmbed endpoint and returns the
// canonical title and thumbnail URL. Best-effort: callers should treat any
// non-nil error as non-fatal and proceed with empty strings.
func FetchStreamingMetadata(ctx context.Context, p StreamProvider, videoID, originalURL string) (title, thumbnailURL string, err error) {
	return fetchStreamingMetadataWithClient(ctx, streamHTTPClient, p, videoID, originalURL)
}

func fetchStreamingMetadataWithClient(ctx context.Context, client *http.Client, p StreamProvider, videoID, originalURL string) (title, thumbnailURL string, err error) {
	endpoint, err := oembedEndpoint(p, originalURL)
	if err != nil {
		return "", "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return "", "", fmt.Errorf("oembed %s: status %d", p, resp.StatusCode)
	}
	var payload struct {
		Title        string `json:"title"`
		ThumbnailURL string `json:"thumbnail_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", "", err
	}
	return payload.Title, payload.ThumbnailURL, nil
}

func oembedEndpoint(p StreamProvider, originalURL string) (string, error) {
	q := url.Values{}
	q.Set("url", originalURL)
	q.Set("format", "json")
	switch p {
	case ProviderYouTube:
		return "https://www.youtube.com/oembed?" + q.Encode(), nil
	case ProviderVimeo:
		return "https://vimeo.com/api/oembed.json?" + q.Encode(), nil
	case ProviderWistia:
		return "https://fast.wistia.com/oembed?" + q.Encode(), nil
	}
	return "", fmt.Errorf("unknown provider %q", p)
}
