package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/example/agent-issue-tracker/internal/database"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPostgresWorkflowAndOwnership(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(5)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(New(db, slog.New(slog.NewTextHandler(io.Discard, nil)), false, time.Hour))
	defer server.Close()
	first := newTestClient(t)
	second := newTestClient(t)
	email := uuid.NewString() + "@example.test"

	status, _, err := first.request(server.URL, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"email": email, "password": "correct horse battery staple",
	})
	if err != nil || status != http.StatusCreated {
		t.Fatalf("register: status=%d err=%v", status, err)
	}
	status, _, err = first.request(server.URL, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"email": email, "password": "correct horse battery staple",
	})
	if err != nil || status != http.StatusConflict {
		t.Fatalf("duplicate registration: status=%d err=%v", status, err)
	}
	status, _, err = first.request(server.URL, http.MethodPost, "/api/v1/projects", map[string]string{"name": "Integration"})
	if err != nil || status != http.StatusCreated {
		t.Fatalf("create project: status=%d err=%v", status, err)
	}
	status, body, err := first.request(server.URL, http.MethodGet, "/api/v1/projects", nil)
	var projects []project
	if err == nil {
		err = json.Unmarshal(body, &projects)
	}
	if err != nil || status != http.StatusOK || len(projects) != 1 {
		t.Fatalf("list projects: status=%d count=%d err=%v", status, len(projects), err)
	}

	status, _, err = first.request(server.URL, http.MethodPost, "/api/v1/projects/"+projects[0].ID+"/issues", map[string]string{
		"title": "Verify permissions",
	})
	if err != nil || status != http.StatusCreated {
		t.Fatalf("create issue: status=%d err=%v", status, err)
	}
	status, body, err = first.request(server.URL, http.MethodGet, "/api/v1/projects/"+projects[0].ID+"/issues", nil)
	var issues []issue
	if err == nil {
		err = json.Unmarshal(body, &issues)
	}
	if err != nil || status != http.StatusOK || len(issues) != 1 {
		t.Fatalf("list issues: status=%d count=%d err=%v", status, len(issues), err)
	}
	status, _, err = first.request(server.URL, http.MethodPatch, "/api/v1/issues/"+issues[0].ID, map[string]string{"status": "in_progress"})
	if err != nil || status != http.StatusOK {
		t.Fatalf("update issue status: status=%d err=%v", status, err)
	}
	status, _, err = first.request(server.URL, http.MethodPost, "/api/v1/issues/"+issues[0].ID+"/comments", map[string]string{"body": "Acceptance criteria verified"})
	if err != nil || status != http.StatusCreated {
		t.Fatalf("create comment: status=%d err=%v", status, err)
	}
	status, body, err = first.request(server.URL, http.MethodGet, "/api/v1/issues/"+issues[0].ID+"/comments", nil)
	var comments []comment
	if err == nil {
		err = json.Unmarshal(body, &comments)
	}
	if err != nil || status != http.StatusOK || len(comments) != 1 || comments[0].Body != "Acceptance criteria verified" {
		t.Fatalf("list comments: status=%d count=%d err=%v", status, len(comments), err)
	}

	status, _, err = first.requestWithoutCSRF(server.URL, http.MethodPost, "/api/v1/projects", map[string]string{"name": "Blocked"})
	if err != nil || status != http.StatusForbidden {
		t.Fatalf("write without CSRF header: status=%d err=%v", status, err)
	}
	status, _, err = second.request(server.URL, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"email": uuid.NewString() + "@example.test", "password": "correct horse battery staple",
	})
	if err != nil || status != http.StatusCreated {
		t.Fatalf("register second user: status=%d err=%v", status, err)
	}
	status, _, err = second.request(server.URL, http.MethodPatch, "/api/v1/issues/"+issues[0].ID, map[string]string{"status": "done"})
	if err != nil || status != http.StatusNotFound {
		t.Fatalf("update another user's issue: status=%d err=%v", status, err)
	}
	status, _, err = first.request(server.URL, http.MethodPost, "/api/v1/auth/logout", map[string]string{})
	if err != nil || status != http.StatusNoContent {
		t.Fatalf("logout: status=%d err=%v", status, err)
	}
	status, _, err = first.request(server.URL, http.MethodGet, "/api/v1/me", nil)
	if err != nil || status != http.StatusUnauthorized {
		t.Fatalf("revoked session: status=%d err=%v", status, err)
	}
}

type testClient struct {
	client *http.Client
}

func newTestClient(t *testing.T) testClient {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return testClient{client: &http.Client{Jar: jar, Timeout: 5 * time.Second}}
}

func (c testClient) request(baseURL, method, path string, body any) (int, json.RawMessage, error) {
	return c.doRequest(baseURL, method, path, body, true)
}

func (c testClient) requestWithoutCSRF(baseURL, method, path string, body any) (int, json.RawMessage, error) {
	return c.doRequest(baseURL, method, path, body, false)
}

func (c testClient) doRequest(baseURL, method, path string, body any, csrf bool) (int, json.RawMessage, error) {
	var encoded []byte
	var err error
	if body != nil {
		encoded, err = json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
	}
	request, err := http.NewRequest(method, baseURL+path, bytes.NewReader(encoded))
	if err != nil {
		return 0, nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	if method != http.MethodGet && csrf {
		request.Header.Set("X-Requested-With", "XMLHttpRequest")
	}
	response, err := c.client.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	result, err := io.ReadAll(response.Body)
	return response.StatusCode, result, err
}
