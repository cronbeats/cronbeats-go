package cronbeatsgo

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type stubResponse struct {
	status int
	body   string
}

type stubCall struct {
	method string
	url    string
	body   string
}

type stubHTTPClient struct {
	responses       []stubResponse
	networkFailures int
	calls           []stubCall
}

func (s *stubHTTPClient) Request(method string, url string, _ map[string]string, body []byte, _ int) (*HttpResponse, error) {
	s.calls = append(s.calls, stubCall{
		method: method,
		url:    url,
		body:   string(body),
	})

	if s.networkFailures > 0 {
		s.networkFailures--
		return nil, &SdkError{Message: "socket timeout", Cause: errors.New("timeout")}
	}

	if len(s.responses) == 0 {
		return &HttpResponse{Status: 200, Body: `{}`, Headers: map[string]string{}}, nil
	}

	next := s.responses[0]
	s.responses = s.responses[1:]
	return &HttpResponse{Status: next.status, Body: next.body, Headers: map[string]string{}}, nil
}

func newTestClient(t *testing.T, httpClient HttpClient, opts *Options) *PingClient {
	t.Helper()
	base := &Options{
		HTTPClient:     httpClient,
		MaxRetries:     0,
		RetryBackoffMs: 1,
		RetryJitterMs:  0,
	}
	if opts != nil {
		if opts.BaseURL != "" {
			base.BaseURL = opts.BaseURL
		}
		if opts.TimeoutMs != 0 {
			base.TimeoutMs = opts.TimeoutMs
		}
		if opts.MaxRetries != 0 {
			base.MaxRetries = opts.MaxRetries
		}
		if opts.RetryBackoffMs != 0 {
			base.RetryBackoffMs = opts.RetryBackoffMs
		}
		base.RetryJitterMs = opts.RetryJitterMs
		if opts.UserAgent != "" {
			base.UserAgent = opts.UserAgent
		}
	}

	client, err := NewPingClient("abc123de", base)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	client.sleep = func(time.Duration) {}
	return client
}

func TestInvalidJobKey(t *testing.T) {
	_, err := NewPingClient("invalid-key", nil)
	if err == nil {
		t.Fatal("expected validation error for invalid job key")
	}
	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestSuccessResponseNormalized(t *testing.T) {
	http := &stubHTTPClient{
		responses: []stubResponse{
			{
				status: 200,
				body:   `{"status":"success","message":"OK","action":"ping","job_key":"abc123de","timestamp":"2026-02-25 12:00:00","processing_time_ms":8.25}`,
			},
		},
	}
	client := newTestClient(t, http, nil)
	res, err := client.Ping()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Ok || res.Action != "ping" || res.JobKey != "abc123de" || res.ProcessingTimeMs != 8.25 {
		t.Fatalf("unexpected normalized payload: %#v", res)
	}
}

func Test404MapsToNotFound(t *testing.T) {
	http := &stubHTTPClient{
		responses: []stubResponse{
			{status: 404, body: `{"status":"error","message":"Job not found"}`},
		},
	}
	client := newTestClient(t, http, nil)
	_, err := client.Ping()
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *ApiError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected ApiError, got %T", err)
	}
	if apiErr.Code != CodeNotFound || apiErr.Retryable || apiErr.HTTPStatus == nil || *apiErr.HTTPStatus != 404 {
		t.Fatalf("unexpected api error: %#v", apiErr)
	}
}

func TestRetryOn429ThenSuccess(t *testing.T) {
	http := &stubHTTPClient{
		responses: []stubResponse{
			{status: 429, body: `{"status":"error","message":"Too many requests"}`},
			{
				status: 200,
				body:   `{"status":"success","message":"OK","action":"ping","job_key":"abc123de","timestamp":"2026-02-25 12:00:00","processing_time_ms":7.1}`,
			},
		},
	}
	client := newTestClient(t, http, &Options{MaxRetries: 2})
	res, err := client.Ping()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Ok {
		t.Fatalf("expected ok response, got %#v", res)
	}
	if len(http.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(http.calls))
	}
}

func TestNoRetryOn400(t *testing.T) {
	http := &stubHTTPClient{
		responses: []stubResponse{
			{status: 400, body: `{"status":"error","message":"Invalid request"}`},
		},
	}
	client := newTestClient(t, http, &Options{MaxRetries: 2})
	_, err := client.Ping()
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *ApiError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected ApiError, got %T", err)
	}
	if apiErr.Code != CodeValidation || apiErr.Retryable {
		t.Fatalf("unexpected api error: %#v", apiErr)
	}
	if len(http.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(http.calls))
	}
}

func TestRetryOnNetworkThenSuccess(t *testing.T) {
	http := &stubHTTPClient{
		networkFailures: 1,
		responses: []stubResponse{
			{
				status: 200,
				body:   `{"status":"success","message":"OK","action":"ping","job_key":"abc123de","timestamp":"2026-02-25 12:00:00","processing_time_ms":3.3}`,
			},
		},
	}
	client := newTestClient(t, http, &Options{MaxRetries: 2})
	res, err := client.Ping()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Ok {
		t.Fatalf("expected ok response, got %#v", res)
	}
	if len(http.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(http.calls))
	}
}

func TestProgressTruncationAndSeqPath(t *testing.T) {
	http := &stubHTTPClient{
		responses: []stubResponse{
			{
				status: 200,
				body:   `{"status":"success","message":"OK","action":"progress","job_key":"abc123de","timestamp":"2026-02-25 12:00:00","processing_time_ms":8}`,
			},
		},
	}
	client := newTestClient(t, http, nil)

	seq := 50
	longMsg := make([]byte, 300)
	for i := range longMsg {
		longMsg[i] = 'x'
	}

	_, err := client.Progress(ProgressOptions{Seq: &seq, Message: string(longMsg)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(http.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(http.calls))
	}
	if got := http.calls[0].url; got != "https://cronbeats.io/ping/abc123de/progress/50" {
		t.Fatalf("unexpected url: %s", got)
	}

	var sent map[string]any
	if err := json.Unmarshal([]byte(http.calls[0].body), &sent); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}
	msg, _ := sent["message"].(string)
	if len(msg) != 255 {
		t.Fatalf("expected truncated message length 255, got %d", len(msg))
	}
}
