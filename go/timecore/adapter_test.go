package timecore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPullDeviceAdapterCursor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Query().Get("cursor") != "next-001" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("wrong request %v", r.URL)
			w.WriteHeader(400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"events":[{"event_id":"9","badge_id":"EMP-0001","direction":"IN","occurred_at":"2026-11-18T07:56:00+07:00"}],"next_cursor":"next-002"}`))
	}))
	defer server.Close()
	page, err := (HTTPPullReader{Endpoint: server.URL + "/events", BearerToken: "secret"}).Fetch(context.Background(), "next-001")
	if err != nil || len(page.Events) != 1 || page.NextCursor != "next-002" {
		t.Fatalf("page=%+v err=%v", page, err)
	}
}
func TestPullRejectsInsecureEndpoint(t *testing.T) {
	_, err := (HTTPPullReader{Endpoint: "http://192.168.1.25/device/events"}).Fetch(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "TLS") {
		t.Fatalf("expected TLS restriction: %v", err)
	}
}
