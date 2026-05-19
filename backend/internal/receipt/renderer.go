package receipt

import (
	"container/list"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// pdfCacheSize is the LRU cap. Average receipt PDF is ~30-80 KB, so 256
// entries stays comfortably under ~20 MB even on the upper end.
const pdfCacheSize = 256

// Renderer wraps a headless Chromium process and renders HTML pages to PDF.
// One allocator is reused across requests so the second receipt onwards skips
// the ~300ms browser cold-start. The mutex below only guards lazy allocator
// initialisation — once that's done, each render opens its own tab and runs
// concurrently with other renders against the same browser.
type Renderer struct {
	mu        sync.Mutex
	allocCtx  context.Context
	allocStop context.CancelFunc

	cache *pdfCache
}

func NewRenderer() *Renderer {
	return &Renderer{cache: newPDFCache(pdfCacheSize)}
}

// ensureAlloc lazily spins up the long-lived allocator. Caller must hold r.mu.
func (r *Renderer) ensureAlloc() {
	if r.allocCtx != nil {
		return
	}
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.NoSandbox,
		chromedp.DisableGPU,
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("headless", "new"),
		chromedp.Flag("hide-scrollbars", true),
	)
	r.allocCtx, r.allocStop = chromedp.NewExecAllocator(context.Background(), opts...)
}

// Close terminates the underlying browser. Safe to call multiple times.
func (r *Renderer) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.allocStop != nil {
		r.allocStop()
		r.allocStop = nil
		r.allocCtx = nil
	}
}

// Render returns a PDF byte slice for the given HTML document. The HTML must
// be self-contained: external <link>/<script>/<img> URLs are fetched by
// Chromium, so make sure remote assets are reachable from the API host.
func (r *Renderer) Render(ctx context.Context, html string) ([]byte, error) {
	r.mu.Lock()
	r.ensureAlloc()
	allocCtx := r.allocCtx
	r.mu.Unlock()

	// Each render gets a fresh tab; the browser itself is reused.
	tabCtx, cancelTab := chromedp.NewContext(allocCtx)
	defer cancelTab()

	// Hard cap per render — keeps a wedged Chromium from holding the request
	// goroutine forever.
	tabCtx, cancelTimeout := context.WithTimeout(tabCtx, 30*time.Second)
	defer cancelTimeout()

	var pdf []byte
	dataURL := "data:text/html;charset=utf-8," + encodeForDataURL(html)
	err := chromedp.Run(tabCtx,
		chromedp.Navigate(dataURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.ActionFunc(func(ctx context.Context) error {
			data, _, err := page.PrintToPDF().
				WithPrintBackground(true).
				WithPreferCSSPageSize(true).
				Do(ctx)
			if err != nil {
				return fmt.Errorf("printToPDF: %w", err)
			}
			pdf = data
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}
	return pdf, nil
}

// RenderCached returns the PDF for the given cache key, generating it lazily
// from buildHTML on a miss. buildHTML is a closure so the expensive work
// (image inlining, template execution) is also skipped on a hit. The boolean
// reports whether the result came from cache, for instrumentation.
//
// Callers are responsible for embedding any inputs that should bust the cache
// (order updated_at, locale, branding version, …) into key. An empty key
// bypasses the cache entirely — used as a safe escape hatch when an input
// needed for keying is unavailable.
func (r *Renderer) RenderCached(ctx context.Context, key string, buildHTML func() (string, error)) ([]byte, bool, error) {
	if key != "" {
		if pdf, ok := r.cache.get(key); ok {
			return pdf, true, nil
		}
	}
	html, err := buildHTML()
	if err != nil {
		return nil, false, err
	}
	pdf, err := r.Render(ctx, html)
	if err != nil {
		return nil, false, err
	}
	if key != "" {
		r.cache.add(key, pdf)
	}
	return pdf, false, nil
}

// pdfCache is a tiny LRU keyed by string, holding []byte payloads. Concurrent
// access is serialised with a mutex — the contended path is cheap (map +
// list ops, no allocation on hits) and avoids pulling in a dependency for
// something this small.
type pdfCache struct {
	mu    sync.Mutex
	cap   int
	items map[string]*list.Element
	order *list.List
}

type pdfCacheEntry struct {
	key string
	val []byte
}

func newPDFCache(cap int) *pdfCache {
	return &pdfCache{
		cap:   cap,
		items: make(map[string]*list.Element, cap),
		order: list.New(),
	}
}

func (c *pdfCache) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return nil, false
	}
	c.order.MoveToFront(el)
	return el.Value.(*pdfCacheEntry).val, true
}

func (c *pdfCache) add(key string, val []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		el.Value.(*pdfCacheEntry).val = val
		c.order.MoveToFront(el)
		return
	}
	el := c.order.PushFront(&pdfCacheEntry{key: key, val: val})
	c.items[key] = el
	if c.order.Len() > c.cap {
		oldest := c.order.Back()
		if oldest != nil {
			c.order.Remove(oldest)
			delete(c.items, oldest.Value.(*pdfCacheEntry).key)
		}
	}
}

// encodeForDataURL percent-encodes the bytes that would otherwise break a
// data: URL — primarily `#` (fragment start), `%` (escape char) and `&`
// (parameter delimiter when the data URL is itself in a larger query). Keeping
// the encoding minimal preserves the data URL's readability when debugging
// rendering issues by pasting it into a browser.
func encodeForDataURL(s string) string {
	var b []byte
	b = make([]byte, 0, len(s)+32)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '#', '%', '&':
			b = append(b, '%')
			const hex = "0123456789ABCDEF"
			b = append(b, hex[c>>4], hex[c&0xF])
		default:
			b = append(b, c)
		}
	}
	return string(b)
}
