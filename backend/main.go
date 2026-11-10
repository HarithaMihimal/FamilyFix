package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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

	database, err := db.New(cfg.DataDir)
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

	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	r.Get("/api/network", func(w http.ResponseWriter, r *http.Request) {
		info := detectNetworkInfo(cfg.Port)
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
		if err := auth.Login(w, r, database, payload.Username, payload.Password); err != nil {
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
		user, err := auth.CurrentUser(r, database)
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
		r.With(auth.RequireAuth(database)).Get("/movies", func(w http.ResponseWriter, r *http.Request) {
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

		r.With(auth.RequireAuth(database)).Post("/movies", func(w http.ResponseWriter, r *http.Request) {
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

		r.With(auth.RequireAuth(database)).Get("/movies/{id}", func(w http.ResponseWriter, r *http.Request) {
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

		r.With(auth.RequireAuth(database)).Get("/movies/{id}/stream", func(w http.ResponseWriter, r *http.Request) {
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

		r.With(auth.RequireAuth(database)).Get("/movies/{id}/download", func(w http.ResponseWriter, r *http.Request) {
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

		r.With(auth.RequireAuth(database)).Post("/movies/{id}/progress", func(w http.ResponseWriter, r *http.Request) {
			id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
			if err != nil {
				http.Error(w, "Invalid movie id", http.StatusBadRequest)
				return
			}
			user, err := auth.CurrentUser(r, database)
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
	})

	r.With(auth.RequireAdmin(database)).Post("/api/scans", func(w http.ResponseWriter, r *http.Request) {
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

	r.With(auth.RequireAdmin(database)).Get("/api/folders", func(w http.ResponseWriter, r *http.Request) {
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
