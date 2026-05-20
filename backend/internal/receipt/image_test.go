package receipt

import "testing"

func TestToResizedWebpURL(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		width int
		want  string
	}{
		{
			name:  "relative jpg becomes 160 webp",
			in:    "/uploads/foo.jpg",
			width: 160,
			want:  "/uploads/r/160/foo.webp",
		},
		{
			name:  "absolute png becomes 160 webp",
			in:    "https://gyeon.hk/uploads/foo.png",
			width: 160,
			want:  "https://gyeon.hk/uploads/r/160/foo.webp",
		},
		{
			name:  "jpeg ext also rewrites",
			in:    "/uploads/photo.jpeg",
			width: 320,
			want:  "/uploads/r/320/photo.webp",
		},
		{
			name:  "webp source keeps .webp ext",
			in:    "/uploads/foo.webp",
			width: 160,
			want:  "/uploads/r/160/foo.webp",
		},
		{
			name:  "virtual webp suffix preserved",
			in:    "/uploads/foo.jpg.webp",
			width: 160,
			// .webp ext is preserved (not .jpg.webp -> .jpg.webp). toWebpFilename
			// only swaps .jpg/.jpeg/.png — `.webp` falls through unchanged, so
			// the backend's virtual .webp fallback (resize.go::webpBaseFallback
			// pattern 1) is the one that resolves this on disk.
			want: "/uploads/r/160/foo.jpg.webp",
		},
		{
			name:  "svg logo passes through unchanged",
			in:    "/uploads/logo.svg",
			width: 320,
			// SVG can't go through the resize endpoint (resizableExt rejects
			// it → 404). It's served by the plain /uploads/ FileServer with
			// MIME image/svg+xml, so the rewrite must leave the URL alone.
			want: "/uploads/logo.svg",
		},
		{
			name:  "absolute svg logo passes through unchanged",
			in:    "https://gyeon.hk/uploads/logo.svg",
			width: 320,
			want:  "https://gyeon.hk/uploads/logo.svg",
		},
		{
			name:  "gif passes through unchanged",
			in:    "/uploads/animation.gif",
			width: 160,
			want:  "/uploads/animation.gif",
		},
		{
			name:  "pdf passes through unchanged",
			in:    "/uploads/document.pdf",
			width: 160,
			want:  "/uploads/document.pdf",
		},
		{
			name:  "no extension passes through unchanged",
			in:    "/uploads/somefile",
			width: 160,
			want:  "/uploads/somefile",
		},
		{
			name:  "already resized URL passes through",
			in:    "/uploads/r/640/foo.webp",
			width: 160,
			want:  "/uploads/r/640/foo.webp",
		},
		{
			name:  "external CDN passes through",
			in:    "https://cdn.example.com/logo.png",
			width: 320,
			want:  "https://cdn.example.com/logo.png",
		},
		{
			name:  "data URI passes through",
			in:    "data:image/png;base64,iVBORw0KGgo=",
			width: 320,
			want:  "data:image/png;base64,iVBORw0KGgo=",
		},
		{
			name:  "empty stays empty",
			in:    "",
			width: 160,
			want:  "",
		},
		{
			name:  "dotdot traversal passes through",
			in:    "/uploads/../etc/passwd",
			width: 160,
			want:  "/uploads/../etc/passwd",
		},
		{
			name:  "nested subpath under uploads passes through",
			in:    "/uploads/sub/foo.jpg",
			width: 160,
			want:  "/uploads/sub/foo.jpg",
		},
		{
			name:  "query string stripped",
			in:    "/uploads/foo.jpg?v=2",
			width: 160,
			want:  "/uploads/r/160/foo.webp",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := toResizedWebpURL(tc.in, tc.width)
			if got != tc.want {
				t.Errorf("toResizedWebpURL(%q, %d) = %q; want %q", tc.in, tc.width, got, tc.want)
			}
		})
	}
}
