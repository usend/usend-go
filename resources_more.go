package usend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"
)

// do is the typed shortcut used by the services in this file.
func do[T any](ctx context.Context, c *Client, method, path string, body any, opts ...RequestOption) (*T, error) {
	var out T
	if err := c.Request(ctx, method, path, body, &out, opts...); err != nil {
		return nil, err
	}
	return &out, nil
}

func (p *ListParams) add(v url.Values) url.Values {
	for k, vals := range p.values() {
		v[k] = vals
	}
	return v
}

func setIf(v url.Values, k, val string) {
	if val != "" {
		v.Set(k, val)
	}
}

// ---------------------------------------------------- emails: more calls

// EmailMetricsParams filters GET /emails/metrics. List values are sent comma separated.
type EmailMetricsParams struct {
	StartDate    string // ISO 8601 date or datetime (default: last 7 days)
	EndDate      string
	Timezone     string // IANA name, default UTC
	Granularity  string // hourly, daily, weekly, monthly
	Metrics      []string
	Dimensions   []string // period, domain, email, broadcast
	DomainIDs    []string
	EmailIDs     []string
	BroadcastIDs []string
}

// EmailMetrics is the response of GET /emails/metrics. Data is set only when
// dimensions were requested; decode it into your own rows.
type EmailMetrics struct {
	Object      string   `json:"object"`
	StartDate   string   `json:"start_date"`
	EndDate     string   `json:"end_date"`
	Metrics     []string `json:"metrics"`
	Dimensions  []string `json:"dimensions"`
	Granularity string   `json:"granularity"`
	Totals      struct {
		Sent            int `json:"sent"`
		Delivered       int `json:"delivered"`
		DeliveryDelayed int `json:"delivery_delayed"`
		Bounced         int `json:"bounced"`
		Complained      int `json:"complained"`
		Opened          int `json:"opened"`
		Clicked         int `json:"clicked"`
		Failed          int `json:"failed"`
	} `json:"totals"`
	Data []map[string]any `json:"data,omitempty"`
}

func join(s []string) string {
	out := ""
	for i, x := range s {
		if i > 0 {
			out += ","
		}
		out += x
	}
	return out
}

// Metrics returns aggregated email counts (full-access key).
func (s *EmailsService) Metrics(ctx context.Context, p *EmailMetricsParams) (*EmailMetrics, error) {
	v := url.Values{}
	if p != nil {
		setIf(v, "start_date", p.StartDate)
		setIf(v, "end_date", p.EndDate)
		setIf(v, "timezone", p.Timezone)
		setIf(v, "granularity", p.Granularity)
		setIf(v, "metrics", join(p.Metrics))
		setIf(v, "dimensions", join(p.Dimensions))
		setIf(v, "domain_id", join(p.DomainIDs))
		setIf(v, "email_id", join(p.EmailIDs))
		setIf(v, "broadcast_id", join(p.BroadcastIDs))
	}
	return do[EmailMetrics](ctx, s.c, http.MethodGet, withQuery("/emails/metrics", v), nil)
}

// ShareResult is the response of Share.
type ShareResult struct {
	Object string `json:"object"`
	ID     string `json:"id"`
	URL    string `json:"url"` // public, read-only, expiring
}

// Share creates a public read-only link to an email. expiresIn is "10m",
// "2 hours", "1 day"... ("" = 48 hours, also the maximum).
func (s *EmailsService) Share(ctx context.Context, id, expiresIn string) (*ShareResult, error) {
	body := map[string]string{}
	if expiresIn != "" {
		body["expires_in"] = expiresIn
	}
	return do[ShareResult](ctx, s.c, http.MethodPost, "/emails/"+esc(id)+"/share", body)
}

// TestEmailParams is the body of POST /emails/test.
type TestEmailParams struct {
	From    string `json:"from"`
	Subject string `json:"subject,omitempty"`
	HTML    string `json:"html,omitempty"`
	Text    string `json:"text,omitempty"`
}

// SendTest sends the content to the signed-in user's own address, with "[Test] "
// in the subject. Dashboard session only: API keys get 403.
func (s *EmailsService) SendTest(ctx context.Context, p *TestEmailParams) (*struct {
	ID string `json:"id"`
	To string `json:"to"`
}, error) {
	return do[struct {
		ID string `json:"id"`
		To string `json:"to"`
	}](ctx, s.c, http.MethodPost, "/emails/test", p)
}

// Insight is one deliverability finding.
type Insight struct {
	ID       string `json:"id"`
	Severity string `json:"severity"` // critical, warning, info, pass
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	Fix      string `json:"fix"`
}

// Insights is the response of email and domain insights.
type Insights struct {
	Object   string `json:"object"`
	EmailID  string `json:"email_id,omitempty"`
	DomainID string `json:"domain_id,omitempty"`
	Summary  struct {
		Critical int `json:"critical"`
		Warning  int `json:"warning"`
		Info     int `json:"info"`
		Pass     int `json:"pass"`
	} `json:"summary"`
	Insights []Insight `json:"insights"`
}

// Insights returns deliverability findings for a stored email.
func (s *EmailsService) Insights(ctx context.Context, id string) (*Insights, error) {
	return do[Insights](ctx, s.c, http.MethodGet, "/emails/"+esc(id)+"/insights", nil)
}

// -------------------------------------------------------- domains: more

