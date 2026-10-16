// Package usend is the official Go SDK for usend, the email API for developers.
//
// It uses only the standard library. Create a Client with New and call the
// resource services:
//
//	client, err := usend.New("us_...")
//	res, err := client.Emails.Send(ctx, &usend.SendEmailParams{
//		From:    "Acme <hello@acme.com>",
//		To:      []string{"ann@example.com"},
//		Subject: "Hello",
//		HTML:    "<p>It works.</p>",
//	})
package usend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Version is the SDK version, sent in the User-Agent header.
const Version = "1.3.0"

// DefaultBaseURL is the hosted usend API.
const DefaultBaseURL = "https://api.usend.et"

const (
	defaultTimeout    = 30 * time.Second
	defaultMaxRetries = 2
	maxRetryDelay     = 60 * time.Second
)

// Client talks to the usend API. It is safe for concurrent use.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	maxRetries int
	userAgent  string

	Emails       *EmailsService
	Domains      *DomainsService
	APIKeys      *APIKeysService
	Templates    *TemplatesService
	Contacts     *ContactsService
	Segments     *SegmentsService
	Audiences    *AudiencesService
	Topics       *TopicsService
	Properties   *ContactPropertiesService
	Broadcasts   *BroadcastsService
	Webhooks     *WebhooksService
	Suppressions *SuppressionsService
	Inbound      *InboundService
	Events       *EventsService
	Automations  *AutomationsService
	Inboxes      *InboxesService
	Stats        *StatsService
	ShortLinks   *ShortLinksService
	Logs         *LogsService
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL sets the API base URL (default https://api.usend.et, or the
// USEND_BASE_URL environment variable).
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

// WithHTTPClient sets the *http.Client used for requests.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) {
		if h != nil {
			c.httpClient = h
		}
	}
}

// WithTimeout sets the per-attempt timeout of the HTTP client (default 30s).
// The client is copied first, so a client passed to WithHTTPClient is not
// modified. Put WithHTTPClient before WithTimeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		hc := *c.httpClient
		hc.Timeout = d
		c.httpClient = &hc
	}
}

// WithMaxRetries sets how many times a request is retried on 429 and 5xx
// responses (default 2; 0 disables retries). A POST or PATCH that failed with a
// 5xx is only retried when it carries an idempotency key; a 429 is always safe
// to retry. Retry-After is honored (up to 60 seconds).
func WithMaxRetries(n int) Option {
	return func(c *Client) {
		if n < 0 {
			n = 0
		}
		c.maxRetries = n
	}
}

// New creates a Client. An empty apiKey falls back to USEND_API_KEY.
func New(apiKey string, opts ...Option) (*Client, error) {
	if apiKey == "" {
		apiKey = os.Getenv("USEND_API_KEY")
	}
	if apiKey == "" {
		return nil, errors.New(`usend: missing API key; pass it to usend.New("us_...") or set USEND_API_KEY`)
	}
	c := &Client{
		apiKey:     apiKey,
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{Timeout: defaultTimeout},
		maxRetries: defaultMaxRetries,
		userAgent:  "usend-go/" + Version,
	}
	if env := os.Getenv("USEND_BASE_URL"); env != "" {
		c.baseURL = strings.TrimRight(env, "/")
	}
	for _, o := range opts {
		o(c)
	}
	c.Emails = &EmailsService{c}
	c.Domains = &DomainsService{c}
	c.APIKeys = &APIKeysService{c}
	c.Templates = &TemplatesService{c}
	c.Contacts = &ContactsService{c}
	c.Segments = &SegmentsService{c}
	c.Audiences = &AudiencesService{c}
	c.Topics = &TopicsService{c}
	c.Properties = &ContactPropertiesService{c}
	c.Broadcasts = &BroadcastsService{c}
	c.Webhooks = &WebhooksService{c}
	c.Suppressions = &SuppressionsService{c}
	c.Inbound = &InboundService{c}
	c.Events = &EventsService{c}
	c.Automations = &AutomationsService{c}
	c.Inboxes = &InboxesService{c}
	c.Stats = &StatsService{c}
	c.ShortLinks = &ShortLinksService{c}
	c.Logs = &LogsService{c}
	return c, nil
}

// BaseURL returns the API base URL in use.
func (c *Client) BaseURL() string { return c.baseURL }

// RequestOption adjusts a single request.
type RequestOption func(http.Header)

// WithIdempotencyKey sets the Idempotency-Key header. The API replays the first
// response for the same key for 24 hours.
func WithIdempotencyKey(key string) RequestOption {
	return func(h http.Header) { h.Set("Idempotency-Key", key) }
}

