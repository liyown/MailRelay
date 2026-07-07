package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	sessions, _ := newTestSessions(t)
	return NewServer(ServerOptions{Sessions: sessions, Repository: seededRepository(t)})
}

func TestHealthRequiresSessionAndSetsSecurityHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()
	newTestServer(t).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("X-Request-ID") == "" {
		t.Fatalf("missing security headers: %#v", w.Header())
	}
}

func TestLoginSessionAndDashboardRoute(t *testing.T) {
	h := newTestServer(t)
	login := httptest.NewRequest(http.MethodPost, "/api/v1/login", bytes.NewBufferString(`{"password":"correct horse"}`))
	login.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, login)
	if w.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", w.Code, w.Body.String())
	}
	var loginBody struct {
		CSRF string `json:"csrf"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &loginBody); err != nil || loginBody.CSRF == "" {
		t.Fatalf("login body=%s err=%v", w.Body.String(), err)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies=%#v", cookies)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard?range=24h", nil)
	req.AddCookie(cookies[0])
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte(`"execution_count":3`)) {
		t.Fatalf("dashboard status=%d body=%s", w.Code, w.Body.String())
	}
	for _, secret := range []string{"topsecret", "api_key", "correct horse"} {
		if bytes.Contains(bytes.ToLower(w.Body.Bytes()), []byte(secret)) {
			t.Fatalf("response leaked %s", secret)
		}
	}
}

func TestRoutesRejectMalformedFiltersAndMethods(t *testing.T) {
	h := newTestServer(t)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/login", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("method status=%d", w.Code)
	}

	login := httptest.NewRequest(http.MethodPost, "/api/v1/login", bytes.NewBufferString(`{"password":"correct horse"}`))
	login.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, login)
	cookie := w.Result().Cookies()[0]
	req = httptest.NewRequest(http.MethodGet, "/api/v1/executions?limit=invalid", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("filter status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestServerEmbedsSPAAndFallsBackForClientRoutes(t *testing.T) {
	h := newTestServer(t)
	for _, target := range []string{"/", "/executions"} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK || !bytes.Contains(w.Body.Bytes(), []byte("MailRelay")) {
			t.Fatalf("target=%s status=%d body=%s", target, w.Code, w.Body.String())
		}
		if !strings.Contains(w.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("target=%s content-type=%q", target, w.Header().Get("Content-Type"))
		}
	}
}
