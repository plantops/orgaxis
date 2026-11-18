package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAvatarAssetsAndHeroStory(t *testing.T) {
	s := testService(t)
	cases := []struct {
		path, content string
		status        int
	}{
		{"/", "text/html", 200},
		{"/app", "text/html", 200},
		{"/assets/avatars/employee-01.webp", "image/webp", 200},
		{"/assets/avatars/employee-15.webp", "image/webp", 200},
		{"/assets/illustrations/hero_warehouse_colleague.webp", "image/webp", 200},
		{"/assets/avatars/employee-99.webp", "text/plain", 404},
		{"/assets/avatars/../../seed.sqlite3", "text/html", 301},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		r := httptest.NewRecorder()
		s.routes().ServeHTTP(r, req)
		if r.Code != tc.status {
			t.Errorf("%s status=%d want=%d", tc.path, r.Code, tc.status)
			continue
		}
		if tc.status == 200 && !strings.HasPrefix(r.Header().Get("Content-Type"), tc.content) {
			t.Errorf("%s type=%s", tc.path, r.Header().Get("Content-Type"))
		}
		if tc.path == "/" && !strings.Contains(r.Body.String(), "hero_warehouse_colleague.webp") {
			t.Error("Missing hero staff illustration")
		}
	}
}
