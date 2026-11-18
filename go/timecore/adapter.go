package timecore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DeviceEvent is a transport DTO, NOT a trusted PersonID assignment. The
// backend must resolve BadgeID using a time-effective employee/badge mapping.
type DeviceEvent struct {
	EventID    string `json:"event_id"`
	BadgeID    string `json:"badge_id"`
	Direction  string `json:"direction"`
	OccurredAt string `json:"occurred_at"`
}
type DevicePage struct {
	Events     []DeviceEvent `json:"events"`
	NextCursor string        `json:"next_cursor"`
}

// HTTPPullReader reads a vendor-side JSON API. This is an adapter CONTRACT,
// not a claim of support for a specific model or proprietary vendor endpoint.
// Configuration must be administrator-controlled, not supplied per employee.
type HTTPPullReader struct {
	Endpoint    string
	BearerToken string
	Client      *http.Client
}

func (a HTTPPullReader) Fetch(ctx context.Context, cursor string) (DevicePage, error) {
	u, err := url.Parse(a.Endpoint)
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return DevicePage{}, errors.New("invalid device API endpoint")
	}
	// Plain HTTP is supported only for explicitly configured loopback lab fixtures.
	if u.Scheme == "http" && u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" {
		return DevicePage{}, errors.New("device API requires TLS outside loopback")
	}
	q := u.Query()
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return DevicePage{}, err
	}
	if a.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+a.BearerToken)
	}
	client := a.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	// A device integration should not follow redirects to unrelated hosts.
	copyClient := *client
	copyClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := copyClient.Do(req)
	if err != nil {
		return DevicePage{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return DevicePage{}, fmt.Errorf("device API HTTP %d", resp.StatusCode)
	}
	if !strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "application/json") {
		return DevicePage{}, errors.New("device API must return JSON")
	}
	var page DevicePage
	d := json.NewDecoder(io.LimitReader(resp.Body, 2<<20))
	d.DisallowUnknownFields()
	if err = d.Decode(&page); err != nil {
		return DevicePage{}, err
	}
	var extra any
	if err = d.Decode(&extra); err != io.EOF {
		return DevicePage{}, errors.New("device API must contain exactly one JSON document")
	}
	if len(page.Events) > 2000 {
		return DevicePage{}, errors.New("device API page over limit")
	}
	for _, v := range page.Events {
		if v.EventID == "" || v.BadgeID == "" {
			return DevicePage{}, errors.New("event is missing source identity")
		}
		if _, err := time.Parse(time.RFC3339Nano, v.OccurredAt); err != nil {
			return DevicePage{}, errors.New("device event timestamp missing RFC3339 timezone")
		}
	}
	return page, nil
}