// DomainClaim is the response of the domain claim calls.
type DomainClaim struct {
	Object   string `json:"object"`
	ID       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"` // pending, verified, failed, expired
	DomainID string `json:"domain_id"`
	Region   string `json:"region"`
	Record   struct {
		Type  string `json:"type"`
		Name  string `json:"name"` // _usend-claim.<name>
		Value string `json:"value"`
		TTL   any    `json:"ttl"`
	} `json:"record"`
	BlockedReason string    `json:"blocked_reason"`
	FailureReason string    `json:"failure_reason"`
	CreatedAt     time.Time `json:"created_at"`
	ExpiresAt     time.Time `json:"expires_at"`
}

// Claim starts a claim on a name another organization already verified (used
// after Create answered 409). Publish the TXT record in Record, then VerifyClaim.
func (s *DomainsService) Claim(ctx context.Context, p *CreateDomainParams) (*DomainClaim, error) {
	return do[DomainClaim](ctx, s.c, http.MethodPost, "/domains/claim", p)
}

// GetClaim returns a claim.
func (s *DomainsService) GetClaim(ctx context.Context, id string) (*DomainClaim, error) {
	return do[DomainClaim](ctx, s.c, http.MethodGet, "/domains/claim/"+esc(id), nil)
}

// VerifyClaim checks the TXT record of a claim.
func (s *DomainsService) VerifyClaim(ctx context.Context, id string) (*DomainClaim, error) {
	return do[DomainClaim](ctx, s.c, http.MethodPost, "/domains/claim/"+esc(id)+"/verify", nil)
}

// Insights returns deliverability findings for a domain.
func (s *DomainsService) Insights(ctx context.Context, id string) (*Insights, error) {
	return do[Insights](ctx, s.c, http.MethodGet, "/domains/"+esc(id)+"/insights", nil)
}

// DMARCCheck resolves a domain's DMARC, SPF, DKIM, BIMI, MTA-STS and TLS-RPT
// records. The result is returned as generic JSON.
func (c *Client) DMARCCheck(ctx context.Context, domain string) (map[string]any, error) {
	out, err := do[map[string]any](ctx, c, http.MethodPost, "/tools/dmarc-check", map[string]string{"domain": domain})
	if err != nil {
		return nil, err
	}
	return *out, nil
}

// UpdateAPIKeyParams is the body of PATCH /api-keys/{id}. To clear a key's
// domain scope send {"domain_id": null} with Request.
type UpdateAPIKeyParams struct {
	Name       string `json:"name,omitempty"`
	Permission string `json:"permission,omitempty"`
	DomainID   string `json:"domain_id,omitempty"`
}

// Update renames a key or changes its permission or domain.
func (s *APIKeysService) Update(ctx context.Context, id string, p *UpdateAPIKeyParams) (*IDResult, error) {
	return do[IDResult](ctx, s.c, http.MethodPatch, "/api-keys/"+esc(id), p)
}

// ----------------------------------------------------- templates: versions

// TemplateVersion is one published version of a template. The content fields
// are only set by GetVersion.
type TemplateVersion struct {
	Object     string             `json:"object"`
	ID         string             `json:"id"`
	TemplateID string             `json:"template_id"`
	Version    int                `json:"version"`
	Name       string             `json:"name"`
	Current    bool               `json:"current"` // identical to what sends use
	CreatedBy  string             `json:"created_by"`
	CreatedAt  time.Time          `json:"created_at"`
	From       string             `json:"from,omitempty"`
	Subject    string             `json:"subject,omitempty"`
	ReplyTo    []string           `json:"reply_to,omitempty"`
	HTML       string             `json:"html,omitempty"`
	Text       string             `json:"text,omitempty"`
	Variables  []TemplateVariable `json:"variables,omitempty"`
}

// Versions lists a template's published versions, newest first.
func (s *TemplatesService) Versions(ctx context.Context, idOrAlias string) (*List[TemplateVersion], error) {
	return do[List[TemplateVersion]](ctx, s.c, http.MethodGet, "/templates/"+esc(idOrAlias)+"/versions", nil)
}

// GetVersion returns one version with its content.
func (s *TemplatesService) GetVersion(ctx context.Context, idOrAlias, versionID string) (*TemplateVersion, error) {
	return do[TemplateVersion](ctx, s.c, http.MethodGet, "/templates/"+esc(idOrAlias)+"/versions/"+esc(versionID), nil)
}

// RestoreVersion copies a version into the draft (it does not publish).
func (s *TemplatesService) RestoreVersion(ctx context.Context, idOrAlias, versionID string) (*IDResult, error) {
	return do[IDResult](ctx, s.c, http.MethodPost, "/templates/"+esc(idOrAlias)+"/versions/"+esc(versionID)+"/restore", nil)
}

// ----------------------------------------------- segments and contacts: more

// Update renames a segment.
func (s *SegmentsService) Update(ctx context.Context, id, name string) (*Segment, error) {
	return do[Segment](ctx, s.c, http.MethodPatch, "/segments/"+esc(id), map[string]string{"name": name})
}

// Contacts returns a page of a segment's contacts. subscription is "", "subscribed" or "unsubscribed".
func (s *SegmentsService) Contacts(ctx context.Context, id, subscription string, p *ListParams) (*List[Contact], error) {
	v := p.add(url.Values{})
	setIf(v, "subscription", subscription)
	return do[List[Contact]](ctx, s.c, http.MethodGet, withQuery("/segments/"+esc(id)+"/contacts", v), nil)
}

