package media

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDetectStreamingVideo(t *testing.T) {
	cases := []struct {
		name     string
		url      string
		provider StreamProvider
		videoID  string
		ok       bool
	}{
		// YouTube positive
		{"yt watch", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", ProviderYouTube, "dQw4w9WgXcQ", true},
		{"yt short host", "https://youtu.be/dQw4w9WgXcQ", ProviderYouTube, "dQw4w9WgXcQ", true},
		{"yt embed", "https://www.youtube.com/embed/dQw4w9WgXcQ", ProviderYouTube, "dQw4w9WgXcQ", true},
		{"yt shorts", "https://www.youtube.com/shorts/dQw4w9WgXcQ", ProviderYouTube, "dQw4w9WgXcQ", true},
		{"yt v-path", "https://www.youtube.com/v/dQw4w9WgXcQ", ProviderYouTube, "dQw4w9WgXcQ", true},
		{"yt mobile", "https://m.youtube.com/watch?v=dQw4w9WgXcQ", ProviderYouTube, "dQw4w9WgXcQ", true},
		{"yt no www", "https://youtube.com/watch?v=dQw4w9WgXcQ", ProviderYouTube, "dQw4w9WgXcQ", true},
		{"yt watch with list", "https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=PLxyz", ProviderYouTube, "dQw4w9WgXcQ", true},
		{"yt watch with t", "https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=42s", ProviderYouTube, "dQw4w9WgXcQ", true},
		{"yt mixed case host", "https://WWW.YOUTUBE.COM/watch?v=dQw4w9WgXcQ", ProviderYouTube, "dQw4w9WgXcQ", true},
		{"yt trailing slash", "https://www.youtube.com/embed/dQw4w9WgXcQ/", ProviderYouTube, "dQw4w9WgXcQ", true},
		{"yt whitespace", "  https://youtu.be/dQw4w9WgXcQ  ", ProviderYouTube, "dQw4w9WgXcQ", true},

		// YouTube negative
		{"yt playlist only", "https://www.youtube.com/playlist?list=PLxyz", "", "", false},
		{"yt watch missing v", "https://www.youtube.com/watch?list=PLxyz", "", "", false},
		{"yt empty embed", "https://www.youtube.com/embed/", "", "", false},

		// Vimeo positive
		{"vimeo basic", "https://vimeo.com/123456789", ProviderVimeo, "123456789", true},
		{"vimeo www", "https://www.vimeo.com/123456789", ProviderVimeo, "123456789", true},
		{"vimeo with hash", "https://vimeo.com/123456789/abc123def", ProviderVimeo, "123456789/abc123def", true},
		{"vimeo channels", "https://vimeo.com/channels/staffpicks/123456789", ProviderVimeo, "123456789", true},
		{"vimeo groups", "https://vimeo.com/groups/cinema/videos/987654321", ProviderVimeo, "987654321", true},
		{"vimeo player", "https://player.vimeo.com/video/123456789", ProviderVimeo, "123456789", true},
		{"vimeo player with h", "https://player.vimeo.com/video/123456789?h=abc123", ProviderVimeo, "123456789/abc123", true},
		{"vimeo player path hash", "https://player.vimeo.com/video/123456789/abc123", ProviderVimeo, "123456789/abc123", true},

		// Vimeo negative
		{"vimeo channel only", "https://vimeo.com/channels/staffpicks", "", "", false},
		{"vimeo non-numeric", "https://vimeo.com/some-name", "", "", false},

		// Wistia positive
		{"wistia medias vendor", "https://example.wistia.com/medias/abcdef1234", ProviderWistia, "abcdef1234", true},
		{"wistia medias net", "https://example.wistia.net/medias/abcdef1234", ProviderWistia, "abcdef1234", true},
		{"wistia embed iframe", "https://fast.wistia.net/embed/iframe/abcdef1234", ProviderWistia, "abcdef1234", true},
		{"wistia fast com iframe", "https://fast.wistia.com/embed/iframe/abcdef1234", ProviderWistia, "abcdef1234", true},

		// Wistia negative
		{"wistia channel", "https://example.wistia.com/channel/abc123", "", "", false},
		{"wistia showcase", "https://example.wistia.com/showcase/xyz789", "", "", false},

		// Generic negative
		{"random url", "https://example.com/foo", "", "", false},
		{"empty", "", "", "", false},
		{"not a url", "blah", "", "", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			provider, videoID, ok := DetectStreamingVideo(c.url)
			if ok != c.ok || provider != c.provider || videoID != c.videoID {
				t.Errorf("DetectStreamingVideo(%q) = (%q, %q, %v); want (%q, %q, %v)",
					c.url, provider, videoID, ok, c.provider, c.videoID, c.ok)
			}
		})
	}
}

func TestFetchStreamingMetadata_StubbedHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"title":"Stub Title","thumbnail_url":"https://example.com/thumb.jpg"}`))
	}))
	defer srv.Close()

	// Build a transport that rewrites every outbound oEmbed call to our test server.
	client := &http.Client{
		Transport: rewriteTransport{base: http.DefaultTransport, target: srv.URL},
	}

	for _, p := range []StreamProvider{ProviderYouTube, ProviderVimeo, ProviderWistia} {
		title, thumb, err := fetchStreamingMetadataWithClient(context.Background(), client, p, "id", "https://example.com/v/id")
		if err != nil {
			t.Fatalf("provider %s: unexpected err: %v", p, err)
		}
		if title != "Stub Title" || thumb != "https://example.com/thumb.jpg" {
			t.Errorf("provider %s: title=%q thumb=%q; want stub values", p, title, thumb)
		}
	}
}

func TestFetchStreamingMetadata_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := &http.Client{
		Transport: rewriteTransport{base: http.DefaultTransport, target: srv.URL},
	}
	_, _, err := fetchStreamingMetadataWithClient(context.Background(), client, ProviderYouTube, "id", "https://example.com/v/id")
	if err == nil {
		t.Error("expected error on 404, got nil")
	}
}

// rewriteTransport sends every request to a fixed target host while preserving
// the original path/query — lets a single httptest.Server stand in for all
// three oEmbed providers.
type rewriteTransport struct {
	base   http.RoundTripper
	target string
}

func (r rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	target := strings.TrimRight(r.target, "/")
	clone.URL.Scheme = "http"
	clone.URL.Host = strings.TrimPrefix(strings.TrimPrefix(target, "http://"), "https://")
	clone.Host = clone.URL.Host
	return r.base.RoundTrip(clone)
}

func TestIsStreamingMime(t *testing.T) {
	cases := map[string]bool{
		"video/youtube":     true,
		"video/vimeo":       true,
		"video/wistia":      true,
		"video/mp4":         false,
		"link":              false,
		"image/jpeg":        false,
		"":                  false,
	}
	for mime, want := range cases {
		if got := IsStreamingMime(mime); got != want {
			t.Errorf("IsStreamingMime(%q)=%v want %v", mime, got, want)
		}
	}
}
