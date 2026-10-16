package usend

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Tag is a name/value pair attached to an email.
type Tag struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Attachment is a file sent with an email. Content is raw bytes; it is
// base64-encoded on the wire.
type Attachment struct {
	Filename    string `json:"filename"`
	Content     []byte `json:"content"`
	ContentType string `json:"content_type,omitempty"`
}

// EmailTemplate sends a published template instead of html/text.
type EmailTemplate struct {
	ID        string         `json:"id"` // template ID or alias
	Variables map[string]any `json:"variables,omitempty"`
}

// SendEmailParams is the body of POST /emails. Send at least one of HTML, Text
// or Template. ScheduledAt is an RFC 3339 time or a phrase such as "in 1 hour".
type SendEmailParams struct {
	From        string            `json:"from"`
	To          []string          `json:"to"`
	Subject     string            `json:"subject,omitempty"`
	HTML        string            `json:"html,omitempty"`
	Text        string            `json:"text,omitempty"`
	Cc          []string          `json:"cc,omitempty"`
	Bcc         []string          `json:"bcc,omitempty"`
	ReplyTo     []string          `json:"reply_to,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Tags        []Tag             `json:"tags,omitempty"`
	Attachments []Attachment      `json:"attachments,omitempty"`
	ScheduledAt string            `json:"scheduled_at,omitempty"`
	Template    *EmailTemplate    `json:"template,omitempty"`
}

// SendEmailResponse is returned by Send.
type SendEmailResponse struct {
	ID string `json:"id"`
}

// BatchError describes one rejected email of a permissive batch.
type BatchError struct {
	Index   int    `json:"index"`
	Message string `json:"message"`
}

// BatchResponse is returned by Batch. Errors is only set in permissive mode.
type BatchResponse struct {
	Data   []SendEmailResponse `json:"data"`
	Errors []BatchError        `json:"errors,omitempty"`
}

// Email is a sent email. Lists omit HTML and Text.
type Email struct {
	Object      string     `json:"object"`
	ID          string     `json:"id"`
	From        string     `json:"from"`
	To          []string   `json:"to"`
	Cc          []string   `json:"cc"`
	Bcc         []string   `json:"bcc"`
	ReplyTo     []string   `json:"reply_to"`
	Subject     string     `json:"subject"`
	HTML        string     `json:"html,omitempty"`
	Text        string     `json:"text,omitempty"`
	Tags        []Tag      `json:"tags"`
	CreatedAt   time.Time  `json:"created_at"`
	ScheduledAt *time.Time `json:"scheduled_at"`
	// LastEvent is one of queued, scheduled, sent, delivered, delivery_delayed,
	// bounced, complained, opened, clicked, failed, canceled.
	LastEvent   string  `json:"last_event"`
	BroadcastID *string `json:"broadcast_id"`
}

// EmailEvent is one entry of an email's event history.
type EmailEvent struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"` // email.delivered, email.bounced, ...
	Recipient string          `json:"recipient"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
}

// EmailAttachment is an attachment of a sent or received email.
type EmailAttachment struct {
	Object             string    `json:"object"`
	ID                 string    `json:"id"`
	Filename           string    `json:"filename"`
	ContentType        string    `json:"content_type"`
	ContentDisposition string    `json:"content_disposition"`
	Size               int       `json:"size"`
	DownloadURL        string    `json:"download_url"` // needs no API key, valid one hour
	ExpiresAt          time.Time `json:"expires_at"`
}

// EmailsService is client.Emails.
type EmailsService struct{ c *Client }

// Send sends one email. Pass WithIdempotencyKey to make retries safe.
func (s *EmailsService) Send(ctx context.Context, p *SendEmailParams, opts ...RequestOption) (*SendEmailResponse, error) {
	var out SendEmailResponse
	if err := s.c.Request(ctx, http.MethodPost, "/emails", p, &out, opts...); err != nil {
		return nil, err
	}
	return &out, nil
}

// Batch sends up to 100 emails in one call (no attachments or scheduled_at).
// Use WithBatchValidation("permissive") to send the valid ones and get the
// rejected ones in BatchResponse.Errors.
func (s *EmailsService) Batch(ctx context.Context, emails []SendEmailParams, opts ...RequestOption) (*BatchResponse, error) {
	var out BatchResponse
	if err := s.c.Request(ctx, http.MethodPost, "/emails/batch", emails, &out, opts...); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get returns one email.
func (s *EmailsService) Get(ctx context.Context, id string) (*Email, error) {
	var out Email
	if err := s.c.Request(ctx, http.MethodGet, "/emails/"+esc(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// List returns a page of emails, newest first.
func (s *EmailsService) List(ctx context.Context, p *ListParams) (*List[Email], error) {
	var out List[Email]
	if err := s.c.Request(ctx, http.MethodGet, withQuery("/emails", p.values()), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update reschedules a scheduled email. scheduledAt is RFC 3339 or "in 1 hour".
func (s *EmailsService) Update(ctx context.Context, id, scheduledAt string) (*IDResult, error) {
	var out IDResult
	body := map[string]string{"scheduled_at": scheduledAt}
	if err := s.c.Request(ctx, http.MethodPatch, "/emails/"+esc(id), body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Cancel cancels a scheduled email.
func (s *EmailsService) Cancel(ctx context.Context, id string) (*IDResult, error) {
	var out IDResult
	if err := s.c.Request(ctx, http.MethodPost, "/emails/"+esc(id)+"/cancel", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Events returns the event history of an email, oldest first.
func (s *EmailsService) Events(ctx context.Context, id string) (*List[EmailEvent], error) {
	var out List[EmailEvent]
	if err := s.c.Request(ctx, http.MethodGet, "/emails/"+esc(id)+"/events", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Attachments lists the attachments of a sent email.
func (s *EmailsService) Attachments(ctx context.Context, emailID string) (*List[EmailAttachment], error) {
	var out List[EmailAttachment]
	if err := s.c.Request(ctx, http.MethodGet, "/emails/"+esc(emailID)+"/attachments", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Attachment returns one attachment of a sent email, with a signed download URL.
func (s *EmailsService) Attachment(ctx context.Context, emailID, id string) (*EmailAttachment, error) {
	var out EmailAttachment
	if err := s.c.Request(ctx, http.MethodGet, "/emails/"+esc(emailID)+"/attachments/"+esc(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// InboundEmail is a received email.
type InboundEmail struct {
	Object          string              `json:"object"`
	ID              string              `json:"id"`
	From            string              `json:"from"`
	To              []string            `json:"to"`
	Cc              []string            `json:"cc,omitempty"`
	ReplyTo         []string            `json:"reply_to,omitempty"`
	Subject         string              `json:"subject"`
	HTML            string              `json:"html,omitempty"`
	Text            string              `json:"text,omitempty"`
	Headers         map[string]any      `json:"headers,omitempty"`
	Attachments     []InboundAttachment `json:"attachments,omitempty"`
	AttachmentCount int                 `json:"attachment_count,omitempty"`
	CreatedAt       time.Time           `json:"created_at"`
}

// InboundAttachment describes an attachment of a received email.
type InboundAttachment struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
}

// InboundService is client.Inbound (received mail).
type InboundService struct{ c *Client }

// List returns received emails, newest first.
func (s *InboundService) List(ctx context.Context, p *ListParams) (*List[InboundEmail], error) {
	var out List[InboundEmail]
	if err := s.c.Request(ctx, http.MethodGet, withQuery("/inbound/emails", p.values()), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get returns one received email.
func (s *InboundService) Get(ctx context.Context, id string) (*InboundEmail, error) {
	var out InboundEmail
	if err := s.c.Request(ctx, http.MethodGet, "/inbound/emails/"+esc(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