// SegmentMetrics is the response of Metrics.
type SegmentMetrics struct {
	Object       string `json:"object"`
	ID           string `json:"id"`
	Contacts     int    `json:"contacts"`
	Subscribed   int    `json:"subscribed"`
	Unsubscribed int    `json:"unsubscribed"`
	Sends        int    `json:"sends"`
	Delivered    int    `json:"delivered"`
	Bounced      int    `json:"bounced"`
	Opens        int    `json:"opens"`
	Clicks       int    `json:"clicks"`
}

// Metrics returns counts for a segment.
func (s *SegmentsService) Metrics(ctx context.Context, id string) (*SegmentMetrics, error) {
	return do[SegmentMetrics](ctx, s.c, http.MethodGet, "/segments/"+esc(id)+"/metrics", nil)
}

// ContactImport is a background contact import.
type ContactImport struct {
	Object    string `json:"object"`
	ID        string `json:"id"`
	SegmentID string `json:"segment_id"`
	Status    string `json:"status"` // pending, processing, completed, failed
	Imported  int    `json:"imported"`
	Updated   int    `json:"updated"`
	Skipped   int    `json:"skipped"`
	Errors    []struct {
		Row     int    `json:"row"`
		Email   string `json:"email,omitempty"`
		Message string `json:"message"`
	} `json:"errors"`
	IgnoredColumns []string   `json:"ignored_columns"`
	Error          string     `json:"error"`
	CreatedAt      time.Time  `json:"created_at"`
	StartedAt      *time.Time `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at"`
}

// CreateContactImportParams is the body of POST /contacts/imports: send either CSV or Contacts.
type CreateContactImportParams struct {
	SegmentID string                `json:"segment_id,omitempty"`
	CSV       string                `json:"csv,omitempty"`
	Contacts  []CreateContactParams `json:"contacts,omitempty"`
}

// CreateImport starts a background import (up to 250,000 rows); poll GetImport.
func (s *ContactsService) CreateImport(ctx context.Context, p *CreateContactImportParams) (*ContactImport, error) {
	return do[ContactImport](ctx, s.c, http.MethodPost, "/contacts/imports", p)
}

// GetImport polls one import.
func (s *ContactsService) GetImport(ctx context.Context, id string) (*ContactImport, error) {
	return do[ContactImport](ctx, s.c, http.MethodGet, "/contacts/imports/"+esc(id), nil)
}

// ListImports returns a page of imports; status filters ("" = all).
func (s *ContactsService) ListImports(ctx context.Context, status string, p *ListParams) (*List[ContactImport], error) {
	v := p.add(url.Values{})
	setIf(v, "status", status)
	return do[List[ContactImport]](ctx, s.c, http.MethodGet, withQuery("/contacts/imports", v), nil)
}

// Get returns one contact property.
func (s *ContactPropertiesService) Get(ctx context.Context, id string) (*ContactProperty, error) {
	return do[ContactProperty](ctx, s.c, http.MethodGet, "/contact-properties/"+esc(id), nil)
}

// ------------------------------------------------------ broadcasts: more

// Recipient types for Broadcasts.Recipients.
const (
	RecipientSent         = "sent"
	RecipientDelivered    = "delivered"
	RecipientOpened       = "opened"
	RecipientClicked      = "clicked"
	RecipientBounced      = "bounced"
	RecipientComplained   = "complained"
	RecipientUnsubscribed = "unsubscribed"
	RecipientSuppressed   = "suppressed"
)

// BroadcastRecipientsParams filters Broadcasts.Recipients.
type BroadcastRecipientsParams struct {
	ListParams
	Email      string // substring match
	BounceType string // with Type "bounced": permanent, transient, undetermined
}

// ClickedLink is a link clicked in a recipient's email.
type ClickedLink struct {
	URL    string `json:"url"`
	Clicks int    `json:"clicks"`
}

// BroadcastRecipient is a recipient who had the requested event. ID is the email's ID.
type BroadcastRecipient struct {
	ID           string        `json:"id"`
	ContactID    string        `json:"contact_id"`
	Email        string        `json:"email"`
	Count        int           `json:"count,omitempty"`       // opened, clicked
	BounceType   string        `json:"bounce_type,omitempty"` // bounced
	ClickedLinks []ClickedLink `json:"clicked_links,omitempty"`
}

// Recipients lists the recipients of a broadcast who had an event. typ is
// required: one of the Recipient* constants.
func (s *BroadcastsService) Recipients(ctx context.Context, id, typ string, p *BroadcastRecipientsParams) (*List[BroadcastRecipient], error) {
	v := url.Values{}
	if p != nil {
		v = p.ListParams.add(v)
		setIf(v, "email", p.Email)
		setIf(v, "bounce_type", p.BounceType)
	}
	v.Set("type", typ)
	return do[List[BroadcastRecipient]](ctx, s.c, http.MethodGet, withQuery("/broadcasts/"+esc(id)+"/recipients", v), nil)
}

// BroadcastClickedLink is a link with its click counts.
type BroadcastClickedLink struct {
	Object       string `json:"object"`
	Link         string `json:"link"`
	Clicks       int    `json:"clicks"`
	UniqueClicks int    `json:"unique_clicks"`
}

