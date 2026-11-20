package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"familyflix/internal/db"
)

func TestDetectNetworkInfo(t *testing.T) {
	info := detectNetworkInfo(3000)
	if info.Port != 3000 {
		t.Fatal("expected port 3000")
	}
	if len(info.URLs) == 0 {
		t.Fatal("expected at least one URL")
	}
}

func TestDeviceTrackerCountsUniqueActiveClients(t *testing.T) {
	tracker := newDeviceTracker()
	handler := tracker.middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	for _, remoteAddr := range []string{"192.168.8.10:4567", "192.168.8.10:4568", "192.168.8.11:4567"} {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.RemoteAddr = remoteAddr
		handler.ServeHTTP(httptest.NewRecorder(), request)
	}

	if got := tracker.count(time.Now()); got != 2 {
		t.Fatalf("expected 2 active devices, got %d", got)
	}

	tracker.mu.Lock()
	tracker.lastSeen["192.168.8.12"] = time.Now().Add(-activeDeviceWindow - time.Second)
	tracker.mu.Unlock()
	if got := tracker.count(time.Now()); got != 2 {
		t.Fatalf("expected expired device to be excluded, got %d active devices", got)
	}
}

func TestRequestClientIPUsesForwardedAddressFromLoopbackProxy(t *testing.T) {
	request, _ := http.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "127.0.0.1:4567"
	request.Header.Set("X-Forwarded-For", "203.0.113.20, 192.168.8.173")

	if got := requestClientIP(request); got != "192.168.8.173" {
		t.Fatalf("expected forwarded client IP, got %q", got)
	}
}

func TestDBSeedAdmin(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	store, err := initDBForTest(dir)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer store.Close()
	user, err := store.GetUserByUsername("admin")
	if err != nil {
		t.Fatalf("admin user missing: %v", err)
	}
	if user.Role != "admin" {
		t.Fatalf("expected admin role, got %s", user.Role)
	}
}

func initDBForTest(dir string) (*db.DB, error) {
	return db.New(dir)
}
