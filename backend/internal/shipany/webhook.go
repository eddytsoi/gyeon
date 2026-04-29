package shipany

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
)

// TrackingEvent is the canonical inbound payload from a ShipAny callback.
// The actual envelope ShipAny sends is not publicly documented; we accept
// either a tracking_number-keyed or a shipment_id-keyed event.
type TrackingEvent struct {
	Event          string          `json:"event"`
	TrackingNumber string          `json:"tracking_number,omitempty"`
	ShipmentID     string          `json:"shipment_id,omitempty"`
	OccurredAt     string          `json:"occurred_at,omitempty"`
	Data           json.RawMessage `json:"data,omitempty"`
}

var ErrInvalidSignature = errors.New("invalid shipany webhook signature")

// VerifyWebhook validates a hex-encoded HMAC-SHA256 signature header.
// The actual scheme ShipAny uses must be confirmed against portal docs;
// implement other schemes here if it differs.
func (s *Service) VerifyWebhook(ctx context.Context, body []byte, sigHeader string) error {
	secret := s.read(ctx, "shipany_webhook_secret")
	if secret == "" {
		return ErrNotConfigured
	}
	if sigHeader == "" {
		return ErrInvalidSignature
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(expected), []byte(sigHeader)) != 1 {
		return ErrInvalidSignature
	}
	return nil
}

// ParseEvent decodes a tracking event payload.
func ParseEvent(body []byte) (*TrackingEvent, error) {
	var e TrackingEvent
	if err := json.Unmarshal(body, &e); err != nil {
		return nil, err
	}
	return &e, nil
}
