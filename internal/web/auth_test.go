package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestSessions(t *testing.T) (*SessionManager, *time.Time) {
	t.Helper()
	now := time.Date(2026, 7, 8, 8, 0, 0, 0, time.UTC)
	hash, err := HashPassword("correct horse", bytes.NewReader(bytes.Repeat([]byte{7}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	m, err := NewSessionManager(SessionOptions{
		Secret:       []byte(strings.Repeat("s", 32)),
		PasswordHash: hash,
		TTL:          time.Hour,
		Now:          func() time.Time { return now },
		Random:       bytes.NewReader(bytes.Repeat([]byte{9}, 256)),
	})
	if err != nil {
		t.Fatal(err)
	}
	return m, &now
}

func TestSessionPasswordAndCookieSecurity(t *testing.T) {
	m, _ := newTestSessions(t)
	if !m.VerifyPassword("correct horse") || m.VerifyPassword("wrong") {
		t.Fatal("password verification mismatch")
	}
	cookie, csrf, err := m.Issue("admin")
	if err != nil {
		t.Fatal(err)
	}
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || csrf == "" {
		t.Fatalf("insecure cookie: %#v csrf=%q", cookie, csrf)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.AddCookie(cookie)
	session, ok := m.Authenticate(req)
	if !ok || session.Subject != "admin" || session.CSRF != csrf {
		t.Fatalf("session=%#v ok=%v", session, ok)
	}
}

func TestSessionRejectsTamperingExpiryAndCSRFMismatch(t *testing.T) {
	m, now := newTestSessions(t)
	cookie, csrf, err := m.Issue("admin")
	if err != nil {
		t.Fatal(err)
	}
	tampered := *cookie
	tampered.Value += "x"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.AddCookie(&tampered)
	if _, ok := m.Authenticate(req); ok {
		t.Fatal("tampered cookie authenticated")
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/logout", nil)
	req.AddCookie(cookie)
	req.Header.Set(csrfHeader, "wrong")
	if m.ValidCSRF(req) {
		t.Fatal("mismatched csrf accepted")
	}
	req.Header.Set(csrfHeader, csrf)
	if !m.ValidCSRF(req) {
		t.Fatal("matching csrf rejected")
	}

	*now = now.Add(2 * time.Hour)
	if _, ok := m.Authenticate(req); ok {
		t.Fatal("expired cookie authenticated")
	}
}

func TestLogoutCookieExpiresImmediately(t *testing.T) {
	m, _ := newTestSessions(t)
	c := m.LogoutCookie()
	if c.MaxAge != -1 || !c.Expires.Before(time.Now()) || !c.HttpOnly {
		t.Fatalf("unexpected logout cookie: %#v", c)
	}
}