// ClickedLinks lists the links clicked in a broadcast, most clicked first.
func (s *BroadcastsService) ClickedLinks(ctx context.Context, id string) (*List[BroadcastClickedLink], error) {
	return do[List[BroadcastClickedLink]](ctx, s.c, http.MethodGet, "/broadcasts/"+esc(id)+"/clicked-links", nil)
}

// BroadcastPerformance is the response of Performance. Rates are percentages.
type BroadcastPerformance struct {
	Object      string             `json:"object"`
	BroadcastID string             `json:"broadcast_id"`
	Counts      map[string]int     `json:"counts"`
	Rates       map[string]float64 `json:"rates"`
	Timeline    []struct {
		Hour    string `json:"hour"`
		Opened  int    `json:"opened"`
		Clicked int    `json:"clicked"`
	} `json:"timeline"`
}

// Performance returns counts, rates and an hourly open/click timeline.
func (s *BroadcastsService) Performance(ctx context.Context, id string) (*BroadcastPerformance, error) {
	return do[BroadcastPerformance](ctx, s.c, http.MethodGet, "/broadcasts/"+esc(id)+"/performance", nil)
}

// Cancel stops a queued, sending or paused broadcast.
func (s *BroadcastsService) Cancel(ctx context.Context, id string) error {
	return s.c.Request(ctx, http.MethodPost, "/broadcasts/"+esc(id)+"/cancel", nil, nil)
}

// Duplicate copies a broadcast into a new draft.
func (s *BroadcastsService) Duplicate(ctx context.Context, id string) (*IDResult, error) {
	return do[IDResult](ctx, s.c, http.MethodPost, "/broadcasts/"+esc(id)+"/duplicate", nil)
}

// -------------------------------------------------------- webhooks: more

// RotateSecret issues a new signing secret; the old one stops signing at once.
func (s *WebhooksService) RotateSecret(ctx context.Context, id string) (*Webhook, error) {
	return do[Webhook](ctx, s.c, http.MethodPost, "/webhooks/"+esc(id)+"/rotate-secret", nil)
}

// WebhookEventRecord is one delivery of an event to one endpoint. Payload is set by GetEvent.
type WebhookEventRecord struct {
	Object         string          `json:"object"`
	ID             string          `json:"id"`
	Type           string          `json:"type"`
	WebhookID      string          `json:"webhook_id"`
	Endpoint       string          `json:"endpoint"`
	Status         string          `json:"status"` // pending, succeeded, failed
	Attempts       int             `json:"attempts"`
	ResponseStatus *int            `json:"response_status"`
	LastError      *string         `json:"last_error"`
	CreatedAt      time.Time       `json:"created_at"`
	DeliveredAt    *time.Time      `json:"delivered_at"`
	Payload        json.RawMessage `json:"payload,omitempty"`
}

// WebhookEventsParams filters Webhooks.Events.
type WebhookEventsParams struct {
	ListParams
	WebhookID string
	Type      string
	Status    string // pending, succeeded, failed
}

// WebhookAttempt is one delivery attempt.
type WebhookAttempt struct {
	Object         string    `json:"object"`
	ID             string    `json:"id"`
	Attempt        int       `json:"attempt"`
	Status         string    `json:"status"`
	ResponseStatus *int      `json:"response_status"`
	Error          string    `json:"error"`
	CreatedAt      time.Time `json:"created_at"`
}

// Events lists deliveries across all endpoints.
func (s *WebhooksService) Events(ctx context.Context, p *WebhookEventsParams) (*List[WebhookEventRecord], error) {
	v := url.Values{}
	if p != nil {
		v = p.ListParams.add(v)
		setIf(v, "webhook_id", p.WebhookID)
		setIf(v, "type", p.Type)
		setIf(v, "status", p.Status)
	}
	return do[List[WebhookEventRecord]](ctx, s.c, http.MethodGet, withQuery("/webhooks/events", v), nil)
}

// GetEvent returns one delivery with the payload that is POSTed.
func (s *WebhooksService) GetEvent(ctx context.Context, id string) (*WebhookEventRecord, error) {
	return do[WebhookEventRecord](ctx, s.c, http.MethodGet, "/webhooks/events/"+esc(id), nil)
}

// ReplayEvent sends a delivery again.
func (s *WebhooksService) ReplayEvent(ctx context.Context, id string) (*IDResult, error) {
	return do[IDResult](ctx, s.c, http.MethodPost, "/webhooks/events/"+esc(id)+"/replay", nil)
}

// EventAttempts lists the attempts of a delivery.
func (s *WebhooksService) EventAttempts(ctx context.Context, id string) (*List[WebhookAttempt], error) {
	return do[List[WebhookAttempt]](ctx, s.c, http.MethodGet, "/webhooks/events/"+esc(id)+"/attempts", nil)
}

// ---------------------------------------------------- suppressions: more

// Get returns a suppression by ID or email address.
func (s *SuppressionsService) Get(ctx context.Context, idOrEmail string) (*Suppression, error) {
	return do[Suppression](ctx, s.c, http.MethodGet, "/suppressions/"+esc(idOrEmail), nil)
}

// BatchCreate suppresses up to 100 addresses.
func (s *SuppressionsService) BatchCreate(ctx context.Context, emails []string) (*List[Suppression], error) {
	return do[List[Suppression]](ctx, s.c, http.MethodPost, "/suppressions/batch", map[string]any{"emails": emails})
}

