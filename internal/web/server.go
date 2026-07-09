package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/becomeopc/opc-mailrelay/internal/config"
)

// Editor persists edits to the console-editable slice of configuration (the
// command catalog, outbound host allowlist, and catalog-notify list). It is
// optional: when nil, the editing endpoints report that editing is unavailable.
type Editor interface {
	// LoadDraft returns the current editable config with ${VAR} tokens intact.
	LoadDraft() (config.Draft, error)
	// ApplyDraft validates, persists, and hot-applies a draft. It returns a
	// *DraftError when the draft itself is invalid (safe to surface to the admin)
	// and any other error for internal failures.
	ApplyDraft(ctx context.Context, draft config.Draft) error
}

type ServerOptions struct {
	Sessions   *SessionManager
	Repository *Repository
	Editor     Editor
}

type server struct {
	sessions *SessionManager
	repo     *Repository
	editor   Editor
	mux      *http.ServeMux
}

func NewServer(options ServerOptions) http.Handler {
	s := &server{sessions: options.Sessions, repo: options.Repository, editor: options.Editor, mux: http.NewServeMux()}
	s.routes()
	return s.security(s.mux)
}

func (s *server) routes() {
	s.mux.HandleFunc("POST /api/v1/login", s.login)
	s.mux.HandleFunc("POST /api/v1/logout", s.requireSession(s.logout))
	s.mux.HandleFunc("GET /api/v1/session", s.requireSession(s.session))
	s.mux.HandleFunc("GET /api/v1/health", s.requireSession(s.health))
	s.mux.HandleFunc("GET /api/v1/dashboard", s.requireSession(s.dashboard))
	s.mux.HandleFunc("GET /api/v1/commands", s.requireSession(s.commands))
	s.mux.HandleFunc("GET /api/v1/config/draft", s.requireSession(s.configDraft))
	s.mux.HandleFunc("PUT /api/v1/config/draft", s.requireCSRF(s.saveConfigDraft))
	s.mux.HandleFunc("GET /api/v1/executions", s.requireSession(s.executions))
	s.mux.HandleFunc("GET /api/v1/jobs", s.requireSession(s.jobs))
	s.mux.HandleFunc("GET /api/v1/replies", s.requireSession(s.replies))
	s.mux.HandleFunc("POST /api/v1/jobs/{id}/replay", s.requireCSRF(s.replayJob))
	s.mux.HandleFunc("POST /api/v1/replies/{id}/replay", s.requireCSRF(s.replayReply))
	s.mux.HandleFunc("GET /api/v1/events", s.requireSession(s.events))
	s.mux.HandleFunc("GET /api/v1/system", s.requireSession(s.system))
	s.mux.HandleFunc("/", s.serveSPA)
}

func (s *server) security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		w.Header().Set("X-Request-ID", requestID())
		next.ServeHTTP(w, r)
	})
}

func requestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "unavailable"
	}
	return hex.EncodeToString(b)
}

func (s *server) requireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := s.sessions.Authenticate(r); !ok {
			s.writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
			return
		}
		next(w, r)
	}
}

