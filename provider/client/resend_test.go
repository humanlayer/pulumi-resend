package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestDoSendsAuthUserAgentAndJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("Authorization"), "Bearer re_test"; got != want {
			t.Fatalf("Authorization header = %q, want %q", got, want)
		}
		if got := r.Header.Get("User-Agent"); got == "" {
			t.Fatal("User-Agent header is empty")
		}
		if got, want := r.Header.Get("Accept"), "application/json"; got != want {
			t.Fatalf("Accept header = %q, want %q", got, want)
		}
		if got, want := r.Header.Get("Content-Type"), "application/json"; got != want {
			t.Fatalf("Content-Type header = %q, want %q", got, want)
		}

		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if got, want := payload["subject"], "hello"; got != want {
			t.Fatalf("subject = %q, want %q", got, want)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"email_123"}`))
	}))
	t.Cleanup(server.Close)

	c := NewClient("re_test")
	c.baseURL = server.URL

	var result struct {
		ID string `json:"id"`
	}
	if err := c.Do(context.Background(), http.MethodPost, "/emails", map[string]string{"subject": "hello"}, &result); err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	if got, want := result.ID, "email_123"; got != want {
		t.Fatalf("result ID = %q, want %q", got, want)
	}
}

func TestDoRetriesRateLimitWithRetryAfter(t *testing.T) {
	t.Parallel()

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"message":"rate limited"}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(server.Close)

	var delays []time.Duration
	c := NewClient("re_test")
	c.baseURL = server.URL
	c.sleep = func(ctx context.Context, delay time.Duration) error {
		delays = append(delays, delay)
		return nil
	}

	var result struct {
		OK bool `json:"ok"`
	}
	if err := c.Do(context.Background(), http.MethodGet, "/domains", nil, &result); err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if !result.OK {
		t.Fatal("result OK = false, want true")
	}
	if got, want := delays, []time.Duration{0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("delays = %v, want %v", got, want)
	}
}

func TestDoRetriesRateLimitWithExponentialBackoff(t *testing.T) {
	t.Parallel()

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"message":"rate limited"}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(server.Close)

	var delays []time.Duration
	c := NewClient("re_test")
	c.baseURL = server.URL
	c.sleep = func(ctx context.Context, delay time.Duration) error {
		delays = append(delays, delay)
		return nil
	}

	var result struct {
		OK bool `json:"ok"`
	}
	if err := c.Do(context.Background(), http.MethodGet, "/domains", nil, &result); err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	if got, want := delays, []time.Duration{time.Second, 2 * time.Second}; !reflect.DeepEqual(got, want) {
		t.Fatalf("delays = %v, want %v", got, want)
	}
}

func TestDoReturnsAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"invalid request","code":"validation_error"}`))
	}))
	t.Cleanup(server.Close)

	c := NewClient("re_test")
	c.baseURL = server.URL

	err := c.Do(context.Background(), http.MethodGet, "/domains", nil, nil)
	if err == nil {
		t.Fatal("Do returned nil error, want APIError")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *APIError", err)
	}
	if got, want := apiErr.StatusCode, http.StatusBadRequest; got != want {
		t.Fatalf("status code = %d, want %d", got, want)
	}
	if got, want := apiErr.Code, "validation_error"; got != want {
		t.Fatalf("code = %q, want %q", got, want)
	}
	if got, want := apiErr.Message, "invalid request"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

func TestListAllPaginatesWithAfterCursor(t *testing.T) {
	t.Parallel()

	type item struct {
		ID string `json:"id"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Query().Get("limit"), "100"; got != want {
			t.Fatalf("limit = %q, want %q", got, want)
		}

		w.Header().Set("Content-Type", "application/json")
		switch after := r.URL.Query().Get("after"); after {
		case "":
			_, _ = w.Write([]byte(`{"object":"list","has_more":true,"data":[{"id":"item_1"},{"id":"item_2"}]}`))
		case "item_2":
			_, _ = w.Write([]byte(`{"object":"list","has_more":false,"data":[{"id":"item_3"}]}`))
		default:
			t.Fatalf("after = %q, want empty or item_2", after)
		}
	}))
	t.Cleanup(server.Close)

	c := NewClient("re_test")
	c.baseURL = server.URL

	items, err := ListAll[item](context.Background(), c, "/api-keys")
	if err != nil {
		t.Fatalf("ListAll returned error: %v", err)
	}
	want := []item{{ID: "item_1"}, {ID: "item_2"}, {ID: "item_3"}}
	if !reflect.DeepEqual(items, want) {
		t.Fatalf("items = %#v, want %#v", items, want)
	}
}