// BatchRemove lifts suppressions by address; unknown addresses are skipped.
func (s *SuppressionsService) BatchRemove(ctx context.Context, emails []string) (*List[DeleteResult], error) {
	return do[List[DeleteResult]](ctx, s.c, http.MethodDelete, "/suppressions/batch", map[string]any{"emails": emails})
}

// ---------------------------------------------------- custom events

// CustomEvent is a named event (for example "order.paid") that automations can start on.
type CustomEvent struct {
	Object    string            `json:"object"`
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Schema    map[string]string `json:"schema"` // key -> string, number, boolean, date
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// CreateEventParams is the body of POST /events.
type CreateEventParams struct {
	Name   string            `json:"name"`
	Schema map[string]string `json:"schema,omitempty"`
}

// UpdateEventParams is the body of PATCH /events/{id}. Use Request with
// {"schema": null} to clear a schema.
type UpdateEventParams struct {
	Name   string            `json:"name,omitempty"`
	Schema map[string]string `json:"schema,omitempty"`
}

// SendEventParams is the body of POST /events/send. Give ContactID or Email.
type SendEventParams struct {
	Event     string         `json:"event"`
	ContactID string         `json:"contact_id,omitempty"`
	Email     string         `json:"email,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
}

// SendEventResult is the 202 response of Send.
type SendEventResult struct {
	Object string `json:"object"`
	Event  string `json:"event"`
	ID     string `json:"id"`
}

// EventsService is client.Events (custom events; email events are client.Emails.Events).
type EventsService struct{ c *Client }

// Create defines an event.
func (s *EventsService) Create(ctx context.Context, p *CreateEventParams) (*IDResult, error) {
	return do[IDResult](ctx, s.c, http.MethodPost, "/events", p)
}

// List returns a page of events.
func (s *EventsService) List(ctx context.Context, p *ListParams) (*List[CustomEvent], error) {
	return do[List[CustomEvent]](ctx, s.c, http.MethodGet, withQuery("/events", p.values()), nil)
}

// Get returns an event by ID or name.
func (s *EventsService) Get(ctx context.Context, idOrName string) (*CustomEvent, error) {
	return do[CustomEvent](ctx, s.c, http.MethodGet, "/events/"+esc(idOrName), nil)
}

// Update edits an event (one used by an automation cannot be renamed).
func (s *EventsService) Update(ctx context.Context, id string, p *UpdateEventParams) (*CustomEvent, error) {
	return do[CustomEvent](ctx, s.c, http.MethodPatch, "/events/"+esc(id), p)
}

// Remove deletes an event (409 while an automation uses it).
func (s *EventsService) Remove(ctx context.Context, id string) (*DeleteResult, error) {
	return do[DeleteResult](ctx, s.c, http.MethodDelete, "/events/"+esc(id), nil)
}

// Send fires an event for a contact. The API answers 202; with an idempotency
// key a retry returns the first occurrence instead of firing again.
func (s *EventsService) Send(ctx context.Context, p *SendEventParams, opts ...RequestOption) (*SendEventResult, error) {
	return do[SendEventResult](ctx, s.c, http.MethodPost, "/events/send", p, opts...)
}

// ---------------------------------------------------------- automations

// AutomationStep is one node of an automation graph. Config depends on Type
// (trigger, delay, condition, wait_for_event, send_email, add_to_segment,
// contact_update, contact_delete); see the API reference.
type AutomationStep struct {
	Key    string         `json:"key"`
	Type   string         `json:"type"`
	Config map[string]any `json:"config,omitempty"`
}

// AutomationConnection links two steps. Type defaults to "default".
type AutomationConnection struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type,omitempty"`
}

// Automation is a workflow started by an event.
type Automation struct {
	Object       string                 `json:"object"`
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Status       string                 `json:"status"` // draft, enabled, disabled
	TriggerEvent string                 `json:"trigger_event"`
	Version      int                    `json:"version"`
	Steps        []AutomationStep       `json:"steps,omitempty"`
	Connections  []AutomationConnection `json:"connections,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// AutomationParams is the body of create and update (all optional on update).
type AutomationParams struct {
	Name        string                 `json:"name,omitempty"`
	Status      string                 `json:"status,omitempty"`
	Steps       []AutomationStep       `json:"steps,omitempty"`
	Connections []AutomationConnection `json:"connections,omitempty"`
}

// AutomationRunStep is the outcome of one step in a run.
type AutomationRunStep struct {
	Key         string         `json:"key"`
	Type        string         `json:"type"`
	Status      string         `json:"status"` // completed, skipped, waiting, failed, cancelled
	Branch      string         `json:"branch"`
	Output      map[string]any `json:"output"`
	StartedAt   *time.Time     `json:"started_at"`
	CompletedAt *time.Time     `json:"completed_at"`
}

// AutomationRun is one execution for one contact. Steps is set by GetRun.
type AutomationRun struct {
	Object       string              `json:"object"`
	ID           string              `json:"id"`
	AutomationID string              `json:"automation_id"`
	Version      int                 `json:"version"`
	Status       string              `json:"status"` // running, completed, failed, cancelled
	Email        string              `json:"email"`
	ContactID    string              `json:"contact_id"`
	EventID      string              `json:"event_id"`
	CurrentStep  string              `json:"current_step"`
	Error        string              `json:"error"`
	StartedAt    time.Time           `json:"started_at"`
	CompletedAt  *time.Time          `json:"completed_at"`
	Steps        []AutomationRunStep `json:"steps,omitempty"`
}

// AutomationsService is client.Automations.
type AutomationsService struct{ c *Client }

// Create makes an automation.
func (s *AutomationsService) Create(ctx context.Context, p *AutomationParams) (*IDResult, error) {
	return do[IDResult](ctx, s.c, http.MethodPost, "/automations", p)
}

// List returns a page of automations; status filters ("" = all).
func (s *AutomationsService) List(ctx context.Context, status string, p *ListParams) (*List[Automation], error) {
	v := p.add(url.Values{})
	setIf(v, "status", status)
	return do[List[Automation]](ctx, s.c, http.MethodGet, withQuery("/automations", v), nil)
}

// Get returns an automation with its steps and connections.
func (s *AutomationsService) Get(ctx context.Context, id string) (*Automation, error) {
	return do[Automation](ctx, s.c, http.MethodGet, "/automations/"+esc(id), nil)
}

// Update edits an automation; a change to the graph raises Version.
func (s *AutomationsService) Update(ctx context.Context, id string, p *AutomationParams) (*IDResult, error) {
	return do[IDResult](ctx, s.c, http.MethodPatch, "/automations/"+esc(id), p)
}

// Remove deletes an automation.
func (s *AutomationsService) Remove(ctx context.Context, id string) error {
	return s.c.Request(ctx, http.MethodDelete, "/automations/"+esc(id), nil, nil)
}

// Duplicate makes a draft copy.
func (s *AutomationsService) Duplicate(ctx context.Context, id string) (*IDResult, error) {
	return do[IDResult](ctx, s.c, http.MethodPost, "/automations/"+esc(id)+"/duplicate", nil)
}

// StopResult is the response of Stop.
type StopResult struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	CancelledRuns int    `json:"cancelled_runs"`
}

// Stop halts every active run. Update the status to "enabled" or "disabled" to bring it back.
func (s *AutomationsService) Stop(ctx context.Context, id string) (*StopResult, error) {
	return do[StopResult](ctx, s.c, http.MethodPost, "/automations/"+esc(id)+"/stop", nil)
}

// Runs lists runs; status is running, completed, failed, cancelled or "".
func (s *AutomationsService) Runs(ctx context.Context, id, status string, p *ListParams) (*List[AutomationRun], error) {
	v := p.add(url.Values{})
	setIf(v, "status", status)
	return do[List[AutomationRun]](ctx, s.c, http.MethodGet, withQuery("/automations/"+esc(id)+"/runs", v), nil)
}

// GetRun returns a run with its steps.
func (s *AutomationsService) GetRun(ctx context.Context, id, runID string) (*AutomationRun, error) {
	return do[AutomationRun](ctx, s.c, http.MethodGet, "/automations/"+esc(id)+"/runs/"+esc(runID), nil)
}

// ------------------------------------------------------------- inboxes

// Inbox is a shared mailbox on a receiving domain.
type Inbox struct {
	Object    string    `json:"object"`
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateInboxParams is the body of POST /inboxes.
type CreateInboxParams struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Enabled *bool  `json:"enabled,omitempty"`
}

// UpdateInboxParams is the body of PATCH /inboxes/{id}.
type UpdateInboxParams struct {
	Name    string `json:"name,omitempty"`
	Enabled *bool  `json:"enabled,omitempty"`
}

// Label tags threads inside an inbox.
type Label struct {
	Object    string    `json:"object"`
	ID        string    `json:"id"`
	InboxID   string    `json:"inbox_id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
}

// LabelParams is the body of label create and update.
type LabelParams struct {
	Name  string `json:"name,omitempty"`
	Color string `json:"color,omitempty"`
}

// Thread is a conversation in an inbox.
type Thread struct {
	Object         string  `json:"object"`
	ID             string  `json:"id"`
	InboxID        string  `json:"inbox_id"`
	Subject        string  `json:"subject"`
	Folder         string  `json:"folder"` // inbox, archive, spam, trash
	AssigneeUserID *string `json:"assignee_user_id"`
	Labels         []struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Color string `json:"color"`
	} `json:"labels"`
	Snippet       string    `json:"snippet"`
	MessageCount  int       `json:"message_count"`
	UnreadCount   int       `json:"unread_count"`
	LastMessageAt time.Time `json:"last_message_at"`
	CreatedAt     time.Time `json:"created_at"`
}

