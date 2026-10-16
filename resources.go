package usend

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// get/post helpers keep the service methods short.

func (c *Client) call(ctx context.Context, method, path string, body any, out any) error {
	return c.Request(ctx, method, path, body, out)
}

// ---------------------------------------------------------------- domains

// DomainRecord is one DNS record a domain needs.
type DomainRecord struct {
	Record   string `json:"record"` // DKIM, SPF, DMARC, Receiving
	Name     string `json:"name"`   // relative to the domain, "@" = apex
	Type     string `json:"type"`
	TTL      string `json:"ttl"`
	Status   string `json:"status"` // pending, verified, failed
	Value    string `json:"value"`
	Priority *int   `json:"priority,omitempty"`
}

// DomainHint explains why a record is not verified.
type DomainHint struct {
	Record  string `json:"record"`
	Message string `json:"message"`
}

// Domain is a sending (and optionally receiving) domain.
type Domain struct {
	Object string `json:"object"`
	ID     string `json:"id"`
	Name   string `json:"name"`
	// Status is not_started, pending, verified, failed or temporary_failure.
	Status           string         `json:"status"`
	CreatedAt        time.Time      `json:"created_at"`
	Region           string         `json:"region"`
	OpenTracking     bool           `json:"open_tracking"`
	ClickTracking    bool           `json:"click_tracking"`
	ReceivingEnabled bool           `json:"receiving_enabled"`
	CustomReturnPath string         `json:"custom_return_path"`
	DNSProvider      string         `json:"dns_provider,omitempty"`
	Records          []DomainRecord `json:"records"`
	Hints            []DomainHint   `json:"hints,omitempty"`
}

// CreateDomainParams is the body of POST /domains.
type CreateDomainParams struct {
	Name             string `json:"name"`
	CustomReturnPath string `json:"custom_return_path,omitempty"`
	TLS              string `json:"tls,omitempty"` // "opportunistic" (default) or "enforced"
}

// UpdateDomainParams is the body of PATCH /domains/{id}; nil fields are left unchanged.
type UpdateDomainParams struct {
	OpenTracking     *bool   `json:"open_tracking,omitempty"`
	ClickTracking    *bool   `json:"click_tracking,omitempty"`
	ReceivingEnabled *bool   `json:"receiving_enabled,omitempty"`
	CustomReturnPath *string `json:"custom_return_path,omitempty"`
	TLS              *string `json:"tls,omitempty"`
}

// AutoConfigureResult is the response of DomainsService.AutoConfigure.
type AutoConfigureResult struct {
	Object   string `json:"object"`
	ID       string `json:"id"`
	Provider string `json:"provider"`
	Zone     string `json:"zone"`
	Records  []struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		Value   string `json:"value"`
		Action  string `json:"action"` // created, updated, unchanged, skipped, conflict
		Message string `json:"message,omitempty"`
	} `json:"records"`
}

// DomainsService is client.Domains.
type DomainsService struct{ c *Client }

