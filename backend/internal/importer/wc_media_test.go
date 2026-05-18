package importer

import (
	"errors"
	"os"
	"testing"
)

func TestWPSanitizeTitle(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"IK FOAM Pro 2-1", "ik-foam-pro-2-1"},
		{"ik-foam-pro-2-1", "ik-foam-pro-2-1"},
		{"  Hello   World  ", "hello-world"},
		{"foo_bar/baz", "foo-bar-baz"},
		{"Hello, World!", "hello-world"},
		{"---trim---", "trim"},
		{"專業級泡沫", ""},
	}
	for _, tc := range cases {
		got := wpSanitizeTitle(tc.in)
		if got != tc.want {
			t.Errorf("wpSanitizeTitle(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestResolveMediaSlug_Live hits the real https://gyeon.hk WP endpoint to
// confirm each ladder rung actually resolves on the production data.
// Skipped unless GYEON_LIVE_IT=1 to keep CI offline-safe.
func TestResolveMediaSlug_Live(t *testing.T) {
	if os.Getenv("GYEON_LIVE_IT") != "1" {
		t.Skip("set GYEON_LIVE_IT=1 to run the live WP integration check")
	}
	c := newWCClient("https://gyeon.hk", "", "")

	t.Run("title-as-meta (product 12917 banner_1)", func(t *testing.T) {
		// Pre-fix this returned errMediaSlugNotFound.
		item, err := c.resolveMediaSlug("IK FOAM Pro 2-1")
		if err != nil {
			t.Fatalf("resolveMediaSlug: %v", err)
		}
		if item.Slug != "ik-foam-pro-2-1" {
			t.Errorf("got slug %q, want ik-foam-pro-2-1", item.Slug)
		}
		if item.SourceURL == "" {
			t.Errorf("empty source_url")
		}
	})

	t.Run("plain slug", func(t *testing.T) {
		item, err := c.resolveMediaSlug("ik-foam-pro-2-1")
		if err != nil {
			t.Fatalf("resolveMediaSlug: %v", err)
		}
		if item.ID == 0 || item.SourceURL == "" {
			t.Errorf("unexpected empty item %+v", item)
		}
	})

	t.Run("numeric id", func(t *testing.T) {
		item, err := c.resolveMediaSlug("12916")
		if err != nil {
			t.Fatalf("resolveMediaSlug: %v", err)
		}
		if item.ID != 12916 {
			t.Errorf("got id %d, want 12916", item.ID)
		}
	})

	t.Run("full url", func(t *testing.T) {
		u := "https://gyeon.hk/wp-content/uploads/2025/07/IK-FOAM-Pro-2-1.jpg"
		item, err := c.resolveMediaSlug(u)
		if err != nil {
			t.Fatalf("resolveMediaSlug: %v", err)
		}
		if item.SourceURL != u {
			t.Errorf("got source_url %q, want %q", item.SourceURL, u)
		}
	})

	t.Run("not found surfaces sentinel", func(t *testing.T) {
		_, err := c.resolveMediaSlug("definitely-not-a-real-attachment-zzz")
		if !errors.Is(err, errMediaSlugNotFound) {
			t.Errorf("got %v, want errMediaSlugNotFound", err)
		}
	})
}