// ThreadListParams filters Inboxes.Threads.
type ThreadListParams struct {
	ListParams
	Folder   string // inbox, archive, spam, trash
	Label    string // label ID
	Assignee string // user ID or "none"
	Unread   bool
	Q        string
}

// UpdateThreadParams is the body of PATCH on a thread. Labels replaces the set
// when non-nil; Read marks every received message read or unread.
type UpdateThreadParams struct {
	Folder         string   `json:"folder,omitempty"`
	Labels         []string `json:"labels,omitempty"`
	AssigneeUserID *string  `json:"assignee_user_id,omitempty"`
	Read           *bool    `json:"read,omitempty"`
}

// InboxEmail is one message of a thread. The content fields are set by GetEmail.
type InboxEmail struct {
	Object         string              `json:"object"`
	ID             string              `json:"id"`
	ThreadID       string              `json:"thread_id"`
	Direction      string              `json:"direction"` // inbound, outbound
	From           string              `json:"from"`
	To             []string            `json:"to"`
	Cc             []string            `json:"cc"`
	Subject        string              `json:"subject"`
	Snippet        string              `json:"snippet"`
	Read           bool                `json:"read"`
	MessageID      string              `json:"message_id"`
	InReplyTo      string              `json:"in_reply_to"`
	InboundEmailID *string             `json:"inbound_email_id"`
	EmailID        *string             `json:"email_id"`
	CreatedAt      time.Time           `json:"created_at"`
	ReplyTo        []string            `json:"reply_to,omitempty"`
	HTML           string              `json:"html,omitempty"`
	Text           string              `json:"text,omitempty"`
	Attachments    []InboundAttachment `json:"attachments,omitempty"`
}

