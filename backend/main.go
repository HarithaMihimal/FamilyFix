package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"familyflix/internal/auth"
	"familyflix/internal/config"
	"familyflix/internal/db"
	"familyflix/internal/media"
	"familyflix/internal/models"
	"familyflix/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	cfg := config.Load()
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatalf("could not create data dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(cfg.DataDir, "movies"), 0o755); err != nil {
		log.Fatalf("could not create movies dir: %v", err)
	}

	sessionKey, err := loadSessionKey(cfg.DataDir, cfg.SessionKey)
	if err != nil {
		log.Fatalf("could not load session signing key: %v", err)
	}

	var database *db.DB
	if cfg.DBHost != "" {
		database, err = db.NewWithMySQL(cfg.DataDir, db.MySQLConfig{
			Host: cfg.DBHost, Port: cfg.DBPort, User: cfg.DBUser,
			Password: cfg.DBPassword, Name: cfg.DBName,
			AdminUsername: cfg.AdminUsername, AdminPassword: cfg.AdminPassword,
		})
	} else {
		database, err = db.New(cfg.DataDir)
	}
	if err != nil {
		log.Fatalf("could not initialize database: %v", err)
	}
	defer database.Close()

	moviesDir := filepath.Join(cfg.DataDir, "movies")
	importedMovies, err := database.ScanFolder(moviesDir)
	if err != nil {
		log.Fatalf("could not scan movie library: %v", err)
	}
	if len(importedMovies) > 0 {
		log.Printf("Imported %d movie(s) from %s", len(importedMovies), moviesDir)
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(allowLocalNetwork)
	devices := newDeviceTracker()
	r.Use(devices.middleware)

	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	r.Get("/api/network", func(w http.ResponseWriter, r *http.Request) {
		info := detectNetworkInfo(cfg.Port)
		info.ActiveDevices = devices.count(time.Now())
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(info)
	})

	r.Post("/api/login", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid login payload", http.StatusBadRequest)
			return
		}
		if payload.Username == "" || payload.Password == "" {
			http.Error(w, "Username and password are required", http.StatusBadRequest)
			return
		}
		if err := auth.Login(w, r, database, sessionKey, payload.Username, payload.Password); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	r.Post("/api/logout", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "familyflix_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
		w.WriteHeader(http.StatusNoContent)
	})

	r.Get("/api/session", func(w http.ResponseWriter, r *http.Request) {
		user, err := auth.CurrentUser(r, database, sessionKey)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"authenticated": "false"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"authenticated": true, "user": user})
	})

	r.Route("/api", func(r chi.Router) {
		r.With(auth.RequireAdmin(database, sessionKey)).Get("/admin/users", func(w http.ResponseWriter, r *http.Request) {
			users, err := database.ListUsers()
			if err != nil {
				http.Error(w, "Could not load users", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(users)
		})

		r.With(auth.RequireAdmin(database, sessionKey)).Post("/admin/users", func(w http.ResponseWriter, r *http.Request) {
			var payload struct {
				Username string `json:"username"`
				Password string `json:"password"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, "Invalid user payload", http.StatusBadRequest)
				return
			}
			payload.Username = strings.TrimSpace(payload.Username)
			if !validUsername(payload.Username) {
				http.Error(w, "Username must be 3-32 characters and contain only letters, numbers, dots, underscores, or hyphens", http.StatusBadRequest)
				return
			}
			if len(payload.Password) < 8 || len(payload.Password) > 72 {
				http.Error(w, "Password must be between 8 and 72 bytes", http.StatusBadRequest)
				return
			}
			if _, err := database.GetUserByUsername(payload.Username); err == nil {
				http.Error(w, "Username is already taken", http.StatusConflict)
				return
			} else if !errors.Is(err, sql.ErrNoRows) {
				log.Printf("could not check username availability: %v", err)
				http.Error(w, "Could not check username availability", http.StatusInternalServerError)
				return
			}
			user, err := database.CreateUser(payload.Username, payload.Password, "user")
			if err != nil {
				http.Error(w, "Could not create user", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(user)
		})

		r.With(auth.RequireAdmin(database, sessionKey)).Delete("/admin/users/{id}", func(w http.ResponseWriter, r *http.Request) {
			id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
			if err != nil || id < 1 {
				http.Error(w, "Invalid user id", http.StatusBadRequest)
				return
			}
			deleted, err := database.DeleteMember(id)
			if err != nil {
				http.Error(w, "Could not delete user", http.StatusInternalServerError)
				return
			}
			if !deleted {
				http.Error(w, "Normal user not found", http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})

		r.With(auth.RequireAuth(database, sessionKey)).Get("/movies", func(w http.ResponseWriter, r *http.Request) {
			if _, err := database.ScanFolder(moviesDir); err != nil {
				log.Printf("could not refresh movie library: %v", err)
				http.Error(w, "Could not refresh movie library", http.StatusInternalServerError)
				return
			}
			movies, err := database.ListMovies()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(movies)
		})

		r.With(auth.RequireAdmin(database, sessionKey)).Post("/movies", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				http.Error(w, "Upload too large or invalid multipart payload", http.StatusBadRequest)
				return
			}
			file, header, err := r.FormFile("file")
			if err != nil {
				http.Error(w, "No movie file provided", http.StatusBadRequest)
				return
			}
			defer file.Close()
			title := r.FormValue("title")
			if title == "" {
				title = db.NormalizeTitle(header.Filename)
			}
			targetDir := filepath.Join(cfg.DataDir, "movies")
			savedPath, err := storage.SaveTempUpload(file, header, targetDir)
			if err != nil {
				http.Error(w, "Could not save uploaded file", http.StatusInternalServerError)
				return
			}
			if err := storage.ValidateMovieFile(savedPath); err != nil {
				_ = os.Remove(savedPath)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			info, err := os.Stat(savedPath)
			if err != nil {
				http.Error(w, "Could not stat uploaded file", http.StatusInternalServerError)
				return
			}
			movie := models.Movie{
				Title:       title,
				FileName:    header.Filename,
				FilePath:    savedPath,
				Format:      db.FileFormat(savedPath),
				FileSize:    info.Size(),
				Genre:       "Uncategorized",
				Description: "Uploaded from the browser",
			}
			id, err := database.SaveMovie(movie)
			if err != nil {
				http.Error(w, fmt.Sprintf("Could not save movie metadata: %v", err), http.StatusInternalServerError)
				return
			}
			movie.ID = id
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(movie)
		})

		r.With(auth.RequireAuth(database, sessionKey)).Get("/movies/{id}", func(w http.ResponseWriter, r *http.Request) {
			id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
			if err != nil {
				http.Error(w, "Invalid movie id", http.StatusBadRequest)
				return
			}
			movie, err := database.GetMovie(id)
			if err != nil {
				http.Error(w, "Movie not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(movie)
		})

		r.With(auth.RequireAuth(database, sessionKey)).Get("/movies/{id}/stream", func(w http.ResponseWriter, r *http.Request) {
			id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
			if err != nil {
				http.Error(w, "Invalid movie id", http.StatusBadRequest)
				return
			}
			movie, err := database.GetMovie(id)
			if err != nil {
				http.Error(w, "Movie not found", http.StatusNotFound)
				return
			}
			media.ServeMovie(w, r, movie.FilePath)
		})

		r.With(auth.RequireAuth(database, sessionKey)).Get("/movies/{id}/download", func(w http.ResponseWriter, r *http.Request) {
			id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
			if err != nil {
				http.Error(w, "Invalid movie id", http.StatusBadRequest)
				return
			}
			movie, err := database.GetMovie(id)
			if err != nil {
				http.Error(w, "Movie not found", http.StatusNotFound)
				return
			}
			media.DownloadMovie(w, r, movie.FilePath)
		})

		r.With(auth.RequireAuth(database, sessionKey)).Get("/progress", func(w http.ResponseWriter, r *http.Request) {
			user, err := auth.CurrentUser(r, database, sessionKey)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			progress, err := database.GetWatchProgress(user.ID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(progress)
		})

		r.With(auth.RequireAuth(database, sessionKey)).Post("/movies/{id}/progress", func(w http.ResponseWriter, r *http.Request) {
			id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
			if err != nil {
				http.Error(w, "Invalid movie id", http.StatusBadRequest)
				return
			}
			user, err := auth.CurrentUser(r, database, sessionKey)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			var payload struct {
				Position float64 `json:"position"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, "Invalid progress payload", http.StatusBadRequest)
				return
			}
			if err := database.SaveWatchProgress(user.ID, id, payload.Position); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})

		r.With(auth.RequireAuth(database, sessionKey)).Get("/my-list", func(w http.ResponseWriter, r *http.Request) {
			user, err := auth.CurrentUser(r, database, sessionKey)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			movies, err := database.ListMyListMovies(user.ID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(movies)
		})

		r.With(auth.RequireAuth(database, sessionKey)).Post("/my-list/{id}", func(w http.ResponseWriter, r *http.Request) {
			id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
			if err != nil {
				http.Error(w, "Invalid movie id", http.StatusBadRequest)
				return
			}
			user, err := auth.CurrentUser(r, database, sessionKey)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			if err := database.AddToMyList(user.ID, id); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})

		r.With(auth.RequireAuth(database, sessionKey)).Delete("/my-list/{id}", func(w http.ResponseWriter, r *http.Request) {
			id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
			if err != nil {
				http.Error(w, "Invalid movie id", http.StatusBadRequest)
				return
			}
			user, err := auth.CurrentUser(r, database, sessionKey)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			if err := database.RemoveFromMyList(user.ID, id); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
	})

	r.With(auth.RequireAdmin(database, sessionKey)).Post("/api/scans", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}
		if payload.Path == "" {
			http.Error(w, "Folder path required", http.StatusBadRequest)
			return
		}
		if err := database.AddFolder(payload.Path); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		movies, err := database.ScanFolder(payload.Path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(movies)
	})

	r.With(auth.RequireAdmin(database, sessionKey)).Get("/api/folders", func(w http.ResponseWriter, r *http.Request) {
		folders, err := database.ListFolders()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(folders)
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		indexPath := filepath.Join(cfg.FrontendDir, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			http.ServeFile(w, r, indexPath)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><body><h1>FamilyFlix</h1><p>Frontend not built yet. Run npm install and npm run build in frontend.</p></body></html>"))
	})

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		indexPath := filepath.Join(cfg.FrontendDir, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			http.ServeFile(w, r, indexPath)
			return
		}
		http.NotFound(w, r)
	})

	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	log.Printf("FamilyFlix listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

const activeDeviceWindow = 2 * time.Minute

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{3,32}$`)

func validUsername(username string) bool {
	return usernamePattern.MatchString(username)
}

func loadSessionKey(dataDir, configured string) ([]byte, error) {
	if configured != "" {
		if len(configured) < 32 {
			return nil, errors.New("FAMILYFLIX_SESSION_KEY must be at least 32 bytes")
		}
		return []byte(configured), nil
	}

	keyPath := filepath.Join(dataDir, "session.key")
	key, err := os.ReadFile(keyPath)
	if err == nil {
		if len(key) < 32 {
			return nil, errors.New("stored session key must be at least 32 bytes")
		}
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	key = make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			existingKey, readErr := os.ReadFile(keyPath)
			if readErr != nil {
				return nil, readErr
			}
			if len(existingKey) < 32 {
				return nil, errors.New("stored session key must be at least 32 bytes")
			}
			return existingKey, nil
		}
		return nil, err
	}
	if _, err := file.Write(key); err != nil {
		_ = file.Close()
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	return key, nil
}

type deviceTracker struct {
	mu       sync.Mutex
	lastSeen map[string]time.Time
}

func newDeviceTracker() *deviceTracker {
	return &deviceTracker{lastSeen: make(map[string]time.Time)}
}

func (d *deviceTracker) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ip := requestClientIP(r); ip != "" {
			d.mu.Lock()
			d.lastSeen[ip] = time.Now()
			d.mu.Unlock()
		}
		next.ServeHTTP(w, r)
	})
}

func (d *deviceTracker) count(now time.Time) int {
	d.mu.Lock()
	defer d.mu.Unlock()

	count := 0
	for ip, lastSeen := range d.lastSeen {
		if now.Sub(lastSeen) > activeDeviceWindow {
			delete(d.lastSeen, ip)
			continue
		}
		count++
	}
	return count
}

func requestClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return ""
	}

	if ip.IsLoopback() {
		forwardedAddresses := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
		for i := len(forwardedAddresses) - 1; i >= 0; i-- {
			forwardedIP := net.ParseIP(strings.TrimSpace(forwardedAddresses[i]))
			if forwardedIP != nil {
				return forwardedIP.String()
			}
		}
	}
	return ip.String()
}

func allowLocalNetwork(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func detectNetworkInfo(port int) models.NetworkInfo {
	interfaces, _ := net.Interfaces()
	addresses := make([]string, 0)
	interfaceNames := make([]string, 0)
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || strings.Contains(iface.Name, "lo") {
			continue
		}
		interfaceNames = append(interfaceNames, iface.Name)
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok || ipNet.IP.IsLoopback() {
				continue
			}
			if ip := ipNet.IP.To4(); ip != nil {
				address := fmt.Sprintf("http://%s:%d", ip.String(), port)
				addresses = append(addresses, address)
			}
		}
	}
	if len(addresses) == 0 {
		addresses = append(addresses, fmt.Sprintf("http://127.0.0.1:%d", port))
	}
	return models.NetworkInfo{
		Host:        "0.0.0.0",
		Port:        port,
		URLs:        addresses,
		Interfaces:  interfaceNames,
		LocalServer: addresses[0],
	}
}
