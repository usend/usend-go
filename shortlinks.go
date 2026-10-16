package usend

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// ShortLink is a link on the shared tracking host (ShortURL) that redirects to TargetURL and counts clicks.
type ShortLink struct {
	Object          string     `json:"object"`
	ID              string     `json:"id"`
	Code            string     `json:"code"`
	ShortURL        string     `json:"short_url"`
	TargetURL       string     `json:"target_url"`
	Title           string     `json:"title"`
	Clicks          int        `json:"clicks"`
	HumanClicks     int        `json:"human_clicks"`
	ExpiresAt       *time.Time `json:"expires_at"`
	Disabled        bool       `json:"disabled"`
	DisabledByAdmin bool       `json:"disabled_by_admin"`
	CreatedAt       time.Time  `json:"created_at"`
}

// CreateShortLinkParams is the body of ShortLinks.Create. Code is 3 to 32 characters of A-Za-z0-9-_ (random when empty).
type CreateShortLinkParams struct {
	TargetURL string `json:"target_url"`
	Code      string `json:"code,omitempty"`
	Title     string `json:"title,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"` // RFC 3339
}

// UpdateShortLinkParams edits a short link; nil fields are left alone. The code cannot change.
type UpdateShortLinkParams struct {
	TargetURL *string `json:"target_url,omitempty"`
	Title     *string `json:"title,omitempty"`
	Disabled  *bool   `json:"disabled,omitempty"`
	// ExpiresAt sets the expiry (RFC 3339); ClearExpiry removes it.
	ExpiresAt   *string `json:"expires_at,omitempty"`
	ClearExpiry bool    `json:"-"`
}

// ShortLinkClicks is a pair of click counters.
type ShortLinkClicks struct {
	Clicks      int `json:"clicks"`
	HumanClicks int `json:"human_clicks"`
}

// ShortLinkStats are the click statistics of a short link over a period (zero-filled UTC days).
type ShortLinkStats struct {
	Object   string          `json:"object"`
	ID       string          `json:"id"`
	Days     int             `json:"days"`
	Totals   ShortLinkClicks `json:"totals"`
	InPeriod ShortLinkClicks `json:"in_period"`
	Daily    []struct {
		Date        string `json:"date"`
		Clicks      int    `json:"clicks"`
		HumanClicks int    `json:"human_clicks"`
	} `json:"daily"`
	Referrers []struct {
		Referrer string `json:"referrer"`
		Clicks   int    `json:"clicks"`
	} `json:"referrers"`
}

// ShortLinksService is client.ShortLinks.
type ShortLinksService struct{ c *Client }

// Create makes a short link.
func (s *ShortLinksService) Create(ctx context.Context, p *CreateShortLinkParams) (*ShortLink, error) {
	return do[ShortLink](ctx, s.c, http.MethodPost, "/short-links", p)
}

// List returns a page of short links, newest first.
func (s *ShortLinksService) List(ctx context.Context, p *ListParams) (*List[ShortLink], error) {
	return do[List[ShortLink]](ctx, s.c, http.MethodGet, withQuery("/short-links", p.values()), nil)
}

// Get returns a short link by ID.
func (s *ShortLinksService) Get(ctx context.Context, id string) (*ShortLink, error) {
	return do[ShortLink](ctx, s.c, http.MethodGet, "/short-links/"+esc(id), nil)
}

// Update edits a short link.
func (s *ShortLinksService) Update(ctx context.Context, id string, p *UpdateShortLinkParams) (*ShortLink, error) {
	var body any = p
	if p != nil && p.ClearExpiry {
		// `expires_at: null` clears the expiry, which omitempty cannot express.
		m := map[string]any{"expires_at": nil}
		if p.TargetURL != nil {
			m["target_url"] = *p.TargetURL
		}
		if p.Title != nil {
			m["title"] = *p.Title
		}
		if p.Disabled != nil {
			m["disabled"] = *p.Disabled
		}
		body = m
	}
	return do[ShortLink](ctx, s.c, http.MethodPatch, "/short-links/"+esc(id), body)
}

// Remove deletes a short link.
func (s *ShortLinksService) Remove(ctx context.Context, id string) (*DeleteResult, error) {
	return do[DeleteResult](ctx, s.c, http.MethodDelete, "/short-links/"+esc(id), nil)
}

// Stats returns click statistics for the last `days` days (1 to 365; 0 means the default 30).
func (s *ShortLinksService) Stats(ctx context.Context, id string, days int) (*ShortLinkStats, error) {
	v := url.Values{}
	if days > 0 {
		v.Set("days", strconv.Itoa(days))
	}
	return do[ShortLinkStats](ctx, s.c, http.MethodGet, withQuery("/short-links/"+esc(id)+"/stats", v), nil)
}

// LogEntry is one request in the API request log.
type LogEntry struct {
	ID             string         `json:"id"`
	Method         string         `json:"method"`
	Endpoint       string         `json:"endpoint"`
	Path           string         `json:"path"`
	Status         int            `json:"status"`
	DurationMS     int            `json:"duration_ms"`
	Source         string         `json:"source"` // api or dashboard
	UserAgent      string         `json:"user_agent"`
	APIKeyID       *string        `json:"api_key_id"`
	APIKeyName     *string        `json:"api_key_name"`
	IP             string         `json:"ip"`
	ErrorName      string         `json:"error_name"`
	ErrorMessage   string         `json:"error_message"`
	CreatedAt      time.Time      `json:"created_at"`
	RequestBody    *string        `json:"request_body"`
	ResponseBody   *string        `json:"response_body"`
	RequestHeaders map[string]any `json:"request_headers"`
}

// ListLogsParams filters the request log; zero fields are not sent.
type ListLogsParams struct {
	ListParams
	Status    string
	Method    string
	Source    string // api or dashboard
	UserAgent string
	APIKeyID  string
	Query     string
	Days      int
}

// LogsService is client.Logs (the API request log, newest first).
type LogsService struct{ c *Client }

// List returns a page of logged requests.
func (s *LogsService) List(ctx context.Context, p *ListLogsParams) (*List[LogEntry], error) {
	v := url.Values{}
	if p != nil {
		v = p.ListParams.values()
		for k, x := range map[string]string{"status": p.Status, "method": p.Method, "source": p.Source, "user_agent": p.UserAgent, "api_key_id": p.APIKeyID, "q": p.Query} {
			if x != "" {
				v.Set(k, x)
			}
		}
		if p.Days > 0 {
			v.Set("days", strconv.Itoa(p.Days))
		}
	}
	return do[List[LogEntry]](ctx, s.c, http.MethodGet, withQuery("/logs", v), nil)
}

// Get returns one logged request with its bodies.
func (s *LogsService) Get(ctx context.Context, id string) (*LogEntry, error) {
	return do[LogEntry](ctx, s.c, http.MethodGet, "/logs/"+esc(id), nil)
}
