package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/becomeopc/opc-mailrelay/internal/store"
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

// deadenJob enqueues a job and drives it to the dead state so replay can be tested.
func deadenJob(t *testing.T, s *store.Store) int64 {
	t.Helper()
	ctx := context.Background()
	now := time.Now()
	id, err := s.Enqueue(ctx, "notify", map[string]any{"message": "x"}, "replay-key", 1, now)
	if err != nil {
		t.Fatal(err)
	}
	// seededRepository already holds pending jobs with lower ids, and ClaimJob
	// returns them in id order. Drive each claimed job to the dead state until our
	// own job is the one that was deadened.
	for {
		job, err := s.ClaimJob(ctx, now, time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		if job == nil {
			t.Fatalf("target job %d was never claimed", id)
		}
		if err := s.RejectJob(ctx, job, "policy"); err != nil {
			t.Fatal(err)
		}
		if job.ID == id {
			return id
		}
	}
}

func loginForTest(t *testing.T, h http.Handler) (*http.Cookie, string) {
	t.Helper()
	login := httptest.NewRequest(http.MethodPost, "/api/v1/login", bytes.NewBufferString(`{"password":"correct horse"}`))
	login.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, login)
	if w.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		CSRF string `json:"csrf"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.CSRF == "" {
		t.Fatalf("login body=%s err=%v", w.Body.String(), err)
	}
	return w.Result().Cookies()[0], body.CSRF
}

func TestReplayJobRequiresCSRFAndIsIdempotent(t *testing.T) {
	sessions, _ := newTestSessions(t)
	repo := seededRepository(t)
	id := deadenJob(t, repo.store)
	h := NewServer(ServerOptions{Sessions: sessions, Repository: repo})
	cookie, csrf := loginForTest(t, h)

	do := func(path string, withCSRF bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.AddCookie(cookie)
		if withCSRF {
			req.Header.Set("X-CSRF-Token", csrf)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}

	path := "/api/v1/jobs/" + strconv.FormatInt(id, 10) + "/replay"
	if w := do(path, false); w.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status=%d body=%s", w.Code, w.Body.String())
	}
	if w := do(path, true); w.Code != http.StatusOK {
		t.Fatalf("replay status=%d body=%s", w.Code, w.Body.String())
	}
	// Replaying a now-pending (no longer dead) job returns 404.
	if w := do(path, true); w.Code != http.StatusNotFound {
		t.Fatalf("second replay status=%d body=%s", w.Code, w.Body.String())
	}
	if w := do("/api/v1/jobs/abc/replay", true); w.Code != http.StatusBadRequest {
		t.Fatalf("bad id status=%d body=%s", w.Code, w.Body.String())
	}
	if w := do("/api/v1/replies/9999/replay", true); w.Code != http.StatusNotFound {
		t.Fatalf("missing reply status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestReplayRejectsUnauthenticated(t *testing.T) {
	h := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/jobs/1/replay", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestDashboardIncludesTrendSeries(t *testing.T) {
	h := newTestServer(t)
	cookie, _ := loginForTest(t, h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard?range=24h", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var body Dashboard
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v body=%s", err, w.Body.String())
	}
	if len(body.Series) != 24 {
		t.Fatalf("expected 24 series buckets, got %d", len(body.Series))
	}
	total := 0
	for _, p := range body.Series {
		total += p.Count
	}
	if total != body.ExecutionCount {
		t.Fatalf("series total %d != execution count %d", total, body.ExecutionCount)
	}
}
