package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"LinkShrink/internal/repository"
	"LinkShrink/internal/service"
)

func testHandler() http.Handler {
	return New(service.New(repository.NewMemory()), "http://localhost:8080")
}

func request(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestCreateAndGet(t *testing.T) {
	h := testHandler()
	const original = "https://example.org/a?x=1#part"
	body := `{"url":"` + original + `"}`
	w := request(h, "POST", "/", body)
	if w.Code != 201 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var created map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	shortURL := created["short_url"]
	parsedURL, err := url.Parse(shortURL)
	if err != nil {
		t.Fatalf("invalid short URL: %v", err)
	}
	code := strings.TrimPrefix(parsedURL.Path, "/")
	if len(created) != 1 || !service.ValidCode(code) {
		t.Fatalf("unexpected response: %v", created)
	}
	if parsedURL.Scheme != "http" {
		t.Fatalf("unexpected scheme: %q", parsedURL.Scheme)
	}

	if parsedURL.Host != "localhost:8080" {
		t.Fatalf("unexpected host: %q", parsedURL.Host)
	}
	w = request(h, "POST", "/", body)
	if w.Code != 200 {
		t.Fatalf("duplicate: %d %s", w.Code, w.Body)
	}
	var duplicate map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &duplicate); err != nil {
		t.Fatal(err)
	}
	if duplicate["short_url"] != shortURL {
		t.Fatal("duplicate URL got another code")
	}
	w = request(h, "GET", "/"+code, "")
	if w.Code != 200 || w.Header().Get("Location") != "" {
		t.Fatalf("GET must return the URL, not redirect: %d", w.Code)
	}
	var resolved map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resolved); err != nil {
		t.Fatal(err)
	}
	if len(resolved) != 1 || resolved["url"] != original {
		t.Fatalf("unexpected response: %v", resolved)
	}
}

func TestHTTPValidation(t *testing.T) {
	tests := []struct {
		name, method, path, body string
		status                   int
	}{
		{"empty", "POST", "/", "", 400},
		{"malformed", "POST", "/", "{", 400},
		{"unknown field", "POST", "/", `{"url":"https://example.org","extra":1}`, 400},
		{"wrong type", "POST", "/", `{"url":42}`, 400},
		{"two objects", "POST", "/", `{"url":"https://example.org"}{}`, 400},
		{"trailing null", "POST", "/", `{"url":"https://example.org"}null`, 400},
		{"null", "POST", "/", `null`, 400},
		{"missing URL", "POST", "/", `{}`, 400},
		{"invalid URL", "POST", "/", `{"url":"not-a-url"}`, 400},
		{"body too large", "POST", "/", `{"url":"https://example.org/` + strings.Repeat("a", 16<<10) + `"}`, 413},
		{"trailing oversized whitespace", "POST", "/", `{"url":"https://example.org"}` + strings.Repeat(" ", 16<<10), 413},
		{"bad code", "GET", "/short", "", 400},
		{"not found", "GET", "/zzzzzzzzzz", "", 404},
		{"wrong create method", "GET", "/", "", 405},
		{"wrong get method", "DELETE", "/0123456789", "", 405},
		{"head not provided", "HEAD", "/0123456789", "", 405},
		{"unknown route", "GET", "/unknown/path", "", 404},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := request(testHandler(), tt.method, tt.path, tt.body)
			if w.Code != tt.status {
				t.Fatalf("got %d %s, want %d", w.Code, w.Body, tt.status)
			}
			if tt.status == 405 && w.Header().Get("Allow") == "" {
				t.Fatal("missing Allow header")
			}
		})
	}
}

type failedService struct{ err error }

func (s failedService) Shorten(context.Context, string) (service.Link, bool, error) {
	return service.Link{}, false, s.err
}
func (s failedService) Resolve(context.Context, string) (service.Link, error) {
	return service.Link{}, s.err
}

func TestErrors(t *testing.T) {
	for _, tt := range []struct {
		err    error
		status int
	}{
		{service.ErrGenerationExhausted, 500}, {context.DeadlineExceeded, 504},
		{context.Canceled, 408}, {errors.New("private database details"), 500},
	} {
		h := New(failedService{err: tt.err}, "http://localhost:8080")
		for _, w := range []*httptest.ResponseRecorder{
			request(h, "GET", "/0123456789", ""),
			request(h, "POST", "/", `{"url":"https://example.org"}`),
		} {
			if w.Code != tt.status || strings.Contains(w.Body.String(), "private") {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
		}
	}
}

func TestConcurrentCreate(t *testing.T) {
	h := testHandler()
	var wg sync.WaitGroup
	var created atomic.Int32
	codes := make(chan string, 500)
	for range 500 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := request(h, "POST", "/", `{"url":"https://example.org/concurrent"}`)
			if w.Code == 201 {
				created.Add(1)
			} else if w.Code != 200 {
				t.Errorf("%d %s", w.Code, w.Body)
				return
			}
			var response struct {
				ShortURL string `json:"short_url"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Error(err)
				return
			}
			codes <- response.ShortURL
		}()
	}
	wg.Wait()
	close(codes)
	if created.Load() != 1 {
		t.Fatalf("created=%d", created.Load())
	}
	first := ""
	for code := range codes {
		if first == "" {
			first = code
		}
		if first != code {
			t.Fatal("same URL got different codes")
		}
	}
}
