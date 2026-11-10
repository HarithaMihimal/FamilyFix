package models

type User struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"`
	CreatedAt    string `json:"created_at"`
}

type Movie struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	FileName    string `json:"file_name"`
	FilePath    string `json:"file_path"`
	PosterURL   string `json:"poster_url"`
	Description string `json:"description"`
	Genre       string `json:"genre"`
	Duration    string `json:"duration"`
	Resolution  string `json:"resolution"`
	Format      string `json:"format"`
	FileSize    int64  `json:"file_size"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type Folder struct {
	ID        int64  `json:"id"`
	Path      string `json:"path"`
	CreatedAt string `json:"created_at"`
}

type NetworkInfo struct {
	Host        string   `json:"host"`
	Port        int      `json:"port"`
	URLs        []string `json:"urls"`
	Interfaces  []string `json:"interfaces"`
	LocalServer string   `json:"local_server"`
}

type WatchProgress struct {
	MovieID int64  `json:"movie_id"`
	UserID  int64  `json:"user_id"`
	Position float64 `json:"position"`
	UpdatedAt string `json:"updated_at"`
}
