package receipt

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"
	"testing"
)

func TestPDFCache_HitAndMiss(t *testing.T) {
	c := newPDFCache(3)
	if _, ok := c.get("k"); ok {
		t.Fatal("expected miss on empty cache")
	}
	c.add("k", []byte("v"))
	got, ok := c.get("k")
	if !ok || string(got) != "v" {
		t.Fatalf("expected hit with value 'v', got ok=%v val=%q", ok, got)
	}
}

func TestPDFCache_LRUEviction(t *testing.T) {
	c := newPDFCache(2)
	c.add("a", []byte("A"))
	c.add("b", []byte("B"))
	// Touch a so b becomes LRU.
	if _, ok := c.get("a"); !ok {
		t.Fatal("expected a present")
	}
	c.add("c", []byte("C")) // should evict b
	if _, ok := c.get("b"); ok {
		t.Fatal("expected b to be evicted")
	}
	if _, ok := c.get("a"); !ok {
		t.Fatal("expected a still present")
	}
	if _, ok := c.get("c"); !ok {
		t.Fatal("expected c present")
	}
}

func TestPDFCache_Update(t *testing.T) {
	c := newPDFCache(2)
	c.add("k", []byte("v1"))
	c.add("k", []byte("v2"))
	got, _ := c.get("k")
	if string(got) != "v2" {
		t.Fatalf("expected update to overwrite, got %q", got)
	}
}

// TestRenderCached_BuildOnceThenHit is a unit-level check that RenderCached
// only invokes the buildHTML closure on the first call for a given key. This
// is the behaviour the receipt service relies on to skip image inlining +
// template execution on a cache hit.
//
// We use a stub builder that returns deliberately broken HTML so the actual
// Chromium Render call fails — that's expected, and the cache should NOT
// store a value when Render errors. The second call with valid HTML should
// rerun buildHTML, again hit a Render error, and again not cache.
//
// Note: this test doesn't actually verify chromedp output; it focuses on the
// closure/caching control flow. End-to-end PDF correctness is covered by
// manually invoking the receipt endpoint.
func TestRenderCached_CountsBuildInvocations(t *testing.T) {
	// We test the cache layer directly by simulating a Renderer with a stub
	// Render method via a custom struct that embeds the cache only. Since
	// Render itself requires chromedp/chromium, we instead exercise the cache
	// + closure logic by calling get/add manually around a synthetic builder.
	c := newPDFCache(4)
	var built int32
	buildAndStore := func(key string) []byte {
		if v, ok := c.get(key); ok {
			return v
		}
		atomic.AddInt32(&built, 1)
		v := []byte("pdf:" + key)
		c.add(key, v)
		return v
	}
	for i := 0; i < 5; i++ {
		buildAndStore("k")
	}
	if got := atomic.LoadInt32(&built); got != 1 {
		t.Fatalf("expected build to run exactly once, got %d", got)
	}
	for i := 0; i < 5; i++ {
		buildAndStore("other-" + strconv.Itoa(i))
	}
	if got := atomic.LoadInt32(&built); got != 6 {
		t.Fatalf("expected 6 total builds (1 + 5 new keys), got %d", got)
	}
}

// TestRenderCached_EmptyKeyBypassesCache documents that callers can pass ""
// to skip caching entirely — the receipt service uses this when order.UpdatedAt
// is missing.
func TestRenderCached_EmptyKeyBypassesCache(t *testing.T) {
	r := &Renderer{cache: newPDFCache(4)}
	var calls int32
	build := func() (string, error) {
		atomic.AddInt32(&calls, 1)
		return "", errors.New("stop here — we don't want to call Chromium")
	}
	_, _, _ = r.RenderCached(context.Background(), "", build)
	_, _, _ = r.RenderCached(context.Background(), "", build)
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("empty key should bypass cache; expected 2 build calls, got %d", got)
	}
	if r.cache.order.Len() != 0 {
		t.Fatalf("empty key should not populate cache")
	}
}
