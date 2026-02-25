package cronbeatsgo

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"strings"
	"time"
)

type Options struct {
	BaseURL        string
	TimeoutMs      int
	MaxRetries     int
	RetryBackoffMs int
	RetryJitterMs  int
	UserAgent      string
	HTTPClient     HttpClient
}

type ProgressOptions struct {
	Seq     *int
	Message string
}

type PingSuccess struct {
	Ok               bool
	Action           string
	JobKey           string
	Timestamp        string
	ProcessingTimeMs float64
	NextExpected     *string
	Raw              map[string]any
}

type PingClient struct {
	baseURL        string
	jobKey         string
	timeoutMs      int
	maxRetries     int
	retryBackoffMs int
	retryJitterMs  int
	userAgent      string
	httpClient     HttpClient
	rng            *rand.Rand
	sleep          func(time.Duration)
}

var jobKeyRegex = regexp.MustCompile(`^[a-zA-Z0-9]{8}$`)

func NewPingClient(jobKey string, opts *Options) (*PingClient, error) {
	if !jobKeyRegex.MatchString(jobKey) {
		return nil, &ValidationError{Message: "jobKey must be exactly 8 Base62 characters."}
	}

	options := Options{}
	if opts != nil {
		options = *opts
	}

	baseURL := strings.TrimRight(defaultString(options.BaseURL, "https://cronbeats.io"), "/")
	timeoutMs := defaultInt(options.TimeoutMs, 5000)
	maxRetries := defaultInt(options.MaxRetries, 2)
	retryBackoffMs := defaultInt(options.RetryBackoffMs, 250)
	retryJitterMs := defaultInt(options.RetryJitterMs, 100)
	userAgent := defaultString(options.UserAgent, "cronbeats-go-sdk/0.1.0")

	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = &NetHTTPClient{}
	}

	return &PingClient{
		baseURL:        baseURL,
		jobKey:         jobKey,
		timeoutMs:      timeoutMs,
		maxRetries:     maxRetries,
		retryBackoffMs: retryBackoffMs,
		retryJitterMs:  retryJitterMs,
		userAgent:      userAgent,
		httpClient:     httpClient,
		rng:            rand.New(rand.NewSource(time.Now().UnixNano())),
		sleep:          time.Sleep,
	}, nil
}

func (c *PingClient) Ping() (*PingSuccess, error) {
	return c.request("ping", fmt.Sprintf("/ping/%s", c.jobKey), nil)
}

func (c *PingClient) Start() (*PingSuccess, error) {
	return c.request("start", fmt.Sprintf("/ping/%s/start", c.jobKey), nil)
}

func (c *PingClient) End(status string) (*PingSuccess, error) {
	statusValue := strings.ToLower(strings.TrimSpace(status))
	if statusValue == "" {
		statusValue = "success"
	}
	if statusValue != "success" && statusValue != "fail" {
		return nil, &ValidationError{Message: `Status must be "success" or "fail".`}
	}
	return c.request("end", fmt.Sprintf("/ping/%s/end/%s", c.jobKey, statusValue), nil)
}

func (c *PingClient) Success() (*PingSuccess, error) {
	return c.End("success")
}

func (c *PingClient) Fail() (*PingSuccess, error) {
	return c.End("fail")
}

func (c *PingClient) Progress(input any, message ...string) (*PingSuccess, error) {
	msg := ""
	if len(message) > 0 {
		msg = message[0]
	}

	seq := -1
	seqProvided := false

	switch v := input.(type) {
	case nil:
	case int:
		seq = v
		seqProvided = true
	case ProgressOptions:
		if v.Seq != nil {
			seq = *v.Seq
			seqProvided = true
		}
		if strings.TrimSpace(v.Message) != "" || msg == "" {
			msg = v.Message
		}
	case *ProgressOptions:
		if v != nil {
			if v.Seq != nil {
				seq = *v.Seq
				seqProvided = true
			}
			if strings.TrimSpace(v.Message) != "" || msg == "" {
				msg = v.Message
			}
		}
	default:
		return nil, &ValidationError{Message: "Progress input must be int, ProgressOptions, or nil."}
	}

	if seqProvided && seq < 0 {
		return nil, &ValidationError{Message: "Progress seq must be a non-negative integer."}
	}

	if len(msg) > 255 {
		msg = msg[:255]
	}

	body := map[string]any{"message": msg}
	if seqProvided {
		return c.request("progress", fmt.Sprintf("/ping/%s/progress/%d", c.jobKey, seq), body)
	}
	return c.request("progress", fmt.Sprintf("/ping/%s/progress", c.jobKey), body)
}

