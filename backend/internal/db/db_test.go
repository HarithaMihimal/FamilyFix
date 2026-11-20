package db

import (
	"os"
	"path/filepath"
	"testing"
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
