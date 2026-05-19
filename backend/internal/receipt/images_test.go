package receipt

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// 1×1 transparent PNG, base64-decoded inline so each test serves a real image
// that http.DetectContentType will recognise.
var tinyPNG, _ = base64.StdEncoding.DecodeString(
	"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=",
)

func TestInlineImages_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(tinyPNG)
	}))
	defer srv.Close()

	out := inlineImages(context.Background(), []string{srv.URL + "/a.png"})
	got := out[srv.URL+"/a.png"]
	if !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Fatalf("expected PNG data URI, got %q", got)
	}
}

func TestInlineImages_SkipsEmptyAndDataURIs(t *testing.T) {
	out := inlineImages(context.Background(), []string{"", "data:image/png;base64,XYZ"})
	if len(out) != 0 {
		t.Fatalf("expected no entries for empty/data inputs, got %v", out)
	}
}

func TestInlineImages_Dedupes(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "image/png")
		w.Write(tinyPNG)
	}))
	defer srv.Close()

	url := srv.URL + "/dup.png"
	inlineImages(context.Background(), []string{url, url, url, url})
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("expected 1 HTTP fetch after dedup, got %d", got)
	}
}

func TestInlineImages_FailureReturnsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	out := inlineImages(context.Background(), []string{srv.URL + "/x.png"})
	if v, ok := out[srv.URL+"/x.png"]; !ok || v != "" {
		t.Fatalf("expected empty string for failed fetch, got ok=%v v=%q", ok, v)
	}
}

func TestInlineImages_RejectsNonImageBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png") // lies on header
		w.Write([]byte("<!doctype html><html></html>"))
	}))
	defer srv.Close()

	out := inlineImages(context.Background(), []string{srv.URL + "/lie.png"})
	if out[srv.URL+"/lie.png"] != "" {
		t.Fatalf("expected empty for non-image body (we sniff, don't trust header)")
	}
}

func TestInlineImages_RejectsOversize(t *testing.T) {
	big := make([]byte, maxImageBytes+1024) // intentionally over the cap
	// Pad first bytes with a valid PNG magic so DetectContentType says image/png
	// — that way we know the oversize check, not the sniff, is what rejects it.
	copy(big, tinyPNG)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(big)
	}))
	defer srv.Close()

	out := inlineImages(context.Background(), []string{srv.URL + "/big.png"})
	if out[srv.URL+"/big.png"] != "" {
		t.Fatalf("expected empty for oversize body")
	}
}

func TestInlineImages_RespectsPerFetchTimeout(t *testing.T) {
	// Server sleeps longer than imageFetchTimeout — request must time out and
	// return "" without blocking the test for the full sleep.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(imageFetchTimeout + 500*time.Millisecond)
		w.Write(tinyPNG)
	}))
	defer srv.Close()

	start := time.Now()
	out := inlineImages(context.Background(), []string{srv.URL + "/slow.png"})
	elapsed := time.Since(start)
	if out[srv.URL+"/slow.png"] != "" {
		t.Fatalf("expected timeout to yield empty result")
	}
	if elapsed > imageFetchTimeout+1*time.Second {
		t.Fatalf("inlineImages took %v — should bail at ~%v", elapsed, imageFetchTimeout)
	}
}

func TestInlineImages_ConcurrencyCap(t *testing.T) {
	// Inflight counter should never exceed imageFetchConcurrency. Each handler
	// sleeps briefly so multiple requests overlap in time.
	var inflight, peak int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := atomic.AddInt32(&inflight, 1)
		for {
			old := atomic.LoadInt32(&peak)
			if cur <= old || atomic.CompareAndSwapInt32(&peak, old, cur) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		atomic.AddInt32(&inflight, -1)
		w.Header().Set("Content-Type", "image/png")
		w.Write(tinyPNG)
	}))
	defer srv.Close()

	urls := make([]string, imageFetchConcurrency*3)
	for i := range urls {
		urls[i] = fmt.Sprintf("%s/img-%d.png", srv.URL, i)
	}
	inlineImages(context.Background(), urls)
	if got := atomic.LoadInt32(&peak); got > int32(imageFetchConcurrency) {
		t.Fatalf("peak inflight %d exceeded cap %d", got, imageFetchConcurrency)
	}
}
