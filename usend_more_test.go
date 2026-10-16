package usend

import (
	"context"
	"net/http"
	"testing"
)

func TestBroadcastRecipientsRequiresTypeParam(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/broadcasts/b1/recipients" || q.Get("type") != "clicked" || q.Get("limit") != "5" || q.Get("email") != "ann" {
			t.Errorf("uri %s", r.URL.RequestURI())
		}
		w.Write([]byte(`{"object":"list","has_more":false,"data":[{"id":"e1","contact_id":"c1","email":"ann@x.com","count":2,"clicked_links":[{"url":"https://a.com","clicks":2}]}]}`))
	})
	l, err := c.Broadcasts.Recipients(context.Background(), "b1", RecipientClicked, &BroadcastRecipientsParams{ListParams: ListParams{Limit: 5}, Email: "ann"})
	if err != nil || l.Data[0].Count != 2 || l.Data[0].ClickedLinks[0].Clicks != 2 {
		t.Fatalf("%+v %v", l, err)
	}
}

func TestEventsSendAccepts202(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Idempotency-Key") != "k" || r.URL.Path != "/events/send" {
			t.Errorf("bad request")
		}
		w.WriteHeader(202)
		w.Write([]byte(`{"object":"event","event":"order.paid","id":"x1"}`))
	})
	res, err := c.Events.Send(context.Background(), &SendEventParams{Event: "order.paid", Email: "a@b.com", Payload: map[string]any{"total": 5}}, WithIdempotencyKey("k"))
	if err != nil || res.ID != "x1" || res.Event != "order.paid" {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestNewResourcePaths(t *testing.T) {
	var got []string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Method+" "+r.URL.RequestURI())
		w.Write([]byte(`{}`))
	})
	ctx := context.Background()
	c.Automations.Stop(ctx, "a1")
	c.Automations.Runs(ctx, "a1", "running", nil)
	c.Webhooks.RotateSecret(ctx, "w1")
	c.Webhooks.EventAttempts(ctx, "e1")
	c.Inboxes.Threads(ctx, "i1", &ThreadListParams{Folder: "inbox", Unread: true})
	c.Inboxes.Reply(ctx, "i1", "t1", "m1", &ReplyParams{Text: "hi"})
	c.Templates.RestoreVersion(ctx, "welcome", "v1")
	c.Suppressions.BatchRemove(ctx, []string{"a@b.com"})
	c.Domains.VerifyClaim(ctx, "d1")
	c.Contacts.CreateImport(ctx, &CreateContactImportParams{CSV: "email\na@b.com"})
	c.Emails.Insights(ctx, "e1")
	want := []string{
		"POST /automations/a1/stop",
		"GET /automations/a1/runs?status=running",
		"POST /webhooks/w1/rotate-secret",
		"GET /webhooks/events/e1/attempts",
		"GET /inboxes/i1/threads?folder=inbox&unread=true",
		"POST /inboxes/i1/threads/t1/emails/m1/reply",
		"POST /templates/welcome/versions/v1/restore",
		"DELETE /suppressions/batch",
		"POST /domains/claim/d1/verify",
		"POST /contacts/imports",
		"GET /emails/e1/insights",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d: got %q want %q", i, got[i], want[i])
		}
	}
}
