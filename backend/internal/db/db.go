package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"familyflix/internal/models"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
	scanMu sync.Mutex
}

func New(dataDir string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dataDir, "familyflix.db")
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	store := &DB{DB: sqlDB}
	if err := store.InitSchema(); err != nil {
		return nil, err
	}
	if err := store.SeedAdmin(); err != nil {
		return nil, err
	}
	return store, nil
}

func (d *DB) InitSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'member',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS movies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			file_name TEXT NOT NULL,
			file_path TEXT NOT NULL,
			poster_url TEXT,
			description TEXT,
			genre TEXT,
			duration TEXT,
			resolution TEXT,
			format TEXT,
			file_size INTEGER DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS folders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			path TEXT UNIQUE NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS watch_progress (
			movie_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			position REAL NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (movie_id, user_id)
		);`,
	}
	for _, q := range queries {
		if _, err := d.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func (d *DB) SeedAdmin() error {
	var count int
	if err := d.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = d.Exec(
		"INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)",
		"admin",
		string(passwordHash),
		"admin",
	)
	return err
}

func (d *DB) GetUserByUsername(username string) (*models.User, error) {
	var user models.User
	err := d.QueryRow(
		"SELECT id, username, password_hash, role, created_at FROM users WHERE username = ?",
		username,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (d *DB) CreateUser(username, password, role string) (*models.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	res, err := d.Exec(
		"INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)",
		username,
		string(hash),
		role,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &models.User{ID: id, Username: username, Role: role, PasswordHash: string(hash)}, nil
}

func (d *DB) AddFolder(path string) error {
	_, err := d.Exec("INSERT OR IGNORE INTO folders (path) VALUES (?)", path)
	return err
}

func (d *DB) ListFolders() ([]models.Folder, error) {
	rows, err := d.Query("SELECT id, path, created_at FROM folders ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]models.Folder, 0)
	for rows.Next() {
		var folder models.Folder
		if err := rows.Scan(&folder.ID, &folder.Path, &folder.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, folder)
	}
	return items, nil
}

func (d *DB) ListMovies() ([]models.Movie, error) {
	rows, err := d.Query("SELECT id, title, file_name, file_path, poster_url, description, genre, duration, resolution, format, file_size, created_at, updated_at FROM movies ORDER BY updated_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	movies := make([]models.Movie, 0)
	for rows.Next() {
		var movie models.Movie
		if err := rows.Scan(&movie.ID, &movie.Title, &movie.FileName, &movie.FilePath, &movie.PosterURL, &movie.Description, &movie.Genre, &movie.Duration, &movie.Resolution, &movie.Format, &movie.FileSize, &movie.CreatedAt, &movie.UpdatedAt); err != nil {
			return nil, err
		}
		movies = append(movies, movie)
	}
	return movies, nil
}

func (d *DB) GetMovie(id int64) (*models.Movie, error) {
	var movie models.Movie
	err := d.QueryRow("SELECT id, title, file_name, file_path, poster_url, description, genre, duration, resolution, format, file_size, created_at, updated_at FROM movies WHERE id = ?", id).Scan(
		&movie.ID,
		&movie.Title,
		&movie.FileName,
		&movie.FilePath,
		&movie.PosterURL,
		&movie.Description,
		&movie.Genre,
		&movie.Duration,
		&movie.Resolution,
		&movie.Format,
		&movie.FileSize,
		&movie.CreatedAt,
		&movie.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &movie, nil
}

func (d *DB) SaveMovie(movie models.Movie) (int64, error) {
	res, err := d.Exec(
		`INSERT INTO movies (title, file_name, file_path, poster_url, description, genre, duration, resolution, format, file_size, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		movie.Title,
		movie.FileName,
		movie.FilePath,
		movie.PosterURL,
		movie.Description,
		movie.Genre,
		movie.Duration,
		movie.Resolution,
		movie.Format,
		movie.FileSize,
		time.Now().UTC().Format(time.RFC3339),
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) UpdateMovieMetadata(id int64, title, description, genre, posterURL, duration, resolution, format string) error {
	_, err := d.Exec(
		`UPDATE movies SET title = ?, description = ?, genre = ?, poster_url = ?, duration = ?, resolution = ?, format = ?, updated_at = ? WHERE id = ?`,
		title,
		description,
		genre,
		posterURL,
		duration,
		resolution,
		format,
		time.Now().UTC().Format(time.RFC3339),
		id,
	)
	return err
}

func (d *DB) DeleteMovie(id int64) error {
	_, err := d.Exec("DELETE FROM movies WHERE id = ?", id)
	return err
}

func (d *DB) SaveWatchProgress(userID int64, movieID int64, position float64) error {
	_, err := d.Exec(
		`INSERT INTO watch_progress (movie_id, user_id, position, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(movie_id, user_id) DO UPDATE SET position = excluded.position, updated_at = excluded.updated_at`,
		movieID,
		userID,
		position,
		time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

func (d *DB) GetWatchProgress(userID int64) ([]models.WatchProgress, error) {
	rows, err := d.Query("SELECT movie_id, user_id, position, updated_at FROM watch_progress WHERE user_id = ? ORDER BY updated_at DESC", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]models.WatchProgress, 0)
	for rows.Next() {
		var item models.WatchProgress
		if err := rows.Scan(&item.MovieID, &item.UserID, &item.Position, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func NormalizeTitle(name string) string {
	base := filepath.Base(name)
	ext := filepath.Ext(base)
	base = strings.TrimSuffix(base, ext)
	base = strings.ReplaceAll(base, "_", " ")
	base = strings.TrimSpace(base)
	if base == "" {
		return "Untitled movie"
	}
	return base
}

func SupportedFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	supported := map[string]bool{".mp4": true, ".mkv": true, ".avi": true, ".mov": true, ".webm": true}
	return supported[ext]
}

func FileFormat(path string) string {
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
}

func (d *DB) ScanFolder(rootPath string) ([]models.Movie, error) {
	rootPath, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, err
	}

	d.scanMu.Lock()
	defer d.scanMu.Unlock()

	movies := make([]models.Movie, 0)
	err = filepath.WalkDir(rootPath, func(fullPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !SupportedFile(entry.Name()) {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect movie %s: %w", fullPath, err)
		}
		movie := models.Movie{
			Title:       NormalizeTitle(fullPath),
			FileName:    entry.Name(),
			FilePath:    filepath.Clean(fullPath),
			Format:      FileFormat(fullPath),
			FileSize:    info.Size(),
			Genre:       "Uncategorized",
			Description: "Imported from local library",
		}

		var existingID int64
		err = d.QueryRow("SELECT id FROM movies WHERE file_path = ? LIMIT 1", movie.FilePath).Scan(&existingID)
		if err == nil {
			return nil
		}
		if err != sql.ErrNoRows {
			return fmt.Errorf("check movie %s: %w", fullPath, err)
		}

		id, err := d.SaveMovie(movie)
		if err != nil {
			return fmt.Errorf("save movie %s: %w", fullPath, err)
		}
		movie.ID = id
		movies = append(movies, movie)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan movie folder %s: %w", rootPath, err)
	}
	return movies, nil
}
