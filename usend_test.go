package usend

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient(t *testing.T, h http.HandlerFunc, opts ...Option) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := New("us_test", append([]Option{WithBaseURL(srv.URL)}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestNewRequiresKey(t *testing.T) {
	t.Setenv("USEND_API_KEY", "")
	if _, err := New(""); err == nil {
		t.Fatal("expected error without key")
	}
	t.Setenv("USEND_API_KEY", "us_env")
	if _, err := New(""); err != nil {
		t.Fatal(err)
	}
}

func TestSendEmail(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/emails" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer us_test" {
			t.Errorf("auth header %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Idempotency-Key") != "k1" {
			t.Errorf("idempotency key %q", r.Header.Get("Idempotency-Key"))
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["from"] != "a@acme.com" || body["subject"] != "Hi" {
			t.Errorf("body %v", body)
		}
		if _, ok := body["html"]; ok {
			t.Errorf("empty html should be omitted")
		}
		att := body["attachments"].([]any)[0].(map[string]any)
		if att["content"] != base64.StdEncoding.EncodeToString([]byte("hello")) {
			t.Errorf("attachment content %v", att["content"])
		}
		w.Write([]byte(`{"id":"e1"}`))
	})
	res, err := c.Emails.Send(context.Background(), &SendEmailParams{
		From: "a@acme.com", To: []string{"b@x.com"}, Subject: "Hi", Text: "yo",
		Attachments: []Attachment{{Filename: "a.txt", Content: []byte("hello")}},
	}, WithIdempotencyKey("k1"))
	if err != nil || res.ID != "e1" {
		t.Fatalf("res=%v err=%v", res, err)
	}
}

func TestAPIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		w.Write([]byte(`{"statusCode":422,"name":"validation_error","message":"The ` + "`to`" + ` field is required."}`))
	})
	_, err := c.Emails.Get(context.Background(), "x")
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("not *Error: %v", err)
	}
	if apiErr.StatusCode != 422 || apiErr.Name != "validation_error" || apiErr.Message == "" {
		t.Fatalf("%+v", apiErr)
	}
}

func TestNonJSONError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("<html>nope</html>"))
	})
	err := c.Request(context.Background(), "GET", "/x", nil, nil)
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Name != "application_error" || apiErr.StatusCode != 404 {
		t.Fatalf("%v", err)
	}
}

func TestRetryOn429HonorsRetryAfter(t *testing.T) {
	var calls int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			w.Write([]byte(`{"statusCode":429,"name":"rate_limit_exceeded","message":"slow down"}`))
			return
		}
		w.Write([]byte(`{"id":"e1"}`))
	})
	// POST without an idempotency key: 429 is still retried.
	res, err := c.Emails.Send(context.Background(), &SendEmailParams{From: "a@b.com", To: []string{"c@d.com"}, Text: "x"})
	if err != nil || res.ID != "e1" || calls != 2 {
		t.Fatalf("res=%v err=%v calls=%d", res, err, calls)
	}
}

func TestRetryAfterSecondsParsed(t *testing.T) {
	if d := parseRetryAfter("2"); d != 2*time.Second {
		t.Fatalf("got %v", d)
	}
	if d := parseRetryAfter(""); d != 0 {
		t.Fatalf("got %v", d)
	}
}

func TestRetry5xx(t *testing.T) {
	var calls int32
	h := func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(503)
			w.Write([]byte(`{"statusCode":503,"name":"internal_server_error","message":"x"}`))
			return
		}
		w.Write([]byte(`{"id":"e1"}`))
	}
	c := newTestClient(t, h)
	p := &SendEmailParams{From: "a@b.com", To: []string{"c@d.com"}, Text: "x"}

	// Without an idempotency key a POST is not retried after a 5xx.
	if _, err := c.Emails.Send(context.Background(), p); err == nil || calls != 1 {
		t.Fatalf("expected single failing call, err=%v calls=%d", err, calls)
	}
	// With one it is.
	calls = 0
	if _, err := c.Emails.Send(context.Background(), p, WithIdempotencyKey("k")); err != nil || calls != 2 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
	// GET is retried.
	calls = 0
	if _, err := c.Emails.Get(context.Background(), "x"); err != nil || calls != 2 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestNoRetriesWhenDisabled(t *testing.T) {
	var calls int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(429)
		w.Write([]byte(`{"statusCode":429,"name":"rate_limit_exceeded","message":"x"}`))
	}, WithMaxRetries(0))
	if err := c.Request(context.Background(), "GET", "/x", nil, nil); err == nil || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestContextCancelDuringBackoff(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(429)
		w.Write([]byte(`{"statusCode":429,"name":"rate_limit_exceeded","message":"x"}`))
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := c.Request(ctx, "GET", "/x", nil, nil)
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 5*time.Second {
		t.Fatalf("err=%v elapsed=%v", err, time.Since(start))
	}
}

