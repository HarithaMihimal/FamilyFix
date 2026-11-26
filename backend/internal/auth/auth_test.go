package auth

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"familyflix/internal/db"
)

func TestSessionCookieIsSignedAndIdentifiesUser(t *testing.T) {
	store, err := db.New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	key := []byte("test-session-signing-key-that-is-long-enough")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/login", nil)
	if err := Login(recorder, request, store, key, "admin", "admin123"); err != nil {
		t.Fatalf("login: %v", err)
	}
	cookie := recorder.Result().Cookies()[0]
	if cookie.Value == "admin" {
		t.Fatal("session cookie must not contain a forgeable plain username")
	}

	request.AddCookie(cookie)
	user, err := CurrentUser(request, store, key)
	if err != nil {
		t.Fatalf("read signed session: %v", err)
	}
	if user.Username != "admin" || user.Role != "admin" {
		t.Fatalf("session user = %#v, want admin", user)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/session", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "YWRtaW4"})
	if _, err := CurrentUser(request, store, key); err == nil {
		t.Fatal("expected unsigned username cookie to be rejected")
	}

	request = httptest.NewRequest(http.MethodGet, "/api/session", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: signSession(key, "admin", time.Now().Add(-time.Second))})
	if _, err := CurrentUser(request, store, key); err == nil {
		t.Fatal("expected expired session cookie to be rejected")
	}
}

func TestRequireAdminRejectsNormalUsersAndForgedCookies(t *testing.T) {
	store, err := db.New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.CreateUser("family-member", "member-pass-123", "user"); err != nil {
		t.Fatal(err)
	}

	key := []byte("test-session-signing-key-that-is-long-enough")
	recorder := httptest.NewRecorder()
	if err := Login(recorder, httptest.NewRequest(http.MethodPost, "/api/login", nil), store, key, "family-member", "member-pass-123"); err != nil {
		t.Fatalf("member login: %v", err)
	}
	cookie := recorder.Result().Cookies()[0]
	handler := RequireAdmin(store, key)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("normal-user admin response = %d, want %d", response.Code, http.StatusForbidden)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: signSession(key, "admin", time.Now().Add(sessionDuration)) + "forged"})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("forged-cookie admin response = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}
