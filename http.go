package cronbeatsgo

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"
)

type HttpResponse struct {
	Status  int
	Body    string
	Headers map[string]string
}

type HttpClient interface {
	Request(method string, url string, headers map[string]string, body []byte, timeoutMs int) (*HttpResponse, error)
}

type NetHTTPClient struct{}

func (c *NetHTTPClient) Request(method string, url string, headers map[string]string, body []byte, timeoutMs int) (*HttpResponse, error) {
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, &SdkError{Message: "failed to create request", Cause: err}
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{Timeout: time.Duration(timeoutMs) * time.Millisecond}
	res, err := client.Do(req)
	if err != nil {
		return nil, &SdkError{Message: "network request failed", Cause: err}
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, &SdkError{Message: "failed to read response body", Cause: err}
	}

	outHeaders := make(map[string]string, len(res.Header))
	for key, values := range res.Header {
		outHeaders[strings.ToLower(key)] = strings.Join(values, ",")
	}

	return &HttpResponse{
		Status:  res.StatusCode,
		Body:    string(raw),
		Headers: outHeaders,
	}, nil
}
