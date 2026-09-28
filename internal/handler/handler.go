package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"LinkShrink/internal/service"
)

type linkService interface {
	Shorten(context.Context, string) (service.Link, bool, error)
	Resolve(context.Context, string) (service.Link, error)
}

type Handler struct {
	service linkService
	baseURL string
}

func New(s linkService, baseUrl string) *Handler { return &Handler{service: s, baseURL: baseUrl} }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	if r.URL.Path == "/" {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.create(w, r)
		return
	}
	code := strings.TrimPrefix(r.URL.Path, "/")
	if strings.Contains(code, "/") {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	link, err := h.service.Resolve(r.Context(), code)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		URL string `json:"url"`
	}{URL: link.URL})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	defer r.Body.Close()
	var request struct {
		URL string `json:"url"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeDecodeError(w, err)
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeDecodeError(w, err)
		return
	}
	link, created, err := h.service.Shorten(r.Context(), request.URL)
	if err != nil {
		writeError(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, struct {
		ShortURL string `json:"short_url"`
	}{ShortURL: h.baseURL + "/" + link.Code})
}

func writeDecodeError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		http.Error(w, "request body is too large", http.StatusRequestEntityTooLarge)
		return
	}
	http.Error(w, "expected one JSON object with a string field url", http.StatusBadRequest)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidURL), errors.Is(err, service.ErrInvalidCode):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, service.ErrNotFound):
		http.Error(w, "link not found", http.StatusNotFound)
	case errors.Is(err, context.DeadlineExceeded):
		http.Error(w, "operation timed out", http.StatusGatewayTimeout)
	case errors.Is(err, context.Canceled):
		http.Error(w, "request canceled", http.StatusRequestTimeout)
	default:
		log.Printf("request failed: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
