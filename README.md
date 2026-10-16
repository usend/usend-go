# usend for Go

The official Go SDK for [usend](https://usend.et), the email API for developers. Standard library only: no third-party dependencies. Needs Go 1.21 or newer.

```bash
go get usend.et/sdk/go
```

`usend.et/sdk/go` is a vanity import path served by usend itself, so your imports do not depend on where the source is hosted.

The package name is `usend`; import it with an alias so the path's last element reads clearly:

```go
import usend "usend.et/sdk/go"
```

## Quick start

```go
client, err := usend.New(os.Getenv("USEND_API_KEY")) // "" also reads USEND_API_KEY
if err != nil {
	log.Fatal(err)
}

res, err := client.Emails.Send(ctx, &usend.SendEmailParams{
	From:    "Acme <hello@acme.com>",
	To:      []string{"ann@example.com"},
	Subject: "Hello",
	HTML:    "<p>It works.</p>",
}, usend.WithIdempotencyKey("welcome-ann-1"))
if err != nil {
	log.Fatal(err)
}
fmt.Println(res.ID)
```

Every method takes a `context.Context` first. Clients are safe for concurrent use.

### Options

```go
client, _ := usend.New(key,
	usend.WithBaseURL("https://api.example.com"), // default https://api.usend.et (or USEND_BASE_URL)
	usend.WithHTTPClient(myHTTPClient),
	usend.WithTimeout(10*time.Second),            // per attempt, default 30s
	usend.WithMaxRetries(3),                      // default 2, 0 disables
)
```

Requests that fail with `429` or `5xx` are retried with exponential backoff, and `Retry-After` is honored (waits over 60 seconds are not attempted; the error is returned). A `429` is always retried. A `5xx` is retried only for requests that cannot cause a duplicate: GET, DELETE, and any request with an idempotency key.

## Resources

| Field | Methods |
|---|---|
| `client.Emails` | `Send`, `Batch`, `Get`, `List`, `Update` (reschedule), `Cancel`, `Events`, `Attachments`, `Attachment`, `Metrics`, `Share`, `SendTest`, `Insights` |
| `client.Domains` | `Create`, `List`, `Get`, `Update`, `Verify`, `AutoConfigure`, `Remove`, `Claim`, `GetClaim`, `VerifyClaim`, `Insights` |
| `client.APIKeys` | `Create`, `List`, `Update`, `Remove` |
| `client.Templates` | `Create`, `List`, `Get`, `Update`, `Publish`, `Duplicate`, `Remove`, `Versions`, `GetVersion`, `RestoreVersion` |
| `client.Contacts` | `Create`, `List`, `Get`, `Update`, `Remove`, `Import`, `CreateImport`, `GetImport`, `ListImports`, `Stats`, `Segments`, `AddToSegment`, `RemoveFromSegment`, `Topics`, `UpdateTopics` |
| `client.Segments`, `client.Audiences` | `Create`, `List`, `Get`, `Remove` (segments also `Update`, `Contacts`, `Metrics`; audiences also contact methods) |
| `client.Topics`, `client.Properties` | topics and contact properties: `List`, `Get`, `Create`, `Update`, `Remove` |
| `client.Broadcasts` | `Create`, `List`, `Get`, `Update`, `Send`, `Test`, `Remove`, `Cancel`, `Duplicate`, `Recipients`, `ClickedLinks`, `Performance` |
| `client.Events` | custom events: `Create`, `List`, `Get`, `Update`, `Remove`, `Send` |
| `client.Automations` | `Create`, `List`, `Get`, `Update`, `Remove`, `Duplicate`, `Stop`, `Runs`, `GetRun` |
| `client.Webhooks` | `Create`, `List`, `Get`, `Update`, `Remove`, `Deliveries`, `Replay`, `RotateSecret`, `Events`, `GetEvent`, `ReplayEvent`, `EventAttempts` |
| `client.Suppressions` | `List`, `Get`, `Create`, `BatchCreate`, `BatchRemove`, `Remove` |
| `client.Inbound` | `List`, `Get` (received mail) |
| `client.Inboxes` | inboxes, labels, threads, messages, `Reply`, `Forward`, drafts |
| `client.Stats`, `client.DMARCCheck` | `Stats.Get`; `DMARCCheck(ctx, domain)` |

`Broadcasts.Recipients(ctx, id, usend.RecipientClicked, params)` requires the event type. `Events.Send` fires a custom event (the API answers 202).

OAuth access tokens (`usa_...`) work as the API key. The OAuth endpoints themselves (`/oauth/*`) and session-only account calls (two-factor, billing, org settings) have no typed methods; use `Request`.

Paginated lists take `*usend.ListParams{Limit, After, Before}` (or nil) and return `*usend.List[T]` with `Data` and `HasMore`.

```go
batch, err := client.Emails.Batch(ctx, emails, usend.WithBatchValidation("permissive"))
```

### Anything else

`Request` calls any endpoint, typed or not:

```go
var out struct {
	Data []map[string]any `json:"data"`
}
err := client.Request(ctx, http.MethodGet, "/logs?days=7", nil, &out)
```

## SMTP

Send through usend over SMTP with your API key as the password. Port 465 is implicit TLS, 587 is STARTTLS. The host is `smtp.usend.et`, derived from the base URL.

```go
cfg, _ := client.SMTPConfig(587)
err := smtp.SendMail(cfg.Addr(), cfg.Auth(), "hello@acme.com", []string{"ann@example.com"}, msg)
// cfg.Host, cfg.Port, cfg.Username ("usend"), cfg.Password (the key), cfg.ImplicitTLS
```

`usend.NewSMTPConfig(key, baseURL, port)` works without a client. Set `USEND_SMTP_HOST` for a self-hosted platform whose hosts do not follow `api.` / `smtp.`.

## Webhooks

Verify the signature against the raw body (Standard Webhooks, secret `whsec_...`, 5 minutes tolerance by default):

```go
body, _ := io.ReadAll(r.Body)
event, err := usend.VerifyWebhook(body, r.Header, os.Getenv("USEND_WEBHOOK_SECRET"))
if err != nil {
	http.Error(w, "invalid signature", http.StatusBadRequest)
	return
}
switch event.Type {
case "email.delivered":
	var data struct {
		EmailID string `json:"email_id"`
	}
	json.Unmarshal(event.Data, &data)
}
```

Errors can be told apart with `errors.Is`: `ErrMissingWebhookHeaders`, `ErrWebhookTimestamp`, `ErrInvalidWebhookSecret`, `ErrInvalidSignature`. Use `usend.WithTolerance(d)` to change the clock tolerance.

## Errors

API errors are `*usend.Error` with the API's `StatusCode`, `Name` and `Message` (and `RetryAfter` when the server sent one):

```go
var apiErr *usend.Error
if errors.As(err, &apiErr) {
	switch apiErr.Name {
	case "validation_error":      // 422
	case "daily_quota_exceeded":  // 429
	}
}
```

Network failures and cancelled contexts are returned as ordinary errors.

## Examples

`examples/send`, `examples/webhook` and `examples/smtp`. Run with `go run ./examples/send`.

## Plain HTTP

The SDK is a thin layer over the REST API; you can always call it directly with `net/http` and `Authorization: Bearer us_...`. See the [API reference](https://usend.et/docs/api-reference/introduction).

## Development

```bash
cd sdk/go && go vet ./... && go test ./...
```

MIT licensed.
