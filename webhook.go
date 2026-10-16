package usend

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Errors returned by VerifyWebhook. Test with errors.Is.
var (
	ErrMissingWebhookHeaders = errors.New("usend: missing webhook-id, webhook-timestamp or webhook-signature header")
	ErrWebhookTimestamp      = errors.New("usend: webhook timestamp is too old or too new")
	ErrInvalidWebhookSecret  = errors.New("usend: invalid webhook signing secret")
	ErrInvalidSignature      = errors.New("usend: invalid webhook signature")
)

// DefaultWebhookTolerance is how far a webhook timestamp may be from now.
const DefaultWebhookTolerance = 5 * time.Minute

// WebhookEvent is a verified webhook payload. Data depends on Type; decode it
// with json.Unmarshal into your own struct or a map.
type WebhookEvent struct {
	ID        string          `json:"-"` // the webhook-id header; unique per delivery
	Type      string          `json:"type"`
	CreatedAt time.Time       `json:"created_at"`
	Data      json.RawMessage `json:"data"`
}

// VerifyOption configures VerifyWebhook.
type VerifyOption func(*verifyConfig)

type verifyConfig struct {
	tolerance time.Duration
	now       func() time.Time
}

// WithTolerance sets the allowed clock difference (default 5 minutes).
func WithTolerance(d time.Duration) VerifyOption {
	return func(c *verifyConfig) { c.tolerance = d }
}

// VerifyWebhook checks a webhook delivery (Standard Webhooks) and returns the
// parsed event. payload must be the raw request body, byte for byte; header is
// the request's headers; secret is the webhook's signing secret ("whsec_...").
//
//	body, _ := io.ReadAll(r.Body)
//	event, err := usend.VerifyWebhook(body, r.Header, os.Getenv("USEND_WEBHOOK_SECRET"))
//	if err != nil { http.Error(w, "invalid signature", http.StatusBadRequest); return }
func VerifyWebhook(payload []byte, header http.Header, secret string, opts ...VerifyOption) (*WebhookEvent, error) {
	cfg := verifyConfig{tolerance: DefaultWebhookTolerance, now: time.Now}
	for _, o := range opts {
		o(&cfg)
	}
	get := func(name string) string {
		if v := header.Get("webhook-" + name); v != "" {
			return v
		}
		return header.Get("svix-" + name) // usend sends both header sets
	}
	id, ts, sigs := get("id"), get("timestamp"), get("signature")
	if id == "" || ts == "" || sigs == "" {
		return nil, ErrMissingWebhookHeaders
	}
	secs, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return nil, ErrWebhookTimestamp
	}
	diff := cfg.now().Sub(time.Unix(secs, 0))
	if diff < 0 {
		diff = -diff
	}
	if diff > cfg.tolerance {
		return nil, ErrWebhookTimestamp
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	if err != nil || len(key) == 0 {
		return nil, ErrInvalidWebhookSecret
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + ts + "."))
	mac.Write(payload)
	expected := mac.Sum(nil)

	ok := false
	for _, part := range strings.Fields(sigs) {
		version, sig, found := strings.Cut(part, ",")
		if !found || version != "v1" {
			continue
		}
		got, err := base64.StdEncoding.DecodeString(sig)
		if err != nil {
			continue
		}
		if hmac.Equal(got, expected) { // constant time
			ok = true
		}
	}
	if !ok {
		return nil, ErrInvalidSignature
	}
	var ev WebhookEvent
	if err := json.Unmarshal(payload, &ev); err != nil {
		return nil, err
	}
	ev.ID = id
	return &ev, nil
}
