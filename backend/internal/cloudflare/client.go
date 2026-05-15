// Package cloudflare wraps the small subset of Cloudflare API calls we use
// (cache purge by URL list, and "purge everything"). Credentials live in
// site_settings under cloudflare_zone_id + cloudflare_api_token — callers
// resolve them and pass in to keep this package dependency-free.
package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
)

// ErrNotConfigured is returned when zone ID or API token is missing. The
// admin "Clear all cache" handler surfaces this to the user; the media
// purge-on-delete path swallows it so deletes succeed without CF wired up.
var ErrNotConfigured = errors.New("cloudflare credentials not configured")

const apiBase = "https://api.cloudflare.com/client/v4"

// PurgeFiles asks Cloudflare to drop cached entries for the given URLs.
// Returns ErrNotConfigured if credentials are missing (caller can decide to
// log-and-ignore vs surface).
func PurgeFiles(ctx context.Context, zone, token string, urls []string) error {
	if len(urls) == 0 {
		return nil
	}
	if zone == "" || token == "" {
		return ErrNotConfigured
	}
	body, _ := json.Marshal(map[string][]string{"files": urls})
	return postPurge(ctx, zone, token, body)
}

// PurgeAll triggers a full zone purge ({"purge_everything": true}). Drains
// all edge caches for this zone — heavy hammer; intended for admin-triggered
// "Clear all cache" only.
func PurgeAll(ctx context.Context, zone, token string) error {
	if zone == "" || token == "" {
		return ErrNotConfigured
	}
	body, _ := json.Marshal(map[string]bool{"purge_everything": true})
	return postPurge(ctx, zone, token, body)
}

func postPurge(ctx context.Context, zone, token string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/zones/%s/purge_cache", apiBase, zone),
		bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build purge request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("cloudflare purge: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		log.Printf("cloudflare purge: status %d body=%s", resp.StatusCode, string(respBody))
		return fmt.Errorf("cloudflare returned status %d", resp.StatusCode)
	}
	return nil
}
