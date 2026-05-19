package receipt

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// Renderer wraps a headless Chromium process and renders HTML pages to PDF.
// One allocator is reused across requests so the second receipt onwards skips
// the ~300ms browser cold-start. Concurrency is serialised: chromedp can
// share an allocator across contexts, but issuing many parallel Page.printToPDF
// calls against the same browser quickly exhausts file descriptors and is not
// worth the complexity for a receipt endpoint that fires once per order view.
type Renderer struct {
	mu        sync.Mutex
	allocCtx  context.Context
	allocStop context.CancelFunc
}

func NewRenderer() *Renderer { return &Renderer{} }

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
