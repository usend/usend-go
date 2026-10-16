package usend

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestShortLinksPaths(t *testing.T) {
	var got []string
	var bodies []string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = append(got, r.Method+" "+r.URL.RequestURI())
		bodies = append(bodies, string(b))
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodDelete:
			_, _ = w.Write([]byte(`{"object":"short_link","id":"l1","deleted":true}`))
		case r.URL.Path == "/short-links" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"l1","code":"abc"}],"has_more":false}`))
		case r.URL.Path == "/short-links/l1/stats":
			_, _ = w.Write([]byte(`{"object":"short_link_stats","id":"l1","days":7,"totals":{"clicks":5,"human_clicks":3}}`))
		default:
			_, _ = w.Write([]byte(`{"object":"short_link","id":"l1","code":"abc","short_url":"https://t.example/s/abc"}`))
		}
	})
	ctx := context.Background()
	l, err := c.ShortLinks.Create(ctx, &CreateShortLinkParams{TargetURL: "https://example.com", Code: "abc"})
	if err != nil || l.ShortURL == "" {
		t.Fatalf("create: %v %+v", err, l)
	}
	page, err := c.ShortLinks.List(ctx, &ListParams{Limit: 5})
	if err != nil || len(page.Data) != 1 {
		t.Fatalf("list: %v", err)
	}
	if _, err = c.ShortLinks.Get(ctx, "l1"); err != nil {
		t.Fatal(err)
	}
	off := true
	if _, err = c.ShortLinks.Update(ctx, "l1", &UpdateShortLinkParams{Disabled: &off}); err != nil {
		t.Fatal(err)
	}
	if _, err = c.ShortLinks.Update(ctx, "l1", &UpdateShortLinkParams{ClearExpiry: true}); err != nil {
		t.Fatal(err)
	}
	st, err := c.ShortLinks.Stats(ctx, "l1", 7)
	if err != nil || st.Totals.HumanClicks != 3 {
		t.Fatalf("stats: %v %+v", err, st)
	}
	if d, err := c.ShortLinks.Remove(ctx, "l1"); err != nil || !d.Deleted {
		t.Fatalf("remove: %v", err)
	}
	want := []string{"POST /short-links", "GET /short-links?limit=5", "GET /short-links/l1", "PATCH /short-links/l1", "PATCH /short-links/l1", "GET /short-links/l1/stats?days=7", "DELETE /short-links/l1"}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("call %d = %q, want %q", i, got[i], w)
		}
	}
	var clear map[string]any
	_ = json.Unmarshal([]byte(bodies[4]), &clear)
	if v, ok := clear["expires_at"]; !ok || v != nil {
		t.Errorf("clearing the expiry must send expires_at:null, sent %s", bodies[4])
	}
}

func TestLogsList(t *testing.T) {
	var uri string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		uri = r.URL.RequestURI()
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"x","method":"POST","status":200}],"has_more":false}`))
	})
	res, err := c.Logs.List(context.Background(), &ListLogsParams{Status: "500", Days: 2})
	if err != nil || len(res.Data) != 1 || res.Data[0].Method != "POST" {
		t.Fatalf("%v %+v", err, res)
	}
	if uri != "/logs?days=2&status=500" {
		t.Errorf("uri = %s", uri)
	}
}
