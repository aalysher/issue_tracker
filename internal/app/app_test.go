package app

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHealthIsIndependentOfDatabase(t *testing.T) {
	handler := New(nil, slog.New(slog.NewTextHandler(io.Discard, nil)), false, time.Hour)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status body = %q, want ok", body["status"])
	}
}

func TestValidEmail(t *testing.T) {
	for _, test := range []struct {
		email string
		valid bool
	}{
		{email: "dev@example.com", valid: true},
		{email: "a@b.c", valid: false},
		{email: "missing-at.example.com", valid: false},
		{email: "a@@example.com", valid: false},
		{email: "@" + strings.Repeat("x", 254), valid: false},
	} {
		t.Run(test.email, func(t *testing.T) {
			if got := validEmail(test.email); got != test.valid {
				t.Fatalf("validEmail(%q) = %v, want %v", test.email, got, test.valid)
			}
		})
	}
}

func TestDecodeJSONRejectsUnknownAndTrailingFields(t *testing.T) {
	for _, body := range []string{`{"name":"test","extra":true}`, `{"name":"test"} {}`} {
		t.Run(body, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
			request.Header.Set("Content-Type", "application/json")
			var destination struct {
				Name string `json:"name"`
			}
			if decodeJSON(response, request, &destination) {
				t.Fatal("decodeJSON accepted invalid input")
			}
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestDecodeJSONRequiresJSONContentType(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"test"}`))
	var destination struct {
		Name string `json:"name"`
	}
	if decodeJSON(response, request, &destination) {
		t.Fatal("decodeJSON accepted a non-JSON request")
	}
	if response.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnsupportedMediaType)
	}
}

func TestDecodeJSONRejectsOversizedBody(t *testing.T) {
	response := httptest.NewRecorder()
	body := `{"name":"` + strings.Repeat("x", (1<<20)+1) + `"}`
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	var destination struct {
		Name string `json:"name"`
	}
	if decodeJSON(response, request, &destination) {
		t.Fatal("decodeJSON accepted an oversized request")
	}
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}
}