func TestListQuery(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("limit") != "10" || q.Get("after") != "abc" || q.Get("before") != "" {
			t.Errorf("query %v", q)
		}
		w.Write([]byte(`{"object":"list","has_more":true,"data":[{"id":"e1","last_event":"delivered","created_at":"2026-10-01T10:00:00Z"}]}`))
	})
	l, err := c.Emails.List(context.Background(), &ListParams{Limit: 10, After: "abc"})
	if err != nil || !l.HasMore || len(l.Data) != 1 || l.Data[0].LastEvent != "delivered" {
		t.Fatalf("%+v %v", l, err)
	}
}

func TestBatchValidationHeader(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-batch-validation") != "permissive" {
			t.Errorf("header missing")
		}
		var body []map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if len(body) != 2 {
			t.Errorf("body %v", body)
		}
		w.Write([]byte(`{"data":[{"id":"1"}],"errors":[{"index":1,"message":"bad"}]}`))
	})
	res, err := c.Emails.Batch(context.Background(), []SendEmailParams{{From: "a@b.com"}, {From: "x"}}, WithBatchValidation("permissive"))
	if err != nil || len(res.Data) != 1 || res.Errors[0].Index != 1 {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestPathEscapingAndMethods(t *testing.T) {
	var gotPath, gotMethod string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.EscapedPath(), r.Method
		w.Write([]byte(`{}`))
	})
	ctx := context.Background()
	if err := c.Contacts.AddToSegment(ctx, "a+b@x.com", "s1"); err != nil {
		t.Fatal(err)
	}
	if gotMethod != "POST" || gotPath != "/contacts/a+b@x.com/segments/s1" {
		t.Fatalf("%s %s", gotMethod, gotPath)
	}
	if _, err := c.Emails.Cancel(ctx, "e 1"); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/emails/e%201/cancel" {
		t.Fatalf("%s", gotPath)
	}
}

func TestContactsListFilters(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("segment_id") != "s1" || q.Get("subscription") != "subscribed" || q.Get("limit") != "5" {
			t.Errorf("query %v", q)
		}
		w.Write([]byte(`{"object":"list","data":[]}`))
	})
	_, err := c.Contacts.List(context.Background(), &ListContactsParams{ListParams: ListParams{Limit: 5}, SegmentID: "s1", Subscription: "subscribed"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestContactTopicsEnvelopeOrArray(t *testing.T) {
	for _, body := range []string{`[{"id":"t1","subscription":"opt_in"}]`, `{"data":[{"id":"t1","subscription":"opt_in"}]}`} {
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) })
		topics, err := c.Contacts.Topics(context.Background(), "a@b.com")
		if err != nil || len(topics) != 1 || topics[0].ID != "t1" {
			t.Fatalf("%s: %v %v", body, topics, err)
		}
	}
}

func TestRequestEscapeHatch(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RequestURI() != "/logs?days=7" {
			t.Errorf("uri %s", r.URL.RequestURI())
		}
		w.Write([]byte(`{"data":[1]}`))
	})
	var out struct{ Data []int }
	if err := c.Request(context.Background(), "GET", "/logs?days=7", nil, &out); err != nil || len(out.Data) != 1 {
		t.Fatalf("%v %v", out, err)
	}
}

func TestSMTPConfig(t *testing.T) {
	t.Setenv("USEND_SMTP_HOST", "")
	cfg, err := NewSMTPConfig("us_k", "", 0)
	if err != nil || cfg.Host != "smtp.usend.et" || cfg.Port != 465 || !cfg.ImplicitTLS || cfg.Username != "usend" || cfg.Password != "us_k" {
		t.Fatalf("%+v %v", cfg, err)
	}
	cfg, _ = NewSMTPConfig("us_k", "https://api.example.com", 587)
	if cfg.Host != "smtp.example.com" || cfg.ImplicitTLS || cfg.Addr() != "smtp.example.com:587" {
		t.Fatalf("%+v", cfg)
	}
	if _, err := NewSMTPConfig("us_k", "", 25); err == nil {
		t.Fatal("port 25 should be refused")
	}
	c, _ := New("us_k")
	if cfg, _ := c.SMTPConfig(587); cfg.Host != "smtp.usend.et" || cfg.Port != 587 {
		t.Fatalf("%+v", cfg)
	}
}

// ---------------------------------------------------------------- webhooks