func (c *PingClient) request(action string, path string, body map[string]any) (*PingSuccess, error) {
	url := fmt.Sprintf("%s%s", c.baseURL, path)

	var payload []byte
	var err error
	if len(body) > 0 {
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, &SdkError{Message: "failed to encode request payload", Cause: err}
		}
	}

	attempt := 0
	for {
		res, reqErr := c.httpClient.Request(
			"POST",
			url,
			map[string]string{
				"Content-Type": "application/json",
				"Accept":       "application/json",
				"User-Agent":   c.userAgent,
			},
			payload,
			c.timeoutMs,
		)

		if reqErr != nil {
			if attempt >= c.maxRetries {
				return nil, &ApiError{
					Code:      CodeNetwork,
					Retryable: true,
					Message:   reqErr.Error(),
					Raw:       reqErr,
				}
			}
			attempt++
			c.sleepWithBackoff(attempt)
			continue
		}

		parsed := safeJSON(res.Body)
		if res.Status >= 200 && res.Status < 300 {
			return c.normalizeSuccess(action, parsed), nil
		}

		code, retryable := mapError(res.Status)
		msg, _ := parsed["message"].(string)
		if msg == "" {
			msg = "Request failed"
		}

		if retryable && attempt < c.maxRetries {
			attempt++
			c.sleepWithBackoff(attempt)
			continue
		}

		status := res.Status
		return nil, &ApiError{
			Code:       code,
			HTTPStatus: &status,
			Retryable:  retryable,
			Message:    msg,
			Raw:        parsed,
		}
	}
}

func (c *PingClient) normalizeSuccess(action string, payload map[string]any) *PingSuccess {
	outAction, _ := payload["action"].(string)
	if outAction == "" {
		outAction = action
	}

	outJobKey, _ := payload["job_key"].(string)
	if outJobKey == "" {
		outJobKey = c.jobKey
	}

	timestamp, _ := payload["timestamp"].(string)

	var nextExpected *string
	if rawNext, exists := payload["next_expected"]; exists && rawNext != nil {
		if v, ok := rawNext.(string); ok {
			nextExpected = &v
		}
	}

	return &PingSuccess{
		Ok:               true,
		Action:           outAction,
		JobKey:           outJobKey,
		Timestamp:        timestamp,
		ProcessingTimeMs: floatOrZero(payload["processing_time_ms"]),
		NextExpected:     nextExpected,
		Raw:              payload,
	}
}

func (c *PingClient) sleepWithBackoff(attempt int) {
	baseMs := float64(c.retryBackoffMs) * math.Pow(2, float64(maxInt(0, attempt-1)))
	jitter := 0
	if c.retryJitterMs > 0 {
		jitter = c.rng.Intn(c.retryJitterMs + 1)
	}
	waitMs := int(baseMs) + jitter
	c.sleep(time.Duration(waitMs) * time.Millisecond)
}

func mapError(status int) (ApiErrorCode, bool) {
	if status == 400 {
		return CodeValidation, false
	}
	if status == 404 {
		return CodeNotFound, false
	}
	if status == 429 {
		return CodeRateLimit, true
	}
	if status >= 500 {
		return CodeServer, true
	}
	return CodeUnknown, false
}

func safeJSON(raw string) map[string]any {
	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return map[string]any{"message": "Invalid JSON response"}
	}
	obj, ok := decoded.(map[string]any)
	if !ok {
		return map[string]any{"message": "Invalid JSON response"}
	}
	return obj
}

func floatOrZero(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case int32:
		return float64(x)
	case json.Number:
		f, err := x.Float64()
		if err == nil {
			return f
		}
	case string:
		var f float64
		_, err := fmt.Sscanf(x, "%f", &f)
		if err == nil {
			return f
		}
	}
	return 0
}

func defaultString(v string, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func defaultInt(v int, fallback int) int {
	if v == 0 {
		return fallback
	}
	return v
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
