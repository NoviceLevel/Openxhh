package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newAuthTestState(t *testing.T) (*serverState, string) {
	t.Helper()
	root := t.TempDir()
	authPath := filepath.Join(root, "webui_auth.json")
	if _, _, err := ensureAuth(authPath); err != nil {
		t.Fatalf("ensureAuth: %v", err)
	}
	state := &serverState{
		rootDir:    root,
		authPath:   authPath,
		service:    "Openxhh",
		sessions:   map[string]time.Time{},
		loginFails: map[string]loginFail{},
	}
	token, expiresAt, err := state.createSessionToken()
	if err != nil {
		t.Fatalf("createSessionToken: %v", err)
	}
	state.sessions[token] = expiresAt
	return state, token
}

func requestWithSession(method, path, token string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.AddCookie(&http.Cookie{Name: webuiSessionCookieName, Value: token})
	return req
}

func TestValidSessionRequiresTrackedSession(t *testing.T) {
	state, token := newAuthTestState(t)
	req := requestWithSession(http.MethodGet, "/", token)

	if !state.validSession(req) {
		t.Fatal("validSession returned false for tracked signed session")
	}
	delete(state.sessions, token)
	if state.validSession(req) {
		t.Fatal("validSession returned true after session was removed")
	}
}

func TestRequireAuthRejectsPostWithoutCSRF(t *testing.T) {
	state, token := newAuthTestState(t)
	called := false
	handler := state.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	req := requestWithSession(http.MethodPost, "/api/restart", token)
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
	if called {
		t.Fatal("handler was called without CSRF token")
	}
}

func TestRequireAuthAllowsPostWithCSRF(t *testing.T) {
	state, token := newAuthTestState(t)
	called := false
	handler := state.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})

	req := requestWithSession(http.MethodPost, "/api/restart", token)
	req.Header.Set("X-CSRF-Token", csrfTokenForSession(token))
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if !called {
		t.Fatal("handler was not called with valid CSRF token")
	}
}

func TestLogoutInvalidatesTrackedSession(t *testing.T) {
	state, token := newAuthTestState(t)
	handler := state.requireAuth(state.handleLogout)
	req := requestWithSession(http.MethodPost, "/logout", token)
	req.Header.Set("X-CSRF-Token", csrfTokenForSession(token))
	rr := httptest.NewRecorder()

	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if state.validSession(requestWithSession(http.MethodGet, "/", token)) {
		t.Fatal("session remains valid after logout")
	}
}

func TestPlaceholderBuildersUseOnlyBoundParameterMarkers(t *testing.T) {
	if got := sqlitePlaceholders(3); got != "?,?,?" {
		t.Fatalf("sqlitePlaceholders = %q, want ?,?,?", got)
	}
	if got := postgresPlaceholders(3); got != "$1,$2,$3" {
		t.Fatalf("postgresPlaceholders = %q, want $1,$2,$3", got)
	}
}

func TestRecordLinkLookupColumnsAreWhitelisted(t *testing.T) {
	for _, column := range []string{"msg_id", "comment_a_id"} {
		if !validRecordLinkLookupColumn(column) {
			t.Fatalf("validRecordLinkLookupColumn(%q) = false", column)
		}
	}
	for _, column := range []string{"msg_id; DROP TABLE at", "comment_text", ""} {
		if validRecordLinkLookupColumn(column) {
			t.Fatalf("validRecordLinkLookupColumn(%q) = true", column)
		}
	}
}

func TestQRCodeImageServesGeneratedPNG(t *testing.T) {
	state, token := newAuthTestState(t)
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	if err := os.WriteFile(filepath.Join(state.rootDir, "qrcode.png"), png, 0600); err != nil {
		t.Fatalf("write qrcode.png: %v", err)
	}
	req := requestWithSession(http.MethodGet, "/qrcode.png", token)
	rr := httptest.NewRecorder()

	state.requireAuth(state.handleQRCodeImage)(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if rr.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("Content-Type = %q, want image/png", rr.Header().Get("Content-Type"))
	}
	if rr.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", rr.Header().Get("Cache-Control"))
	}
}

func TestQRCodePageLinksToImage(t *testing.T) {
	state, token := newAuthTestState(t)
	req := requestWithSession(http.MethodGet, "/qrcode", token)
	rr := httptest.NewRecorder()

	state.requireAuth(state.handleQRCodePage)(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "/qrcode.png") {
		t.Fatalf("qrcode page should link image, got %q", rr.Body.String())
	}
}

func TestQRCodePageShowsCompactScanSize(t *testing.T) {
	state, token := newAuthTestState(t)
	req := requestWithSession(http.MethodGet, "/qrcode", token)
	rr := httptest.NewRecorder()

	state.requireAuth(state.handleQRCodePage)(rr, req)

	body := rr.Body.String()
	for _, want := range []string{"overflow:auto", "width:360px", "height:360px"} {
		if !strings.Contains(body, want) {
			t.Fatalf("qrcode page missing %q in %q", want, body)
		}
	}
}

func TestIndexTemplateIncludesMeguminPersonaTemplateAction(t *testing.T) {
	var body bytes.Buffer
	if err := indexTemplate.Execute(&body, indexViewData{Authed: true, Service: "Openxhh", CSRFToken: "test-csrf"}); err != nil {
		t.Fatalf("render index template: %v", err)
	}
	html := body.String()
	for _, want := range []string{
		"套用惠惠酒馆人设",
		"meguminPersonaTemplate",
		"先回应对方这句话本身，再体现惠惠",
		"不要每句都出现惠惠、红魔族、爆裂魔法、本大魔法师、委托、召唤",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("index template missing %q", want)
		}
	}
}
