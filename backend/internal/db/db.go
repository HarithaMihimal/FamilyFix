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
	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
	scanMu sync.Mutex
	driver string
}

func New(dataDir string) (*DB, error) {
	store, err := open(dataDir, "sqlite", filepath.Join(dataDir, "familyflix.db"))
	if err != nil {
		return nil, err
	}
	if err := store.SeedAdmin(); err != nil {
		_ = store.Close()
		return nil, err
	}
	return store, nil
}

type MySQLConfig struct {
	Host          string
	Port          int
	User          string
	Password      string
	Name          string
	AdminUsername string
	AdminPassword string
}

func NewWithMySQL(dataDir string, cfg MySQLConfig) (*DB, error) {
	if cfg.Host == "" || cfg.User == "" || cfg.Name == "" {
		return nil, fmt.Errorf("DB_HOST, DB_USER, and DB_NAME are required when using MySQL")
	}
	if cfg.Port == 0 {
		cfg.Port = 3306
	}
	mysqlConfig := mysql.Config{
		User:      cfg.User,
		Passwd:    cfg.Password,
		Net:       "tcp",
		Addr:      fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		DBName:    cfg.Name,
		ParseTime: true,
		Loc:       time.UTC,
	}
	dsn := mysqlConfig.FormatDSN()
	store, err := open(dataDir, "mysql", dsn)
	if err != nil {
		return nil, err
	}
	adminUsername := cfg.AdminUsername
	if adminUsername == "" {
		adminUsername = "admin"
	}
	adminPassword := cfg.AdminPassword
	if adminPassword == "" {
		adminPassword = "admin123"
	}
	if err := store.SeedAdminWithCredentials(adminUsername, adminPassword); err != nil {
		_ = store.Close()
		return nil, err
	}
	return store, nil
}

