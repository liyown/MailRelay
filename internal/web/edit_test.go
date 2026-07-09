package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/becomeopc/opc-mailrelay/internal/command"
	"github.com/becomeopc/opc-mailrelay/internal/config"
)

type fakeEditor struct {
	draft    config.Draft
	applyErr error
	applied  *config.Draft
}

func (f *fakeEditor) LoadDraft() (config.Draft, error) { return f.draft, nil }
func (f *fakeEditor) ApplyDraft(_ context.Context, d config.Draft) error {
	f.applied = &d
	return f.applyErr
}

func TestConfigDraftReadAndWrite(t *testing.T) {
	sessions, _ := newTestSessions(t)
	editor := &fakeEditor{draft: config.Draft{
		Commands:      []command.Command{{Name: "push", Handler: "http"}},
		HTTPHosts:     []string{"api.example.com"},
		CatalogNotify: []string{"me@example.com"},
	}}
	h := NewServer(ServerOptions{Sessions: sessions, Repository: seededRepository(t), Editor: editor})
	cookie, csrf := loginForTest(t, h)

	// GET draft requires a session and returns the editable config.
	get := httptest.NewRequest(http.MethodGet, "/api/v1/config/draft", nil)
	get.AddCookie(cookie)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, get)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"push"`) {
		t.Fatalf("get draft status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/config/draft", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated get=%d", w.Code)
	}

	body := `{"commands":[{"name":"deploy","handler":"http"}],"http_hosts":["api.example.com"],"catalog_notify":[]}`

	// PUT without a CSRF token is rejected.
	req := httptest.NewRequest(http.MethodPut, "/api/v1/config/draft", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("put without csrf=%d", w.Code)
	}

	// PUT with a CSRF token applies the draft.
	req = httptest.NewRequest(http.MethodPut, "/api/v1/config/draft", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("put status=%d body=%s", w.Code, w.Body.String())
	}
	if editor.applied == nil || len(editor.applied.Commands) != 1 || editor.applied.Commands[0].Name != "deploy" {
		t.Fatalf("editor did not receive the draft: %#v", editor.applied)
	}

	// A DraftError surfaces as 422 invalid_config with the safe message.
	editor.applyErr = &DraftError{Message: `unknown handler "bogus"`}
	req = httptest.NewRequest(http.MethodPut, "/api/v1/config/draft", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "invalid_config") || !strings.Contains(w.Body.String(), "bogus") {
		t.Fatalf("invalid draft status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestConfigDraftUnavailableWithoutEditor(t *testing.T) {
	sessions, _ := newTestSessions(t)
	h := NewServer(ServerOptions{Sessions: sessions, Repository: seededRepository(t)})
	cookie, _ := loginForTest(t, h)
	get := httptest.NewRequest(http.MethodGet, "/api/v1/config/draft", nil)
	get.AddCookie(cookie)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, get)
	if w.Code != http.StatusNotImplemented {
		t.Fatalf("get draft without editor=%d", w.Code)
	}
}