func signed(secretB64, id string, ts time.Time, body string) http.Header {
	key, _ := base64.StdEncoding.DecodeString(secretB64)
	m := hmac.New(sha256.New, key)
	tsv := strconv.FormatInt(ts.Unix(), 10)
	m.Write([]byte(id + "." + tsv + "." + body))
	h := http.Header{}
	h.Set("webhook-id", id)
	h.Set("webhook-timestamp", tsv)
	h.Set("webhook-signature", "v1,"+base64.StdEncoding.EncodeToString(m.Sum(nil)))
	return h
}

func TestVerifyWebhook(t *testing.T) {
	secretB64 := base64.StdEncoding.EncodeToString([]byte("super-secret-key-bytes"))
	secret := "whsec_" + secretB64
	body := `{"type":"email.delivered","created_at":"2026-10-01T10:00:00Z","data":{"email_id":"e1"}}`

	ev, err := VerifyWebhook([]byte(body), signed(secretB64, "msg_1", time.Now(), body), secret)
	if err != nil || ev.Type != "email.delivered" || ev.ID != "msg_1" {
		t.Fatalf("%+v %v", ev, err)
	}
	var data struct {
		EmailID string `json:"email_id"`
	}
	json.Unmarshal(ev.Data, &data)
	if data.EmailID != "e1" {
		t.Fatalf("data %+v", data)
	}

	// svix-* headers work too.
	h := signed(secretB64, "msg_1", time.Now(), body)
	s := http.Header{}
	s.Set("svix-id", h.Get("webhook-id"))
	s.Set("svix-timestamp", h.Get("webhook-timestamp"))
	s.Set("svix-signature", h.Get("webhook-signature"))
	if _, err := VerifyWebhook([]byte(body), s, secret); err != nil {
		t.Fatal(err)
	}

	// A second, valid signature after a bogus one is accepted.
	h = signed(secretB64, "msg_1", time.Now(), body)
	h.Set("webhook-signature", "v1,AAAA "+h.Get("webhook-signature"))
	if _, err := VerifyWebhook([]byte(body), h, secret); err != nil {
		t.Fatal(err)
	}

	// Tampered body.
	if _, err := VerifyWebhook([]byte(body+" "), signed(secretB64, "msg_1", time.Now(), body), secret); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("tampered: %v", err)
	}
	// Wrong secret.
	other := "whsec_" + base64.StdEncoding.EncodeToString([]byte("another-secret"))
	if _, err := VerifyWebhook([]byte(body), signed(secretB64, "msg_1", time.Now(), body), other); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("wrong secret: %v", err)
	}
	// Old timestamp, then widened tolerance.
	old := signed(secretB64, "msg_1", time.Now().Add(-10*time.Minute), body)
	if _, err := VerifyWebhook([]byte(body), old, secret); !errors.Is(err, ErrWebhookTimestamp) {
		t.Fatalf("old: %v", err)
	}
	if _, err := VerifyWebhook([]byte(body), old, secret, WithTolerance(time.Hour)); err != nil {
		t.Fatalf("tolerance: %v", err)
	}
	// Missing headers and bad secret.
	if _, err := VerifyWebhook([]byte(body), http.Header{}, secret); !errors.Is(err, ErrMissingWebhookHeaders) {
		t.Fatalf("missing: %v", err)
	}
	if _, err := VerifyWebhook([]byte(body), signed(secretB64, "m", time.Now(), body), "whsec_***"); !errors.Is(err, ErrInvalidWebhookSecret) {
		t.Fatalf("secret: %v", err)
	}
}

// Known-answer test: pins the algorithm to the documented formula, computed independently.
func TestVerifyWebhookKnownAnswer(t *testing.T) {
	// secret "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw" is the Standard Webhooks example secret.
	secret := "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw"
	key, _ := base64.StdEncoding.DecodeString("MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw")
	m := hmac.New(sha256.New, key)
	m.Write([]byte(`msg_p5jXN8AQM9LWM0D4loKWxJek.1614265330.{"test": 2432232314}`))
	h := http.Header{}
	h.Set("webhook-id", "msg_p5jXN8AQM9LWM0D4loKWxJek")
	h.Set("webhook-timestamp", "1614265330")
	h.Set("webhook-signature", "v1,"+base64.StdEncoding.EncodeToString(m.Sum(nil)))
	now := func() time.Time { return time.Unix(1614265330, 0) }
	cfg := func(c *verifyConfig) { c.now = now }
	if _, err := VerifyWebhook([]byte(`{"test": 2432232314}`), h, secret, cfg); err != nil {
		t.Fatal(err)
	}
}