func open(dataDir, driver, source string) (*DB, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	sqlDB, err := sql.Open(driver, source)
	if err != nil {
		return nil, err
	}
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	store := &DB{DB: sqlDB, driver: driver}
	if err := store.InitSchema(); err != nil {
		_ = sqlDB.Close()
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
			role TEXT NOT NULL DEFAULT 'user',
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
		`CREATE TABLE IF NOT EXISTS my_list (
			user_id INTEGER NOT NULL,
			movie_id INTEGER NOT NULL,
			added_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, movie_id)
		);`,
	}
	if d.driver == "mysql" {
		queries = []string{
			`CREATE TABLE IF NOT EXISTS users (
				id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
				username VARCHAR(191) NOT NULL UNIQUE,
				password_hash VARCHAR(255) NOT NULL,
				role VARCHAR(32) NOT NULL DEFAULT 'user',
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE TABLE IF NOT EXISTS movies (
				id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
				title TEXT NOT NULL,
				file_name TEXT NOT NULL,
				file_path TEXT NOT NULL,
				poster_url TEXT,
				description TEXT,
				genre TEXT,
				duration TEXT,
				resolution TEXT,
				format TEXT,
				file_size BIGINT DEFAULT 0,
				created_at VARCHAR(40) NOT NULL,
				updated_at VARCHAR(40) NOT NULL
			)`,
			`CREATE TABLE IF NOT EXISTS folders (
				id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
				path TEXT NOT NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				UNIQUE KEY folders_path_unique (path(191))
			)`,
			`CREATE TABLE IF NOT EXISTS watch_progress (
				movie_id BIGINT NOT NULL,
				user_id BIGINT NOT NULL,
				position DOUBLE NOT NULL DEFAULT 0,
				updated_at VARCHAR(40) NOT NULL,
				PRIMARY KEY (movie_id, user_id)
			)`,
			`CREATE TABLE IF NOT EXISTS my_list (
				user_id BIGINT NOT NULL,
				movie_id BIGINT NOT NULL,
				added_at VARCHAR(40) NOT NULL,
				PRIMARY KEY (user_id, movie_id)
			)`,
		}
	}
	for _, q := range queries {
		if _, err := d.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func (d *DB) SeedAdmin() error {
	return d.SeedAdminWithCredentials("admin", "admin123")
}

func (d *DB) SeedAdminWithCredentials(username, password string) error {
	var count int
	if err := d.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	if username == "" || len(password) < 8 || len(password) > 72 {
		return fmt.Errorf("admin username is required and admin password must be between 8 and 72 bytes")
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = d.Exec(
		"INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)",
		username,
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

func (d *DB) ListUsers() ([]models.User, error) {
	rows, err := d.Query("SELECT id, username, role, created_at FROM users ORDER BY created_at, id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]models.User, 0)
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.Username, &user.Role, &user.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (d *DB) DeleteMember(id int64) (bool, error) {
	tx, err := d.Begin()
	if err != nil {
		return false, err
	}
	result, err := tx.Exec("DELETE FROM users WHERE id = ? AND role IN ('user', 'member')", id)
	if err != nil {
		_ = tx.Rollback()
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return false, err
	}
	if affected == 0 {
		if err := tx.Rollback(); err != nil {
			return false, err
		}
		return false, nil
	}
	if _, err := tx.Exec("DELETE FROM watch_progress WHERE user_id = ?", id); err != nil {
		_ = tx.Rollback()
		return false, err
	}
	if _, err := tx.Exec("DELETE FROM my_list WHERE user_id = ?", id); err != nil {
		_ = tx.Rollback()
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (d *DB) AddFolder(path string) error {
	query := "INSERT OR IGNORE INTO folders (path) VALUES (?)"
	if d.driver == "mysql" {
		query = "INSERT IGNORE INTO folders (path) VALUES (?)"
	}
	_, err := d.Exec(query, path)
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
	query := `INSERT INTO watch_progress (movie_id, user_id, position, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(movie_id, user_id) DO UPDATE SET position = excluded.position, updated_at = excluded.updated_at`
	if d.driver == "mysql" {
		query = `INSERT INTO watch_progress (movie_id, user_id, position, updated_at)
			VALUES (?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE position = VALUES(position), updated_at = VALUES(updated_at)`
	}
	_, err := d.Exec(
		query,
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

func (d *DB) AddToMyList(userID, movieID int64) error {
	query := "INSERT OR IGNORE INTO my_list (user_id, movie_id, added_at) VALUES (?, ?, ?)"
	if d.driver == "mysql" {
		query = "INSERT IGNORE INTO my_list (user_id, movie_id, added_at) VALUES (?, ?, ?)"
	}
	_, err := d.Exec(
		query,
		userID,
		movieID,
		time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

func (d *DB) RemoveFromMyList(userID, movieID int64) error {
	_, err := d.Exec("DELETE FROM my_list WHERE user_id = ? AND movie_id = ?", userID, movieID)
	return err
}

func (d *DB) ListMyListMovies(userID int64) ([]models.Movie, error) {
	rows, err := d.Query(`
		SELECT m.id, m.title, m.file_name, m.file_path, m.poster_url, m.description, m.genre, m.duration, m.resolution, m.format, m.file_size, m.created_at, m.updated_at
		FROM movies m
		JOIN my_list ml ON ml.movie_id = m.id
		WHERE ml.user_id = ?
		ORDER BY ml.added_at DESC
	`, userID)
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
	scannedPaths := make(map[string]struct{})
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
		scannedPaths[movie.FilePath] = struct{}{}

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
	if err := d.removeMissingMovies(rootPath, scannedPaths); err != nil {
		return nil, fmt.Errorf("remove missing movies from %s: %w", rootPath, err)
	}
	return movies, nil
}

func (d *DB) removeMissingMovies(rootPath string, scannedPaths map[string]struct{}) error {
	rows, err := d.Query("SELECT id, file_path FROM movies")
	if err != nil {
		return err
	}
	type storedMovie struct {
		id       int64
		filePath string
	}
	missing := make([]storedMovie, 0)
	for rows.Next() {
		var movie storedMovie
		if err := rows.Scan(&movie.id, &movie.filePath); err != nil {
			_ = rows.Close()
			return err
		}
		if _, exists := scannedPaths[filepath.Clean(movie.filePath)]; exists {
			continue
		}
		absolutePath, err := filepath.Abs(movie.filePath)
		if err != nil {
			_ = rows.Close()
			return err
		}
		relativePath, err := filepath.Rel(rootPath, absolutePath)
		if err != nil || relativePath == "." || relativePath == ".." ||
			strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) || filepath.IsAbs(relativePath) {
			continue
		}
		missing = append(missing, movie)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(missing) == 0 {
		return nil
	}

	tx, err := d.Begin()
	if err != nil {
		return err
	}
	for _, movie := range missing {
		if _, err := tx.Exec("DELETE FROM movies WHERE id = ?", movie.id); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}
