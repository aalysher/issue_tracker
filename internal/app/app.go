package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

const sessionCookie = "tracker_session"

type App struct {
	db           *sql.DB
	logger       *slog.Logger
	secureCookie bool
	sessionTTL   time.Duration
}

func New(db *sql.DB, logger *slog.Logger, secureCookie bool, sessionTTL time.Duration) http.Handler {
	a := &App{db: db, logger: logger, secureCookie: secureCookie, sessionTTL: sessionTTL}
	r := chi.NewRouter()
	r.Use(a.requestLogger)
	r.Get("/healthz", a.health)
	r.Get("/readyz", a.ready)
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/register", a.register)
		r.Post("/auth/login", a.login)
		r.Group(func(r chi.Router) {
			r.Use(a.authenticate)
			r.Get("/me", a.me)
			r.Post("/auth/logout", a.logout)
			r.Get("/projects", a.listProjects)
			r.Post("/projects", a.createProject)
			r.Get("/projects/{projectID}/issues", a.listIssues)
			r.Post("/projects/{projectID}/issues", a.createIssue)
			r.Patch("/issues/{issueID}", a.updateIssue)
			r.Get("/issues/{issueID}/comments", a.listComments)
			r.Post("/issues/{issueID}/comments", a.createComment)
		})
	})
	return r
}

type contextKey string

const userIDKey contextKey = "user_id"

func (a *App) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		a.logger.Info("http request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
	})
}

func (a *App) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) ready(w http.ResponseWriter, r *http.Request) {
	if err := a.db.PingContext(r.Context()); err != nil {
		a.logger.Error("readiness check failed", "error", err)
		writeError(w, http.StatusServiceUnavailable, "not_ready", "database unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type user struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
}

func (a *App) register(w http.ResponseWriter, r *http.Request) {
	var input credentials
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if !validEmail(input.Email) || len(input.Password) < 12 || len(input.Password) > 72 {
		writeError(w, http.StatusBadRequest, "invalid_input", "provide a valid email and a password of 12 to 72 bytes")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		a.internalError(w, "hash password", "could not create account", err)
		return
	}
	created := user{ID: uuid.NewString(), Email: input.Email}
	err = a.db.QueryRowContext(r.Context(),
		"INSERT INTO users(id,email,password_hash) VALUES($1,$2,$3) RETURNING created_at",
		created.ID, created.Email, string(hash)).Scan(&created.CreatedAt)
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.Code == "23505" {
			writeError(w, http.StatusConflict, "email_in_use", "an account with this email already exists")
			return
		}
		a.internalError(w, "create user", "could not create account", err)
		return
	}
	if !a.createSession(w, r, created.ID) {
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var input credentials
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	var id, passwordHash string
	var createdAt time.Time
	err := a.db.QueryRowContext(r.Context(),
		"SELECT id,password_hash,created_at FROM users WHERE email=$1", input.Email).
		Scan(&id, &passwordHash, &createdAt)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(input.Password)) != nil) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "email or password is incorrect")
		return
	}
	if err != nil {
		a.internalError(w, "load user for sign in", "could not sign in", err)
		return
	}
	if !a.createSession(w, r, id) {
		return
	}
	writeJSON(w, http.StatusOK, user{ID: id, Email: input.Email, CreatedAt: createdAt})
}

func (a *App) createSession(w http.ResponseWriter, r *http.Request, userID string) bool {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		a.logger.Error("generate session token failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create session")
		return false
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	expires := time.Now().Add(a.sessionTTL)
	if _, err := a.db.ExecContext(r.Context(), "DELETE FROM sessions WHERE expires_at <= now()"); err != nil {
		a.internalError(w, "remove expired sessions", "could not create session", err)
		return false
	}
	if _, err := a.db.ExecContext(r.Context(),
		"INSERT INTO sessions(token_hash,user_id,expires_at) VALUES($1,$2,$3)", hash[:], userID, expires); err != nil {
		a.internalError(w, "persist session", "could not create session", err)
		return false
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/", Expires: expires, MaxAge: int(a.sessionTTL.Seconds()),
		HttpOnly: true, Secure: a.secureCookie, SameSite: http.SameSiteStrictMode,
	})
	return true
}