// WithBatchValidation sets x-batch-validation ("strict" or "permissive") on a batch send.
func WithBatchValidation(mode string) RequestOption {
	return func(h http.Header) { h.Set("x-batch-validation", mode) }
}

// WithHeader sets an arbitrary request header.
func WithHeader(key, value string) RequestOption {
	return func(h http.Header) { h.Set(key, value) }
}

// Request is the escape hatch for endpoints without a typed method. body is
// JSON-encoded when non-nil; the JSON response is decoded into out when out is
// non-nil. path starts with "/" (for example "/logs?days=7").
//
// A non-2xx response is returned as *Error.
func (c *Client) Request(ctx context.Context, method, path string, body, out any, opts ...RequestOption) error {
	var payload []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("usend: encode request: %w", err)
		}
		payload = b
	}
	hdr := http.Header{}
	for _, o := range opts {
		o(hdr)
	}
	// Repeating a non-idempotent request after a 5xx could send twice.
	safeRetry := method == http.MethodGet || method == http.MethodDelete || method == http.MethodPut || hdr.Get("Idempotency-Key") != ""

	var lastErr error
	for attempt := 0; ; attempt++ {
		var retryAfter time.Duration
		retry := false

		resp, err := c.do(ctx, method, path, payload, hdr)
		if err != nil {
			if ctx.Err() != nil {
				return err
			}
			lastErr = err
			retry = safeRetry
		} else {
			data, rerr := io.ReadAll(resp.Body)
			resp.Body.Close()
			switch {
			case rerr != nil:
				lastErr = fmt.Errorf("usend: read response: %w", rerr)
				retry = safeRetry
			case resp.StatusCode >= 200 && resp.StatusCode < 300:
				if out == nil || len(bytes.TrimSpace(data)) == 0 {
					return nil
				}
				if err := json.Unmarshal(data, out); err != nil {
					return fmt.Errorf("usend: decode response: %w", err)
				}
				return nil
			default:
				apiErr := parseError(resp, data)
				lastErr = apiErr
				retryAfter = apiErr.RetryAfter
				retry = resp.StatusCode == http.StatusTooManyRequests || (resp.StatusCode >= 500 && safeRetry)
			}
		}
		if !retry || attempt >= c.maxRetries {
			return lastErr
		}
		delay := backoff(attempt)
		if retryAfter > 0 {
			if retryAfter > maxRetryDelay {
				return lastErr // the server wants us to wait too long
			}
			delay = retryAfter
		}
		t := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
}

func (c *Client) do(ctx context.Context, method, path string, payload []byte, hdr http.Header) (*http.Response, error) {
	var rd io.Reader
	if payload != nil {
		rd = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rd)
	if err != nil {
		return nil, fmt.Errorf("usend: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range hdr {
		req.Header[k] = v
	}
	return c.httpClient.Do(req)
}

func parseError(resp *http.Response, data []byte) *Error {
	e := &Error{}
	if json.Unmarshal(data, e) != nil || e.Name == "" {
		e = &Error{Name: "application_error", Message: "HTTP " + strconv.Itoa(resp.StatusCode)}
	}
	e.StatusCode = resp.StatusCode // the status line is authoritative
	e.RetryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
	return e
}

func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.ParseFloat(v, 64); err == nil && secs >= 0 {
		return time.Duration(secs * float64(time.Second))
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

func backoff(attempt int) time.Duration {
	d := 500 * time.Millisecond << uint(attempt)
	if d > 8*time.Second {
		d = 8 * time.Second
	}
	return d
}

func esc(s string) string { return url.PathEscape(s) }

// ListParams are the pagination parameters of list endpoints. Rows are newest
// first; After returns rows older than the given ID, Before newer ones.
type ListParams struct {
	Limit  int    // 1-100, default 50
	After  string // row ID
	Before string // row ID
}

func (p *ListParams) values() url.Values {
	v := url.Values{}
	if p == nil {
		return v
	}
	if p.Limit > 0 {
		v.Set("limit", strconv.Itoa(p.Limit))
	}
	if p.After != "" {
		v.Set("after", p.After)
	}
	if p.Before != "" {
		v.Set("before", p.Before)
	}
	return v
}

func withQuery(path string, v url.Values) string {
	if len(v) == 0 {
		return path
	}
	return path + "?" + v.Encode()
}

// List is the envelope of list endpoints. HasMore is only set by paginated lists.
type List[T any] struct {
	Object  string `json:"object"`
	Data    []T    `json:"data"`
	HasMore bool   `json:"has_more"`
}

// IDResult is the minimal { object, id } response of create and update calls.
type IDResult struct {
	Object string `json:"object"`
	ID     string `json:"id"`
}

// DeleteResult is the response of delete calls on most resources.
type DeleteResult struct {
	Object  string `json:"object"`
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}
