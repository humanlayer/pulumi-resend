package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL    = "https://api.resend.com"
	defaultUserAgent  = "pulumi-resend/0.1.0-dev"
	defaultPageLimit  = 100
	defaultMaxRetries = 7
)

type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	sleep      func(context.Context, time.Duration) error
	maxRetries int
}

type APIError struct {
	StatusCode int
	Code       string
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("resend API error: status %d: %s (%s)", e.StatusCode, e.Message, e.Code)
	}
	return fmt.Sprintf("resend API error: status %d: %s", e.StatusCode, e.Message)
}

type ListResponse[T any] struct {
	Object  string `json:"object"`
	HasMore bool   `json:"has_more"`
	Data    []T    `json:"data"`
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		baseURL:    defaultBaseURL,
		httpClient: http.DefaultClient,
		sleep:      sleepContext,
		maxRetries: defaultMaxRetries,
	}
}

func (c *Client) Do(ctx context.Context, method, path string, body, result any) error {
	requestBody, err := encodeBody(body)
	if err != nil {
		return err
	}

	for attempt := 0; ; attempt++ {
		resp, err := c.doOnce(ctx, method, path, requestBody)
		if err != nil {
			return err
		}

		if resp.StatusCode == http.StatusTooManyRequests && attempt < c.maxRetries {
			delay := c.retryDelay(resp.Header.Get("Retry-After"), attempt)
			_ = resp.Body.Close()
			if err := c.sleep(ctx, delay); err != nil {
				return err
			}
			continue
		}

		return decodeResponse(resp, result)
	}
}

func ListAll[T any](ctx context.Context, c *Client, path string) ([]T, error) {
	var all []T
	after := ""

	for {
		pagePath, err := paginatedPath(path, defaultPageLimit, after)
		if err != nil {
			return nil, err
		}

		var page ListResponse[T]
		if err := c.Do(ctx, http.MethodGet, pagePath, nil, &page); err != nil {
			return nil, err
		}

		all = append(all, page.Data...)
		if !page.HasMore {
			return all, nil
		}
		if len(page.Data) == 0 {
			return nil, fmt.Errorf("resend list %q returned has_more without data", path)
		}

		after, err = itemID(page.Data[len(page.Data)-1])
		if err != nil {
			return nil, err
		}
	}
}

func encodeBody(body any) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode request body: %w", err)
	}
	return encoded, nil
}

func (c *Client) doOnce(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	requestURL, err := c.requestURL(path)
	if err != nil {
		return nil, err
	}

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL, reader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", defaultUserAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	return resp, nil
}

func (c *Client) requestURL(path string) (string, error) {
	base, err := url.Parse(strings.TrimRight(c.baseURL, "/") + "/")
	if err != nil {
		return "", fmt.Errorf("parse base URL: %w", err)
	}
	rel, err := url.Parse(strings.TrimLeft(path, "/"))
	if err != nil {
		return "", fmt.Errorf("parse request path: %w", err)
	}
	return base.ResolveReference(rel).String(), nil
}

func (c *Client) retryDelay(retryAfter string, attempt int) time.Duration {
	if delay, ok := parseRetryAfter(retryAfter); ok {
		return delay
	}
	return time.Second << attempt
}

func parseRetryAfter(value string) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	if seconds, err := strconv.ParseFloat(value, 64); err == nil {
		if seconds < 0 {
			seconds = 0
		}
		return time.Duration(seconds * float64(time.Second)), true
	}
	if when, err := http.ParseTime(value); err == nil {
		delay := time.Until(when)
		if delay < 0 {
			delay = 0
		}
		return delay, true
	}
	return 0, false
}

func decodeResponse(resp *http.Response, result any) error {
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return newAPIError(resp.StatusCode, data)
	}
	if result == nil || len(strings.TrimSpace(string(data))) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, result); err != nil {
		return fmt.Errorf("decode response body: %w", err)
	}
	return nil
}

func newAPIError(statusCode int, data []byte) error {
	parsed := struct {
		Code    string `json:"code"`
		Name    string `json:"name"`
		Message string `json:"message"`
		Error   *struct {
			Code    string `json:"code"`
			Name    string `json:"name"`
			Message string `json:"message"`
		} `json:"error"`
	}{}
	_ = json.Unmarshal(data, &parsed)

	code := parsed.Code
	message := parsed.Message
	if parsed.Name != "" && code == "" {
		code = parsed.Name
	}
	if parsed.Error != nil {
		if parsed.Error.Code != "" {
			code = parsed.Error.Code
		} else if parsed.Error.Name != "" {
			code = parsed.Error.Name
		}
		if parsed.Error.Message != "" {
			message = parsed.Error.Message
		}
	}
	if message == "" {
		message = strings.TrimSpace(string(data))
	}
	if message == "" {
		message = http.StatusText(statusCode)
	}

	return &APIError{
		StatusCode: statusCode,
		Code:       code,
		Message:    message,
		Body:       string(data),
	}
}

func paginatedPath(path string, limit int, after string) (string, error) {
	u, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("parse pagination path: %w", err)
	}
	query := u.Query()
	if query.Get("limit") == "" {
		query.Set("limit", strconv.Itoa(limit))
	}
	if after != "" {
		query.Set("after", after)
	}
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func itemID(item any) (string, error) {
	encoded, err := json.Marshal(item)
	if err != nil {
		return "", fmt.Errorf("encode pagination item: %w", err)
	}
	var decoded struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return "", fmt.Errorf("decode pagination item id: %w", err)
	}
	if decoded.ID == "" {
		return "", fmt.Errorf("pagination item has no id field")
	}
	return decoded.ID, nil
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
