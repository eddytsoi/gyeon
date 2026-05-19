package receipt

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// maxImageBytes caps each fetched image at 2 MB. Anything larger is treated as
// a fetch failure (returns "" so the template renders a blank cell). 2 MB is
// well above any reasonable product thumbnail and well below the size where a
// single image starts to dominate the rendered PDF.
const maxImageBytes = 2 * 1024 * 1024

// imageFetchTimeout is the per-URL deadline. Set tight: a receipt is supposed
// to be fast, and a slow image is strictly worse than a missing image.
const imageFetchTimeout = 2 * time.Second

// imageFetchConcurrency bounds parallel HTTP fetches. 6 is enough for a
// receipt with a logo + a handful of line items without hammering the asset
// host (or our own egress) when many receipts render at once.
const imageFetchConcurrency = 6

// imageClient is shared across calls so we benefit from keep-alives when the
// asset host is the same across line items (typical case: storefront media CDN).
var imageClient = &http.Client{Timeout: imageFetchTimeout}

// inlineImages downloads each unique URL in parallel and returns a map from
// the original URL to a data: URI suitable for embedding in <img src="…">.
// On any per-URL failure the value is "" — the template renders that cell
// blank rather than failing the whole receipt. URLs that are already data:
// or empty are skipped (callers keep the original).
func inlineImages(ctx context.Context, urls []string) map[string]string {
	seen := make(map[string]struct{}, len(urls))
	var unique []string
	for _, u := range urls {
		if u == "" || strings.HasPrefix(u, "data:") {
			continue
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		unique = append(unique, u)
	}
	if len(unique) == 0 {
		return nil
	}

	out := make(map[string]string, len(unique))
	var mu sync.Mutex
	sem := make(chan struct{}, imageFetchConcurrency)
	var wg sync.WaitGroup
	for _, u := range unique {
		wg.Add(1)
		sem <- struct{}{}
		go func(url string) {
			defer wg.Done()
			defer func() { <-sem }()
			data := fetchInline(ctx, url)
			mu.Lock()
			out[url] = data
			mu.Unlock()
		}(u)
	}
	wg.Wait()
	return out
}

// fetchInline returns a data: URI for the given URL, or "" on any failure
// (context cancelled, non-2xx, oversize, transport error, sniff failure).
func fetchInline(ctx context.Context, url string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ""
	}
	resp, err := imageClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ""
	}
	// LimitReader + extra byte trick: read one more than the cap so we can
	// detect oversize without buffering the entire body.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes+1))
	if err != nil || len(body) == 0 || len(body) > maxImageBytes {
		return ""
	}
	mime := http.DetectContentType(body)
	if !strings.HasPrefix(mime, "image/") {
		return ""
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(body)
}