// Create adds a domain.
func (s *DomainsService) Create(ctx context.Context, p *CreateDomainParams) (*Domain, error) {
	var out Domain
	if err := s.c.call(ctx, http.MethodPost, "/domains", p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// List returns all domains.
func (s *DomainsService) List(ctx context.Context) (*List[Domain], error) {
	var out List[Domain]
	if err := s.c.call(ctx, http.MethodGet, "/domains", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get returns one domain with its DNS records.
func (s *DomainsService) Get(ctx context.Context, id string) (*Domain, error) {
	var out Domain
	if err := s.c.call(ctx, http.MethodGet, "/domains/"+esc(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update changes tracking, receiving or the return-path label.
func (s *DomainsService) Update(ctx context.Context, id string, p *UpdateDomainParams) (*Domain, error) {
	var out Domain
	if err := s.c.call(ctx, http.MethodPatch, "/domains/"+esc(id), p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Verify starts verification (async); poll Get for the result.
func (s *DomainsService) Verify(ctx context.Context, id string) (*IDResult, error) {
	var out IDResult
	if err := s.c.call(ctx, http.MethodPost, "/domains/"+esc(id)+"/verify", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AutoConfigure publishes the domain's records in your Cloudflare zone. The
// token is used for this request only and is never stored by usend.
func (s *DomainsService) AutoConfigure(ctx context.Context, id, cloudflareToken string) (*AutoConfigureResult, error) {
	var out AutoConfigureResult
	body := map[string]string{"provider": "cloudflare", "api_token": cloudflareToken}
	if err := s.c.call(ctx, http.MethodPost, "/domains/"+esc(id)+"/auto-configure", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Remove deletes a domain.
func (s *DomainsService) Remove(ctx context.Context, id string) (*DeleteResult, error) {
	var out DeleteResult
	if err := s.c.call(ctx, http.MethodDelete, "/domains/"+esc(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --------------------------------------------------------------- api keys

// APIKey is an API key listing entry (the token is only shown at creation).
type APIKey struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Permission  string     `json:"permission"` // full_access or sending_access
	DomainID    *string    `json:"domain_id"`
	TokenPrefix string     `json:"token_prefix"`
	CreatedAt   time.Time  `json:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
}

// CreateAPIKeyParams is the body of POST /api-keys.
type CreateAPIKeyParams struct {
	Name       string `json:"name"`
	Permission string `json:"permission"` // "full_access" or "sending_access"
	DomainID   string `json:"domain_id,omitempty"`
}

// CreateAPIKeyResponse carries the new token, shown only once.
type CreateAPIKeyResponse struct {
	ID    string `json:"id"`
	Token string `json:"token"`
}

// APIKeysService is client.APIKeys.
type APIKeysService struct{ c *Client }

// Create makes a key. Store Token now; it cannot be read again.
func (s *APIKeysService) Create(ctx context.Context, p *CreateAPIKeyParams) (*CreateAPIKeyResponse, error) {
	var out CreateAPIKeyResponse
	if err := s.c.call(ctx, http.MethodPost, "/api-keys", p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// List returns all keys.
func (s *APIKeysService) List(ctx context.Context) (*List[APIKey], error) {
	var out List[APIKey]
	if err := s.c.call(ctx, http.MethodGet, "/api-keys", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Remove revokes a key.
func (s *APIKeysService) Remove(ctx context.Context, id string) error {
	return s.c.call(ctx, http.MethodDelete, "/api-keys/"+esc(id), nil, nil)
}

// -------------------------------------------------------------- templates

// TemplateVariable declares a {{{KEY}}} placeholder.
type TemplateVariable struct {
	Key           string `json:"key"`
	Type          string `json:"type"` // string or number
	FallbackValue any    `json:"fallback_value,omitempty"`
}

// Template is an email template. Sends use the last published version.
type Template struct {
	Object                 string             `json:"object"`
	ID                     string             `json:"id"`
	Name                   string             `json:"name"`
	Alias                  string             `json:"alias"`
	Status                 string             `json:"status"` // draft or published
	PublishedAt            *time.Time         `json:"published_at"`
	HasUnpublishedVersions bool               `json:"has_unpublished_versions"`
	From                   string             `json:"from,omitempty"`
	Subject                string             `json:"subject,omitempty"`
	ReplyTo                []string           `json:"reply_to,omitempty"`
	HTML                   string             `json:"html,omitempty"`
	Text                   string             `json:"text,omitempty"`
	Variables              []TemplateVariable `json:"variables,omitempty"`
	CreatedAt              time.Time          `json:"created_at"`
	UpdatedAt              time.Time          `json:"updated_at"`
}

// TemplateParams is the body of create and update; on update all fields are optional.
type TemplateParams struct {
	Name      string             `json:"name,omitempty"`
	Alias     string             `json:"alias,omitempty"`
	HTML      string             `json:"html,omitempty"`
	Text      string             `json:"text,omitempty"`
	From      string             `json:"from,omitempty"`
	Subject   string             `json:"subject,omitempty"`
	ReplyTo   []string           `json:"reply_to,omitempty"`
	Variables []TemplateVariable `json:"variables,omitempty"`
}

// TemplatesService is client.Templates. Methods take an ID or an alias.
type TemplatesService struct{ c *Client }

// Create makes a draft template.
func (s *TemplatesService) Create(ctx context.Context, p *TemplateParams) (*IDResult, error) {
	var out IDResult
	if err := s.c.call(ctx, http.MethodPost, "/templates", p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// List returns a page of templates.
func (s *TemplatesService) List(ctx context.Context, p *ListParams) (*List[Template], error) {
	var out List[Template]
	if err := s.c.call(ctx, http.MethodGet, withQuery("/templates", p.values()), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get returns a template by ID or alias.
func (s *TemplatesService) Get(ctx context.Context, idOrAlias string) (*Template, error) {
	var out Template
	if err := s.c.call(ctx, http.MethodGet, "/templates/"+esc(idOrAlias), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update edits the draft.
func (s *TemplatesService) Update(ctx context.Context, idOrAlias string, p *TemplateParams) (*IDResult, error) {
	var out IDResult
	if err := s.c.call(ctx, http.MethodPatch, "/templates/"+esc(idOrAlias), p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Publish makes the draft the version that sends use.
func (s *TemplatesService) Publish(ctx context.Context, idOrAlias string) (*IDResult, error) {
	var out IDResult
	if err := s.c.call(ctx, http.MethodPost, "/templates/"+esc(idOrAlias)+"/publish", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Duplicate copies a template into a new draft.
func (s *TemplatesService) Duplicate(ctx context.Context, idOrAlias string) (*IDResult, error) {
	var out IDResult
	if err := s.c.call(ctx, http.MethodPost, "/templates/"+esc(idOrAlias)+"/duplicate", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Remove deletes a template.
func (s *TemplatesService) Remove(ctx context.Context, idOrAlias string) (*DeleteResult, error) {
	var out DeleteResult
	if err := s.c.call(ctx, http.MethodDelete, "/templates/"+esc(idOrAlias), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --------------------------------------------- segments and audiences

// Segment is a group of contacts. Audiences are the same thing under the older name.
type Segment struct {
	Object       string    `json:"object"`
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	CreatedAt    time.Time `json:"created_at"`
	ContactCount int       `json:"contact_count"`
}

type groupService struct {
	c    *Client
	path string // "/segments" or "/audiences"
}

func (s groupService) create(ctx context.Context, name string) (*Segment, error) {
	var out Segment
	if err := s.c.call(ctx, http.MethodPost, s.path, map[string]string{"name": name}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s groupService) list(ctx context.Context) (*List[Segment], error) {
	var out List[Segment]
	if err := s.c.call(ctx, http.MethodGet, s.path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s groupService) get(ctx context.Context, id string) (*Segment, error) {
	var out Segment
	if err := s.c.call(ctx, http.MethodGet, s.path+"/"+esc(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s groupService) remove(ctx context.Context, id string) (*DeleteResult, error) {
	var out DeleteResult
	if err := s.c.call(ctx, http.MethodDelete, s.path+"/"+esc(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SegmentsService is client.Segments.
type SegmentsService struct{ c *Client }

func (s *SegmentsService) g() groupService { return groupService{s.c, "/segments"} }

// Create makes a segment.
func (s *SegmentsService) Create(ctx context.Context, name string) (*Segment, error) {
	return s.g().create(ctx, name)
}

// List returns all segments.
func (s *SegmentsService) List(ctx context.Context) (*List[Segment], error) { return s.g().list(ctx) }

// Get returns one segment.
func (s *SegmentsService) Get(ctx context.Context, id string) (*Segment, error) {
	return s.g().get(ctx, id)
}

// Remove deletes a segment (refused while a broadcast uses it).
func (s *SegmentsService) Remove(ctx context.Context, id string) (*DeleteResult, error) {
	return s.g().remove(ctx, id)
}

// AudiencesService is client.Audiences, the older name of segments with
// contacts nested under the audience.
type AudiencesService struct{ c *Client }

func (s *AudiencesService) g() groupService { return groupService{s.c, "/audiences"} }

// Create makes an audience.
func (s *AudiencesService) Create(ctx context.Context, name string) (*Segment, error) {
	return s.g().create(ctx, name)
}

// List returns all audiences.
func (s *AudiencesService) List(ctx context.Context) (*List[Segment], error) { return s.g().list(ctx) }

// Get returns one audience.
func (s *AudiencesService) Get(ctx context.Context, id string) (*Segment, error) {
	return s.g().get(ctx, id)
}

// Remove deletes an audience.
func (s *AudiencesService) Remove(ctx context.Context, id string) (*DeleteResult, error) {
	return s.g().remove(ctx, id)
}

// CreateContact adds a contact to an audience.
func (s *AudiencesService) CreateContact(ctx context.Context, audienceID string, p *CreateContactParams) (*IDResult, error) {
	var out IDResult
	if err := s.c.call(ctx, http.MethodPost, "/audiences/"+esc(audienceID)+"/contacts", p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListContacts returns a page of an audience's contacts.
func (s *AudiencesService) ListContacts(ctx context.Context, audienceID string, p *ListParams) (*List[Contact], error) {
	var out List[Contact]
	path := withQuery("/audiences/"+esc(audienceID)+"/contacts", p.values())
	if err := s.c.call(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetContact returns one contact by ID or email.
func (s *AudiencesService) GetContact(ctx context.Context, audienceID, idOrEmail string) (*Contact, error) {
	var out Contact
	if err := s.c.call(ctx, http.MethodGet, "/audiences/"+esc(audienceID)+"/contacts/"+esc(idOrEmail), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateContact edits a contact.
func (s *AudiencesService) UpdateContact(ctx context.Context, audienceID, idOrEmail string, p *UpdateContactParams) (*IDResult, error) {
	var out IDResult
	if err := s.c.call(ctx, http.MethodPatch, "/audiences/"+esc(audienceID)+"/contacts/"+esc(idOrEmail), p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RemoveContact deletes a contact from an audience.
func (s *AudiencesService) RemoveContact(ctx context.Context, audienceID, idOrEmail string) (*DeleteResult, error) {
	var out DeleteResult
	if err := s.c.call(ctx, http.MethodDelete, "/audiences/"+esc(audienceID)+"/contacts/"+esc(idOrEmail), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ImportContacts imports a CSV into an audience.
func (s *AudiencesService) ImportContacts(ctx context.Context, audienceID, csv string) (*ImportResult, error) {
	var out ImportResult
	if err := s.c.call(ctx, http.MethodPost, "/audiences/"+esc(audienceID)+"/contacts/import", map[string]string{"csv": csv}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// --------------------------------------------------------------- contacts

// Contact is an address with optional properties.
type Contact struct {
	Object       string         `json:"object"`
	ID           string         `json:"id"`
	Email        string         `json:"email"`
	FirstName    string         `json:"first_name"`
	LastName     string         `json:"last_name"`
	Unsubscribed bool           `json:"unsubscribed"`
	Properties   map[string]any `json:"properties,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

// CreateContactParams is the body of POST /contacts. Without Segments the
// contact joins the default "General" segment.
type CreateContactParams struct {
	Email        string         `json:"email"`
	FirstName    string         `json:"first_name,omitempty"`
	LastName     string         `json:"last_name,omitempty"`
	Unsubscribed bool           `json:"unsubscribed,omitempty"`
	Segments     []string       `json:"segments,omitempty"`
	Properties   map[string]any `json:"properties,omitempty"`
}

// UpdateContactParams is the body of PATCH; nil and empty fields are left
// unchanged. A nil value in Properties removes that property.
type UpdateContactParams struct {
	FirstName    *string        `json:"first_name,omitempty"`
	LastName     *string        `json:"last_name,omitempty"`
	Unsubscribed *bool          `json:"unsubscribed,omitempty"`
	Properties   map[string]any `json:"properties,omitempty"`
}

// ListContactsParams filters GET /contacts.
type ListContactsParams struct {
	ListParams
	SegmentID    string
	TopicID      string
	Subscription string // "subscribed" or "unsubscribed"
	Q            string // search text
}

// ContactStats is the response of GET /contacts/stats.
type ContactStats struct {
	Object        string `json:"object"`
	Total         int    `json:"total"`
	Subscribers   int    `json:"subscribers"`
	Unsubscribers int    `json:"unsubscribers"`
}

// ImportResult summarises a CSV import.
type ImportResult struct {
	Imported int `json:"imported"`
	Updated  int `json:"updated"`
	Skipped  int `json:"skipped"`
	Errors   []struct {
		Row     int    `json:"row"`
		Email   string `json:"email,omitempty"`
		Message string `json:"message"`
	} `json:"errors"`
	IgnoredColumns []string `json:"ignored_columns"`
}

// ContactTopic is a contact's subscription to a topic.
type ContactTopic struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Subscription string `json:"subscription"` // opt_in or opt_out
	Explicit     bool   `json:"explicit"`
}

// TopicChoice sets a contact's subscription to one topic.
type TopicChoice struct {
	ID           string `json:"id"`
	Subscription string `json:"subscription"` // opt_in or opt_out
}

// ContactsService is client.Contacts (account-wide, one row per address).
// Methods take a contact ID or an email address.
type ContactsService struct{ c *Client }

// Create adds a contact.
func (s *ContactsService) Create(ctx context.Context, p *CreateContactParams) (*IDResult, error) {
	var out IDResult
	if err := s.c.call(ctx, http.MethodPost, "/contacts", p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// List returns a page of contacts.
func (s *ContactsService) List(ctx context.Context, p *ListContactsParams) (*List[Contact], error) {
	v := url.Values{}
	if p != nil {
		v = p.ListParams.values()
		for k, val := range map[string]string{"segment_id": p.SegmentID, "topic_id": p.TopicID, "subscription": p.Subscription, "q": p.Q} {
			if val != "" {
				v.Set(k, val)
			}
		}
	}
	var out List[Contact]
	if err := s.c.call(ctx, http.MethodGet, withQuery("/contacts", v), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Stats returns contact counts.
func (s *ContactsService) Stats(ctx context.Context) (*ContactStats, error) {
	var out ContactStats
	if err := s.c.call(ctx, http.MethodGet, "/contacts/stats", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Import imports a CSV into a segment ("" = General).
func (s *ContactsService) Import(ctx context.Context, csv, segmentID string) (*ImportResult, error) {
	body := map[string]string{"csv": csv}
	if segmentID != "" {
		body["segment_id"] = segmentID
	}
	var out ImportResult
	if err := s.c.call(ctx, http.MethodPost, "/contacts/import", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get returns one contact.
func (s *ContactsService) Get(ctx context.Context, idOrEmail string) (*Contact, error) {
	var out Contact
	if err := s.c.call(ctx, http.MethodGet, "/contacts/"+esc(idOrEmail), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update edits a contact in every segment it belongs to.
func (s *ContactsService) Update(ctx context.Context, idOrEmail string, p *UpdateContactParams) (*IDResult, error) {
	var out IDResult
	if err := s.c.call(ctx, http.MethodPatch, "/contacts/"+esc(idOrEmail), p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Remove deletes a contact from every segment.
func (s *ContactsService) Remove(ctx context.Context, idOrEmail string) (*DeleteResult, error) {
	var out DeleteResult
	if err := s.c.call(ctx, http.MethodDelete, "/contacts/"+esc(idOrEmail), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Segments lists the segments a contact belongs to.
func (s *ContactsService) Segments(ctx context.Context, idOrEmail string) (*List[Segment], error) {
	var out List[Segment]
	if err := s.c.call(ctx, http.MethodGet, "/contacts/"+esc(idOrEmail)+"/segments", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AddToSegment adds a contact to a segment.
func (s *ContactsService) AddToSegment(ctx context.Context, idOrEmail, segmentID string) error {
	return s.c.call(ctx, http.MethodPost, "/contacts/"+esc(idOrEmail)+"/segments/"+esc(segmentID), nil, nil)
}

// RemoveFromSegment removes a contact from a segment.
func (s *ContactsService) RemoveFromSegment(ctx context.Context, idOrEmail, segmentID string) error {
	return s.c.call(ctx, http.MethodDelete, "/contacts/"+esc(idOrEmail)+"/segments/"+esc(segmentID), nil, nil)
}

// Topics returns a contact's topic subscriptions.
func (s *ContactsService) Topics(ctx context.Context, idOrEmail string) ([]ContactTopic, error) {
	// The API answers with a list envelope or a bare array depending on version.
	var raw jsonList[ContactTopic]
	if err := s.c.call(ctx, http.MethodGet, "/contacts/"+esc(idOrEmail)+"/topics", nil, &raw); err != nil {
		return nil, err
	}
	return raw.items, nil
}

// UpdateTopics sets a contact's topic subscriptions.
func (s *ContactsService) UpdateTopics(ctx context.Context, idOrEmail string, topics []TopicChoice) error {
	body := map[string]any{"topics": topics}
	return s.c.call(ctx, http.MethodPatch, "/contacts/"+esc(idOrEmail)+"/topics", body, nil)
}

// ------------------------------------------------------ contact properties

// ContactProperty defines a typed custom field on contacts.
type ContactProperty struct {
	Object        string    `json:"object"`
	ID            string    `json:"id"`
	Key           string    `json:"key"`
	Type          string    `json:"type"` // string, number, boolean, date
	FallbackValue any       `json:"fallback_value"`
	CreatedAt     time.Time `json:"created_at"`
}

// CreateContactPropertyParams is the body of POST /contact-properties.
type CreateContactPropertyParams struct {
	Key           string `json:"key"`
	Type          string `json:"type"`
	FallbackValue any    `json:"fallback_value,omitempty"`
}

// ContactPropertiesService is client.Properties.
type ContactPropertiesService struct{ c *Client }

// List returns all properties.
func (s *ContactPropertiesService) List(ctx context.Context) (*List[ContactProperty], error) {
	var out List[ContactProperty]
	if err := s.c.call(ctx, http.MethodGet, "/contact-properties", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Create defines a property.
func (s *ContactPropertiesService) Create(ctx context.Context, p *CreateContactPropertyParams) (*ContactProperty, error) {
	var out ContactProperty
	if err := s.c.call(ctx, http.MethodPost, "/contact-properties", p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateFallback changes a property's fallback value (key and type are fixed).
func (s *ContactPropertiesService) UpdateFallback(ctx context.Context, id string, fallback any) (*ContactProperty, error) {
	var out ContactProperty
	if err := s.c.call(ctx, http.MethodPatch, "/contact-properties/"+esc(id), map[string]any{"fallback_value": fallback}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Remove deletes a property and its values from every contact.
func (s *ContactPropertiesService) Remove(ctx context.Context, id string) error {
	return s.c.call(ctx, http.MethodDelete, "/contact-properties/"+esc(id), nil, nil)
}

// ----------------------------------------------------------------- topics

// Topic lets people leave one kind of email without leaving everything.
type Topic struct {
	Object              string    `json:"object"`
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	Description         string    `json:"description"`
	DefaultSubscription string    `json:"default_subscription"` // opt_in or opt_out
	Subscribers         int       `json:"subscribers"`
	CreatedAt           time.Time `json:"created_at"`
}

// TopicParams is the body of create and update.
type TopicParams struct {
	Name                string `json:"name,omitempty"`
	Description         string `json:"description,omitempty"`
	DefaultSubscription string `json:"default_subscription,omitempty"`
}

// TopicsService is client.Topics.
type TopicsService struct{ c *Client }

// List returns all topics.
func (s *TopicsService) List(ctx context.Context) (*List[Topic], error) {
	var out List[Topic]
	if err := s.c.call(ctx, http.MethodGet, "/topics", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get returns one topic.
func (s *TopicsService) Get(ctx context.Context, id string) (*Topic, error) {
	var out Topic
	if err := s.c.call(ctx, http.MethodGet, "/topics/"+esc(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Create makes a topic.
func (s *TopicsService) Create(ctx context.Context, p *TopicParams) (*Topic, error) {
	var out Topic
	if err := s.c.call(ctx, http.MethodPost, "/topics", p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update edits a topic.
func (s *TopicsService) Update(ctx context.Context, id string, p *TopicParams) (*Topic, error) {
	var out Topic
	if err := s.c.call(ctx, http.MethodPatch, "/topics/"+esc(id), p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Remove deletes a topic (refused while a broadcast uses it).
func (s *TopicsService) Remove(ctx context.Context, id string) error {
	return s.c.call(ctx, http.MethodDelete, "/topics/"+esc(id), nil, nil)
}

// ------------------------------------------------------------- broadcasts

// BroadcastStats are delivery counters of a broadcast.
type BroadcastStats struct {
	Total        int `json:"total"`
	Sent         int `json:"sent"`
	Delivered    int `json:"delivered"`
	Bounced      int `json:"bounced"`
	Opened       int `json:"opened"`
	Clicked      int `json:"clicked"`
	Unsubscribed int `json:"unsubscribed"`
}

// Broadcast is one email to a whole audience.
type Broadcast struct {
	Object     string   `json:"object"`
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	AudienceID string   `json:"audience_id"`
	TopicID    *string  `json:"topic_id"`
	From       string   `json:"from"`
	Subject    string   `json:"subject"`
	ReplyTo    []string `json:"reply_to"`
	HTML       string   `json:"html"`
	Text       string   `json:"text"`
	// Status is draft, queued, sending, sent, failed, paused or cancelled.
	Status      string         `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	ScheduledAt *time.Time     `json:"scheduled_at"`
	SentAt      *time.Time     `json:"sent_at"`
	Stats       BroadcastStats `json:"stats"`
}

// BroadcastParams is the body of create and (draft-only) update. On update,
// point TopicID at an empty string to detach the topic.
type BroadcastParams struct {
	AudienceID string   `json:"audience_id,omitempty"`
	From       string   `json:"from,omitempty"`
	Subject    string   `json:"subject,omitempty"`
	HTML       string   `json:"html,omitempty"`
	Text       string   `json:"text,omitempty"`
	ReplyTo    []string `json:"reply_to,omitempty"`
	Name       string   `json:"name,omitempty"`
	TopicID    *string  `json:"topic_id,omitempty"` // pointer to "" clears the topic
}

// BroadcastsService is client.Broadcasts.
type BroadcastsService struct{ c *Client }

// Create makes a draft broadcast.
func (s *BroadcastsService) Create(ctx context.Context, p *BroadcastParams) (*SendEmailResponse, error) {
	var out SendEmailResponse
	if err := s.c.call(ctx, http.MethodPost, "/broadcasts", p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// List returns all broadcasts.
func (s *BroadcastsService) List(ctx context.Context) (*List[Broadcast], error) {
	var out List[Broadcast]
	if err := s.c.call(ctx, http.MethodGet, "/broadcasts", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get returns one broadcast with its stats.
func (s *BroadcastsService) Get(ctx context.Context, id string) (*Broadcast, error) {
	var out Broadcast
	if err := s.c.call(ctx, http.MethodGet, "/broadcasts/"+esc(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update edits a draft.
func (s *BroadcastsService) Update(ctx context.Context, id string, p *BroadcastParams) error {
	return s.c.call(ctx, http.MethodPatch, "/broadcasts/"+esc(id), p, nil)
}

// Send sends now, or at scheduledAt (RFC 3339) when it is not empty.
func (s *BroadcastsService) Send(ctx context.Context, id, scheduledAt string) error {
	body := map[string]string{}
	if scheduledAt != "" {
		body["scheduled_at"] = scheduledAt
	}
	return s.c.call(ctx, http.MethodPost, "/broadcasts/"+esc(id)+"/send", body, nil)
}

// Test sends one copy to a single address.
func (s *BroadcastsService) Test(ctx context.Context, id, to string) error {
	return s.c.call(ctx, http.MethodPost, "/broadcasts/"+esc(id)+"/test", map[string]string{"to": to}, nil)
}

// Remove deletes a draft.
func (s *BroadcastsService) Remove(ctx context.Context, id string) error {
	return s.c.call(ctx, http.MethodDelete, "/broadcasts/"+esc(id), nil, nil)
}

// --------------------------------------------------------------- webhooks

// Webhook is a webhook endpoint. SigningSecret ("whsec_...") is what
// VerifyWebhook needs.
type Webhook struct {
	Object        string    `json:"object"`
	ID            string    `json:"id"`
	Endpoint      string    `json:"endpoint"`
	Events        []string  `json:"events"`
	Enabled       bool      `json:"enabled"`
	SigningSecret string    `json:"signing_secret"`
	CreatedAt     time.Time `json:"created_at"`
}

// CreateWebhookParams is the body of POST /webhooks.
type CreateWebhookParams struct {
	Endpoint string   `json:"endpoint"`
	Events   []string `json:"events"`
}

// UpdateWebhookParams is the body of PATCH /webhooks/{id}.
type UpdateWebhookParams struct {
	Endpoint string   `json:"endpoint,omitempty"`
	Events   []string `json:"events,omitempty"`
	Enabled  *bool    `json:"enabled,omitempty"`
}

// WebhookDelivery is one delivery attempt series of an event.
type WebhookDelivery struct {
	ID             string     `json:"id"`
	EventType      string     `json:"event_type"`
	Status         string     `json:"status"` // pending, succeeded, failed
	Attempts       int        `json:"attempts"`
	ResponseStatus *int       `json:"response_status"`
	LastError      *string    `json:"last_error"`
	CreatedAt      time.Time  `json:"created_at"`
	DeliveredAt    *time.Time `json:"delivered_at"`
}

// WebhooksService is client.Webhooks.
type WebhooksService struct{ c *Client }

// Create registers an endpoint.
func (s *WebhooksService) Create(ctx context.Context, p *CreateWebhookParams) (*Webhook, error) {
	var out Webhook
	if err := s.c.call(ctx, http.MethodPost, "/webhooks", p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// List returns all webhooks.
func (s *WebhooksService) List(ctx context.Context) (*List[Webhook], error) {
	var out List[Webhook]
	if err := s.c.call(ctx, http.MethodGet, "/webhooks", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get returns one webhook.
func (s *WebhooksService) Get(ctx context.Context, id string) (*Webhook, error) {
	var out Webhook
	if err := s.c.call(ctx, http.MethodGet, "/webhooks/"+esc(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update edits a webhook.
func (s *WebhooksService) Update(ctx context.Context, id string, p *UpdateWebhookParams) (*Webhook, error) {
	var out Webhook
	if err := s.c.call(ctx, http.MethodPatch, "/webhooks/"+esc(id), p, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Remove deletes a webhook.
func (s *WebhooksService) Remove(ctx context.Context, id string) error {
	return s.c.call(ctx, http.MethodDelete, "/webhooks/"+esc(id), nil, nil)
}

// Deliveries returns a page of delivery attempts, newest first.
func (s *WebhooksService) Deliveries(ctx context.Context, id string, p *ListParams) (*List[WebhookDelivery], error) {
	var out List[WebhookDelivery]
	if err := s.c.call(ctx, http.MethodGet, withQuery("/webhooks/"+esc(id)+"/deliveries", p.values()), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Replay sends a delivery again.
func (s *WebhooksService) Replay(ctx context.Context, id, deliveryID string) error {
	return s.c.call(ctx, http.MethodPost, "/webhooks/"+esc(id)+"/deliveries/"+esc(deliveryID)+"/replay", nil, nil)
}

// ------------------------------------------------------------ suppressions

// Suppression is an address that is never sent to.
type Suppression struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Reason    string    `json:"reason"` // hard_bounce, complaint, unsubscribe, manual
	CreatedAt time.Time `json:"created_at"`
}

// SuppressionsService is client.Suppressions.
type SuppressionsService struct{ c *Client }

// List returns a page of suppressions.
func (s *SuppressionsService) List(ctx context.Context, p *ListParams) (*List[Suppression], error) {
	var out List[Suppression]
	if err := s.c.call(ctx, http.MethodGet, withQuery("/suppressions", p.values()), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Create suppresses an address manually.
func (s *SuppressionsService) Create(ctx context.Context, email string) (*Suppression, error) {
	var out Suppression
	if err := s.c.call(ctx, http.MethodPost, "/suppressions", map[string]string{"email": email}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Remove lifts a suppression.
func (s *SuppressionsService) Remove(ctx context.Context, id string) error {
	return s.c.call(ctx, http.MethodDelete, "/suppressions/"+esc(id), nil, nil)
}

// ------------------------------------------------------------------ stats

// DayStats is one day of sending statistics.
type DayStats struct {
	Date       string `json:"date"`
	Sent       int    `json:"sent"`
	Delivered  int    `json:"delivered"`
	Bounced    int    `json:"bounced"`
	Complained int    `json:"complained"`
	Opened     int    `json:"opened"`
	Clicked    int    `json:"clicked"`
}

// StatsResponse is the response of GET /stats.
type StatsResponse struct {
	Days   int `json:"days"`
	Totals struct {
		Sent       int `json:"sent"`
		Delivered  int `json:"delivered"`
		Bounced    int `json:"bounced"`
		Complained int `json:"complained"`
		Opened     int `json:"opened"`
		Clicked    int `json:"clicked"`
		Failed     int `json:"failed"`
		Delayed    int `json:"delayed"`
	} `json:"totals"`
	Daily []DayStats `json:"daily"`
}

// StatsService is client.Stats.
type StatsService struct{ c *Client }

// Get returns statistics for the last days days (0 = server default, 30).
func (s *StatsService) Get(ctx context.Context, days int) (*StatsResponse, error) {
	var out StatsResponse
	path := "/stats"
	if days > 0 {
		path += "?days=" + url.QueryEscape(itoa(days))
	}
	if err := s.c.call(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