// requireCSRF guards state-changing endpoints: a valid session plus a matching
// CSRF token. Unauthenticated requests get 401, valid session without a good
// token gets 403.
func (s *server) requireCSRF(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := s.sessions.Authenticate(r); !ok {
			s.writeError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required")
			return
		}
		if !s.sessions.ValidCSRF(r) {
			s.writeError(w, r, http.StatusForbidden, "csrf_mismatch", "CSRF token mismatch")
			return
		}
		next(w, r)
	}
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		s.writeError(w, r, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	var body struct {
		Password string `json:"password"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil || body.Password == "" {
		s.writeError(w, r, http.StatusBadRequest, "invalid_request", "A password is required")
		return
	}
	if !s.sessions.VerifyPassword(body.Password) {
		s.writeError(w, r, http.StatusUnauthorized, "invalid_credentials", "Invalid credentials")
		return
	}
	cookie, csrf, err := s.sessions.Issue("admin")
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "internal", "Unable to create session")
		return
	}
	http.SetCookie(w, cookie)
	s.writeJSON(w, http.StatusOK, map[string]any{"user": map[string]string{"id": "admin", "name": "平台管理员"}, "csrf": csrf})
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if !s.sessions.ValidCSRF(r) {
		s.writeError(w, r, http.StatusForbidden, "csrf_mismatch", "CSRF token mismatch")
		return
	}
	http.SetCookie(w, s.sessions.LogoutCookie())
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) session(w http.ResponseWriter, r *http.Request) {
	session, _ := s.sessions.Authenticate(r)
	s.writeJSON(w, http.StatusOK, map[string]any{"user": map[string]string{"id": session.Subject, "name": "平台管理员"}, "csrf": session.CSRF})
}

func (s *server) health(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

func (s *server) dashboard(w http.ResponseWriter, r *http.Request) {
	rangeKey := r.URL.Query().Get("range")
	if rangeKey == "" {
		rangeKey = "24h"
	}
	result, err := s.repo.Dashboard(r.Context(), rangeKey)
	if err != nil {
		if strings.Contains(err.Error(), "invalid range") {
			s.writeError(w, r, http.StatusBadRequest, "invalid_filter", "Invalid dashboard range")
			return
		}
		s.writeError(w, r, http.StatusInternalServerError, "internal", "Unable to load dashboard")
		return
	}
	s.writeJSON(w, http.StatusOK, result)
}

func (s *server) commands(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]any{"items": s.repo.Commands()})
}

// configDraft returns the editable configuration (commands, outbound host
// allowlist, catalog-notify list) with ${VAR} tokens preserved. Unlike the
// read-only /commands view, this deliberately exposes command config to the
// authenticated admin so it can be edited; secrets should therefore be kept in
// ${VAR} references or sensitive params rather than inlined.
func (s *server) configDraft(w http.ResponseWriter, r *http.Request) {
	if s.editor == nil {
		s.writeError(w, r, http.StatusNotImplemented, "not_implemented", "Configuration editing is not available")
		return
	}
	draft, err := s.editor.LoadDraft()
	if err != nil {
		s.writeError(w, r, http.StatusInternalServerError, "internal", "Unable to load configuration")
		return
	}
	s.writeJSON(w, http.StatusOK, draft)
}

func (s *server) saveConfigDraft(w http.ResponseWriter, r *http.Request) {
	if s.editor == nil {
		s.writeError(w, r, http.StatusNotImplemented, "not_implemented", "Configuration editing is not available")
		return
	}
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		s.writeError(w, r, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 512<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var draft config.Draft
	if err := dec.Decode(&draft); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "invalid_request", "Request body is not valid JSON")
		return
	}
	if err := s.editor.ApplyDraft(r.Context(), draft); err != nil {
		var de *DraftError
		if errors.As(err, &de) {
			s.writeJSON(w, http.StatusUnprocessableEntity, ErrorEnvelope{Error: APIError{Code: "invalid_config", Message: de.Message, Fields: de.Fields, RequestID: w.Header().Get("X-Request-ID")}})
			return
		}
		s.writeError(w, r, http.StatusInternalServerError, "internal", "Unable to apply configuration")
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func parseLimit(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return 20, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > 100 {
		return 0, errors.New("invalid limit")
	}
	return value, nil
}

func (s *server) executions(w http.ResponseWriter, r *http.Request) {
	limit, err := parseLimit(r)
	if err != nil {
		s.writeError(w, r, http.StatusBadRequest, "invalid_filter", "Invalid limit")
		return
	}
	result, err := s.repo.Executions(r.Context(), ExecutionFilter{Cursor: r.URL.Query().Get("cursor"), Status: r.URL.Query().Get("status"), Command: r.URL.Query().Get("command"), Limit: limit})
	s.writeListResult(w, r, result, err)
}

func (s *server) jobs(w http.ResponseWriter, r *http.Request) {
	limit, err := parseLimit(r)
	if err != nil {
		s.writeError(w, r, http.StatusBadRequest, "invalid_filter", "Invalid limit")
		return
	}
	result, err := s.repo.Jobs(r.Context(), JobFilter{Cursor: r.URL.Query().Get("cursor"), Status: r.URL.Query().Get("status"), Limit: limit})
	s.writeListResult(w, r, result, err)
}

func (s *server) replies(w http.ResponseWriter, r *http.Request) {
	limit, err := parseLimit(r)
	if err != nil {
		s.writeError(w, r, http.StatusBadRequest, "invalid_filter", "Invalid limit")
		return
	}
	result, err := s.repo.Replies(r.Context(), ReplyFilter{Cursor: r.URL.Query().Get("cursor"), Status: r.URL.Query().Get("status"), Limit: limit})
	s.writeListResult(w, r, result, err)
}

func (s *server) events(w http.ResponseWriter, r *http.Request) {
	limit, err := parseLimit(r)
	if err != nil {
		s.writeError(w, r, http.StatusBadRequest, "invalid_filter", "Invalid limit")
		return
	}
	result, err := s.repo.Events(r.Context(), EventFilter{Cursor: r.URL.Query().Get("cursor"), Severity: r.URL.Query().Get("severity"), Limit: limit})
	s.writeListResult(w, r, result, err)
}

func (s *server) system(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, s.repo.System())
}

func (s *server) replayJob(w http.ResponseWriter, r *http.Request) {
	s.replay(w, r, s.repo.ReplayJob)
}

func (s *server) replayReply(w http.ResponseWriter, r *http.Request) {
	s.replay(w, r, s.repo.ReplayReply)
}

func (s *server) replay(w http.ResponseWriter, r *http.Request, action func(context.Context, int64) error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		s.writeError(w, r, http.StatusBadRequest, "invalid_request", "A positive numeric id is required")
		return
	}
	if err := action(r.Context(), id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			s.writeError(w, r, http.StatusNotFound, "not_found", "No dead item with that id")
			return
		}
		s.writeError(w, r, http.StatusInternalServerError, "internal", "Unable to replay item")
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *server) writeListResult(w http.ResponseWriter, r *http.Request, value any, err error) {
	if err != nil {
		if strings.Contains(err.Error(), "invalid cursor") {
			s.writeError(w, r, http.StatusBadRequest, "invalid_filter", "Invalid cursor")
			return
		}
		s.writeError(w, r, http.StatusInternalServerError, "internal", "Unable to load records")
		return
	}
	s.writeJSON(w, http.StatusOK, value)
}

func (s *server) writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	s.writeJSON(w, status, ErrorEnvelope{Error: APIError{Code: code, Message: message, RequestID: w.Header().Get("X-Request-ID")}})
}

func (s *server) writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
