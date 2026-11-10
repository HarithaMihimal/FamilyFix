package main

import (
	"path/filepath"
	"testing"

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