func (a *App) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil || cookie.Value == "" {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "sign in to continue")
			return
		}
		hash := sha256.Sum256([]byte(cookie.Value))
		var id string
		err = a.db.QueryRowContext(r.Context(),
			"SELECT user_id FROM sessions WHERE token_hash=$1 AND expires_at>now()", hash[:]).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "sign in to continue")
			return
		}
		if err != nil {
			a.internalError(w, "authenticate session", "could not authenticate request", err)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Header.Get("X-Requested-With") != "XMLHttpRequest" {
			writeError(w, http.StatusForbidden, "csrf_check_failed", "required request header is missing")
			return
		}
		next.ServeHTTP(w, r.WithContext(contextWithUser(r, id)))
	})
}

func contextWithUser(r *http.Request, id string) context.Context {
	return context.WithValue(r.Context(), userIDKey, id)
}

func currentUserID(r *http.Request) string {
	id, _ := r.Context().Value(userIDKey).(string)
	return id
}

func (a *App) me(w http.ResponseWriter, r *http.Request) {
	var result user
	err := a.db.QueryRowContext(r.Context(), "SELECT id,email,created_at FROM users WHERE id=$1", currentUserID(r)).
		Scan(&result.ID, &result.Email, &result.CreatedAt)
	if err != nil {
		a.internalError(w, "load account", "could not load account", err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookie)
	if err == nil {
		hash := sha256.Sum256([]byte(cookie.Value))
		if _, err := a.db.ExecContext(r.Context(), "DELETE FROM sessions WHERE token_hash=$1", hash[:]); err != nil {
			a.internalError(w, "delete session", "could not sign out", err)
			return
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.secureCookie, SameSite: http.SameSiteStrictMode})
	w.WriteHeader(http.StatusNoContent)
}

type project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

func (a *App) listProjects(w http.ResponseWriter, r *http.Request) {
	rows, err := a.db.QueryContext(r.Context(), "SELECT id,name,created_at FROM projects WHERE owner_id=$1 ORDER BY created_at DESC", currentUserID(r))
	if err != nil {
		a.internalError(w, "list projects", "could not load projects", err)
		return
	}
	defer rows.Close()
	results := make([]project, 0)
	for rows.Next() {
		var p project
		if err := rows.Scan(&p.ID, &p.Name, &p.CreatedAt); err != nil {
			a.internalError(w, "scan project", "could not load projects", err)
			return
		}
		results = append(results, p)
	}
	if err := rows.Err(); err != nil {
		a.internalError(w, "iterate projects", "could not load projects", err)
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (a *App) createProject(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if utf8.RuneCountInString(input.Name) == 0 || utf8.RuneCountInString(input.Name) > 120 {
		writeError(w, http.StatusBadRequest, "invalid_input", "project name must be 1 to 120 characters")
		return
	}
	var result project
	err := a.db.QueryRowContext(r.Context(),
		"INSERT INTO projects(id,owner_id,name) VALUES($1,$2,$3) RETURNING id,name,created_at",
		uuid.NewString(), currentUserID(r), input.Name).Scan(&result.ID, &result.Name, &result.CreatedAt)
	if err != nil {
		a.internalError(w, "create project", "could not create project", err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

type issue struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"projectId"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (a *App) listIssues(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	rows, err := a.db.QueryContext(r.Context(), `
		SELECT i.id,i.project_id,i.title,i.description,i.status,i.created_at,i.updated_at
		FROM issues i JOIN projects p ON p.id=i.project_id
		WHERE i.project_id=$1 AND p.owner_id=$2 ORDER BY i.created_at DESC`,
		projectID, currentUserID(r))
	if err != nil {
		a.internalError(w, "list issues", "could not load issues", err)
		return
	}
	defer rows.Close()
	results := make([]issue, 0)
	for rows.Next() {
		var item issue
		if err := rows.Scan(&item.ID, &item.ProjectID, &item.Title, &item.Description, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			a.internalError(w, "scan issue", "could not load issues", err)
			return
		}
		results = append(results, item)
	}
	if err := rows.Err(); err != nil {
		a.internalError(w, "iterate issues", "could not load issues", err)
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (a *App) createIssue(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Title = strings.TrimSpace(input.Title)
	if utf8.RuneCountInString(input.Title) == 0 || utf8.RuneCountInString(input.Title) > 200 || utf8.RuneCountInString(input.Description) > 10000 {
		writeError(w, http.StatusBadRequest, "invalid_input", "title must be 1 to 200 characters and description at most 10000")
		return
	}
	var result issue
	err := a.db.QueryRowContext(r.Context(), `
		INSERT INTO issues(id,project_id,created_by,title,description)
		SELECT $1,p.id,$3,$4,$5 FROM projects p WHERE p.id=$2 AND p.owner_id=$3
		RETURNING id,project_id,title,description,status,created_at,updated_at`,
		uuid.NewString(), chi.URLParam(r, "projectID"), currentUserID(r), input.Title, input.Description).
		Scan(&result.ID, &result.ProjectID, &result.Title, &result.Description, &result.Status, &result.CreatedAt, &result.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "project not found")
		return
	}
	if err != nil {
		a.internalError(w, "create issue", "could not create issue", err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (a *App) updateIssue(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Status string `json:"status"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Status != "open" && input.Status != "in_progress" && input.Status != "done" {
		writeError(w, http.StatusBadRequest, "invalid_input", "status must be open, in_progress, or done")
		return
	}
	var result issue
	err := a.db.QueryRowContext(r.Context(), `
		UPDATE issues i SET status=$1,updated_at=now()
		FROM projects p WHERE i.id=$2 AND p.id=i.project_id AND p.owner_id=$3
		RETURNING i.id,i.project_id,i.title,i.description,i.status,i.created_at,i.updated_at`,
		input.Status, chi.URLParam(r, "issueID"), currentUserID(r)).
		Scan(&result.ID, &result.ProjectID, &result.Title, &result.Description, &result.Status, &result.CreatedAt, &result.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "issue not found")
		return
	}
	if err != nil {
		a.internalError(w, "update issue", "could not update issue", err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type comment struct {
	ID        string    `json:"id"`
	IssueID   string    `json:"issueId"`
	AuthorID  string    `json:"authorId"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}

func (a *App) listComments(w http.ResponseWriter, r *http.Request) {
	issueID := chi.URLParam(r, "issueID")
	rows, err := a.db.QueryContext(r.Context(), `
		SELECT c.id,c.issue_id,c.author_id,c.body,c.created_at
		FROM comments c JOIN issues i ON i.id=c.issue_id JOIN projects p ON p.id=i.project_id
		WHERE c.issue_id=$1 AND p.owner_id=$2 ORDER BY c.created_at`,
		issueID, currentUserID(r))
	if err != nil {
		a.internalError(w, "list comments", "could not load comments", err)
		return
	}
	defer rows.Close()
	results := make([]comment, 0)
	for rows.Next() {
		var item comment
		if err := rows.Scan(&item.ID, &item.IssueID, &item.AuthorID, &item.Body, &item.CreatedAt); err != nil {
			a.internalError(w, "scan comment", "could not load comments", err)
			return
		}
		results = append(results, item)
	}
	if err := rows.Err(); err != nil {
		a.internalError(w, "iterate comments", "could not load comments", err)
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (a *App) createComment(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Body string `json:"body"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Body = strings.TrimSpace(input.Body)
	if utf8.RuneCountInString(input.Body) == 0 || utf8.RuneCountInString(input.Body) > 4000 {
		writeError(w, http.StatusBadRequest, "invalid_input", "comment must be 1 to 4000 characters")
		return
	}
	var result comment
	err := a.db.QueryRowContext(r.Context(), `
		INSERT INTO comments(id,issue_id,author_id,body)
		SELECT $1,i.id,$3,$4 FROM issues i JOIN projects p ON p.id=i.project_id
		WHERE i.id=$2 AND p.owner_id=$3
		RETURNING id,issue_id,author_id,body,created_at`,
		uuid.NewString(), chi.URLParam(r, "issueID"), currentUserID(r), input.Body).
		Scan(&result.ID, &result.IssueID, &result.AuthorID, &result.Body, &result.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "issue not found")
		return
	}
	if err != nil {
		a.internalError(w, "create comment", "could not create comment", err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func validEmail(email string) bool {
	if len(email) > 254 || strings.Count(email, "@") != 1 {
		return false
	}
	parts := strings.SplitN(email, "@", 2)
	return len(parts[0]) > 0 && len(parts[1]) > 3 && strings.Contains(parts[1], ".")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "content type must be application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body exceeds 1 MiB")
			return false
		}
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must contain a single JSON value")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func (a *App) internalError(w http.ResponseWriter, operation, message string, err error) {
	a.logger.Error(operation, "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error", message)
}