// ReplyParams is the body of reply. With no To, the answer goes to Reply-To (else the sender), or with ReplyAll to everyone but the inbox.
type ReplyParams struct {
	Text     string   `json:"text,omitempty"`
	HTML     string   `json:"html,omitempty"`
	To       []string `json:"to,omitempty"`
	Cc       []string `json:"cc,omitempty"`
	Bcc      []string `json:"bcc,omitempty"`
	Subject  string   `json:"subject,omitempty"`
	ReplyAll bool     `json:"reply_all,omitempty"`
}

// ForwardParams is the body of forward.
type ForwardParams struct {
	To      []string `json:"to"`
	Text    string   `json:"text,omitempty"`
	HTML    string   `json:"html,omitempty"`
	Cc      []string `json:"cc,omitempty"`
	Bcc     []string `json:"bcc,omitempty"`
	Subject string   `json:"subject,omitempty"`
}

// InboxSendResult is the response of reply, forward and draft send.
type InboxSendResult struct {
	Object   string `json:"object"`
	ID       string `json:"id"`
	EmailID  string `json:"email_id"` // the sent email in /emails
	ThreadID string `json:"thread_id"`
}

// Draft is an unsent message in an inbox.
type Draft struct {
	Object         string    `json:"object"`
	ID             string    `json:"id"`
	InboxID        string    `json:"inbox_id"`
	Kind           string    `json:"kind"` // new, reply, forward
	ThreadID       *string   `json:"thread_id"`
	ReplyToEmailID *string   `json:"reply_to_email_id"`
	ForwardEmailID *string   `json:"forward_email_id"`
	ReplyAll       bool      `json:"reply_all"`
	To             []string  `json:"to"`
	Cc             []string  `json:"cc"`
	Bcc            []string  `json:"bcc"`
	Subject        string    `json:"subject"`
	HTML           string    `json:"html"`
	Text           string    `json:"text"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// DraftParams is the body of draft create and update.
type DraftParams struct {
	To             []string `json:"to,omitempty"`
	Cc             []string `json:"cc,omitempty"`
	Bcc            []string `json:"bcc,omitempty"`
	Subject        string   `json:"subject,omitempty"`
	HTML           string   `json:"html,omitempty"`
	Text           string   `json:"text,omitempty"`
	ReplyToEmailID string   `json:"reply_to_email_id,omitempty"`
	ForwardEmailID string   `json:"forward_email_id,omitempty"`
	ReplyAll       bool     `json:"reply_all,omitempty"`
}

// InboxesService is client.Inboxes (full-access keys and sessions only).
type InboxesService struct{ c *Client }

func ib(id string) string { return "/inboxes/" + esc(id) }

// Create makes an inbox on a verified, receiving-enabled domain.
func (s *InboxesService) Create(ctx context.Context, p *CreateInboxParams) (*Inbox, error) {
	return do[Inbox](ctx, s.c, http.MethodPost, "/inboxes", p)
}

// List returns a page of inboxes.
func (s *InboxesService) List(ctx context.Context, p *ListParams) (*List[Inbox], error) {
	return do[List[Inbox]](ctx, s.c, http.MethodGet, withQuery("/inboxes", p.values()), nil)
}

// Get returns one inbox.
func (s *InboxesService) Get(ctx context.Context, id string) (*Inbox, error) {
	return do[Inbox](ctx, s.c, http.MethodGet, ib(id), nil)
}

// Update renames or enables/disables an inbox.
func (s *InboxesService) Update(ctx context.Context, id string, p *UpdateInboxParams) (*Inbox, error) {
	return do[Inbox](ctx, s.c, http.MethodPatch, ib(id), p)
}

// Remove deletes an inbox.
func (s *InboxesService) Remove(ctx context.Context, id string) (*DeleteResult, error) {
	return do[DeleteResult](ctx, s.c, http.MethodDelete, ib(id), nil)
}

// CreateLabel adds a label (at most 50 per inbox).
func (s *InboxesService) CreateLabel(ctx context.Context, inboxID string, p *LabelParams) (*Label, error) {
	return do[Label](ctx, s.c, http.MethodPost, ib(inboxID)+"/labels", p)
}

// Labels lists an inbox's labels.
func (s *InboxesService) Labels(ctx context.Context, inboxID string) (*List[Label], error) {
	return do[List[Label]](ctx, s.c, http.MethodGet, ib(inboxID)+"/labels", nil)
}

// UpdateLabel edits a label.
func (s *InboxesService) UpdateLabel(ctx context.Context, inboxID, labelID string, p *LabelParams) (*Label, error) {
	return do[Label](ctx, s.c, http.MethodPatch, ib(inboxID)+"/labels/"+esc(labelID), p)
}

// RemoveLabel deletes a label.
func (s *InboxesService) RemoveLabel(ctx context.Context, inboxID, labelID string) error {
	return s.c.Request(ctx, http.MethodDelete, ib(inboxID)+"/labels/"+esc(labelID), nil, nil)
}

// Threads lists threads, newest activity first.
func (s *InboxesService) Threads(ctx context.Context, inboxID string, p *ThreadListParams) (*List[Thread], error) {
	v := url.Values{}
	if p != nil {
		v = p.ListParams.add(v)
		setIf(v, "folder", p.Folder)
		setIf(v, "label", p.Label)
		setIf(v, "assignee", p.Assignee)
		setIf(v, "q", p.Q)
		if p.Unread {
			v.Set("unread", "true")
		}
	}
	return do[List[Thread]](ctx, s.c, http.MethodGet, withQuery(ib(inboxID)+"/threads", v), nil)
}

// Thread returns one thread.
func (s *InboxesService) Thread(ctx context.Context, inboxID, threadID string) (*Thread, error) {
	return do[Thread](ctx, s.c, http.MethodGet, ib(inboxID)+"/threads/"+esc(threadID), nil)
}

// UpdateThread moves, labels, assigns or marks a thread read.
func (s *InboxesService) UpdateThread(ctx context.Context, inboxID, threadID string, p *UpdateThreadParams) (*Thread, error) {
	return do[Thread](ctx, s.c, http.MethodPatch, ib(inboxID)+"/threads/"+esc(threadID), p)
}

// RemoveThread deletes a thread permanently.
func (s *InboxesService) RemoveThread(ctx context.Context, inboxID, threadID string) error {
	return s.c.Request(ctx, http.MethodDelete, ib(inboxID)+"/threads/"+esc(threadID), nil, nil)
}

// Emails lists the messages of a thread, newest first.
func (s *InboxesService) Emails(ctx context.Context, inboxID, threadID string, p *ListParams) (*List[InboxEmail], error) {
	return do[List[InboxEmail]](ctx, s.c, http.MethodGet, withQuery(ib(inboxID)+"/threads/"+esc(threadID)+"/emails", p.values()), nil)
}

// GetEmail returns one message with its body and attachments.
func (s *InboxesService) GetEmail(ctx context.Context, inboxID, threadID, emailID string) (*InboxEmail, error) {
	return do[InboxEmail](ctx, s.c, http.MethodGet, ib(inboxID)+"/threads/"+esc(threadID)+"/emails/"+esc(emailID), nil)
}

// Reply answers a message through the normal sending pipeline.
func (s *InboxesService) Reply(ctx context.Context, inboxID, threadID, emailID string, p *ReplyParams) (*InboxSendResult, error) {
	return do[InboxSendResult](ctx, s.c, http.MethodPost, ib(inboxID)+"/threads/"+esc(threadID)+"/emails/"+esc(emailID)+"/reply", p)
}

// Forward forwards a message (with its attachments).
func (s *InboxesService) Forward(ctx context.Context, inboxID, threadID, emailID string, p *ForwardParams) (*InboxSendResult, error) {
	return do[InboxSendResult](ctx, s.c, http.MethodPost, ib(inboxID)+"/threads/"+esc(threadID)+"/emails/"+esc(emailID)+"/forward", p)
}

// CreateDraft saves a draft.
func (s *InboxesService) CreateDraft(ctx context.Context, inboxID string, p *DraftParams) (*Draft, error) {
	return do[Draft](ctx, s.c, http.MethodPost, ib(inboxID)+"/drafts", p)
}

// Drafts lists drafts.
func (s *InboxesService) Drafts(ctx context.Context, inboxID string, p *ListParams) (*List[Draft], error) {
	return do[List[Draft]](ctx, s.c, http.MethodGet, withQuery(ib(inboxID)+"/drafts", p.values()), nil)
}

// GetDraft returns a draft.
func (s *InboxesService) GetDraft(ctx context.Context, inboxID, draftID string) (*Draft, error) {
	return do[Draft](ctx, s.c, http.MethodGet, ib(inboxID)+"/drafts/"+esc(draftID), nil)
}

// UpdateDraft edits a draft.
func (s *InboxesService) UpdateDraft(ctx context.Context, inboxID, draftID string, p *DraftParams) (*Draft, error) {
	return do[Draft](ctx, s.c, http.MethodPatch, ib(inboxID)+"/drafts/"+esc(draftID), p)
}

// RemoveDraft deletes a draft.
func (s *InboxesService) RemoveDraft(ctx context.Context, inboxID, draftID string) error {
	return s.c.Request(ctx, http.MethodDelete, ib(inboxID)+"/drafts/"+esc(draftID), nil, nil)
}

// SendDraft sends and deletes a draft; one that fails validation stays.
func (s *InboxesService) SendDraft(ctx context.Context, inboxID, draftID string) (*InboxSendResult, error) {
	return do[InboxSendResult](ctx, s.c, http.MethodPost, ib(inboxID)+"/drafts/"+esc(draftID)+"/send", nil)
}
