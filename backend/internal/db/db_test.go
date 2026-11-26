package db

import (
	"os"
	"path/filepath"
	"testing"

	"familyflix/internal/models"
)

func TestScanFolderIndexesMoviesRecursivelyWithoutDuplicates(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "action")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	moviePath := filepath.Join(nested, "Family Movie.MKV")
	if err := os.WriteFile(moviePath, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("not a movie"), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	imported, err := store.ScanFolder(root)
	if err != nil {
		t.Fatalf("first scan failed: %v", err)
	}
	if len(imported) != 1 {
		t.Fatalf("first scan imported %d movies, want 1", len(imported))
	}
	if imported[0].FilePath != moviePath {
		t.Fatalf("movie path = %q, want %q", imported[0].FilePath, moviePath)
	}

	imported, err = store.ScanFolder(root)
	if err != nil {
		t.Fatalf("second scan failed: %v", err)
	}
	if len(imported) != 0 {
		t.Fatalf("second scan imported %d movies, want 0", len(imported))
	}

	movies, err := store.ListMovies()
	if err != nil {
		t.Fatalf("list movies failed: %v", err)
	}
	if len(movies) != 1 {
		t.Fatalf("database contains %d movies, want 1", len(movies))
	}

	if err := os.Remove(moviePath); err != nil {
		t.Fatal(err)
	}
	newMoviePath := filepath.Join(root, "New Arrival.mp4")
	if err := os.WriteFile(newMoviePath, []byte("new video"), 0o600); err != nil {
		t.Fatal(err)
	}
	imported, err = store.ScanFolder(root)
	if err != nil {
		t.Fatalf("refresh scan failed: %v", err)
	}
	if len(imported) != 1 || imported[0].FilePath != newMoviePath {
		t.Fatalf("refresh imported %#v, want only %q", imported, newMoviePath)
	}
	movies, err = store.ListMovies()
	if err != nil {
		t.Fatalf("list movies after refresh failed: %v", err)
	}
	if len(movies) != 1 || movies[0].FilePath != newMoviePath {
		t.Fatalf("database after refresh contains %#v, want only %q", movies, newMoviePath)
	}

	if _, err := store.ScanFolder(filepath.Join(root, "missing-folder")); err == nil {
		t.Fatal("expected scan of missing folder to fail")
	}
	movies, err = store.ListMovies()
	if err != nil {
		t.Fatalf("list movies after failed scan failed: %v", err)
	}
	if len(movies) != 1 || movies[0].FilePath != newMoviePath {
		t.Fatalf("failed scan changed movie list: %#v", movies)
	}
}

func TestUserManagementKeepsAdminsAndHidesPasswordHashes(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	member, err := store.CreateUser("family-member", "member-pass-123", "user")
	if err != nil {
		t.Fatalf("create member: %v", err)
	}
	storedMember, err := store.GetUserByUsername(member.Username)
	if err != nil {
		t.Fatalf("get member: %v", err)
	}
	if storedMember.PasswordHash == "" || storedMember.PasswordHash == "member-pass-123" {
		t.Fatal("expected a stored password hash")
	}

	users, err := store.ListUsers()
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	var listedMember *models.User
	for i := range users {
		if users[i].ID == member.ID {
			listedMember = &users[i]
			break
		}
	}
	if listedMember == nil || listedMember.PasswordHash != "" {
		t.Fatal("listed users must not include password hashes")
	}

	admin, err := store.GetUserByUsername("admin")
	if err != nil {
		t.Fatalf("get seeded admin: %v", err)
	}
	if deleted, err := store.DeleteMember(admin.ID); err != nil || deleted {
		t.Fatalf("DeleteMember(admin) = (%v, %v), want (false, nil)", deleted, err)
	}
	if deleted, err := store.DeleteMember(member.ID); err != nil || !deleted {
		t.Fatalf("DeleteMember(member) = (%v, %v), want (true, nil)", deleted, err)
	}
	if _, err := store.GetUserByUsername(member.Username); err == nil {
		t.Fatal("expected deleted member to be unavailable")
	}
}
