package config

import (
	"os"
	"strconv"
)

type Config struct {
	Host         string
	Port         int
	DataDir      string
	FrontendDir  string
	SessionKey   string
}

func Load() Config {
	port, _ := strconv.Atoi(os.Getenv("FAMILYFLIX_PORT"))
	if port == 0 {
		port = 3000
	}

	dataDir := os.Getenv("FAMILYFLIX_DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	frontendDir := os.Getenv("FAMILYFLIX_FRONTEND_DIR")
	if frontendDir == "" {
		frontendDir = "../frontend/dist"
	}

	cfg := Config{
		Host:        os.Getenv("FAMILYFLIX_HOST"),
		Port:        port,
		DataDir:     dataDir,
		FrontendDir: frontendDir,
		SessionKey:  os.Getenv("FAMILYFLIX_SESSION_KEY"),
	}
	if cfg.Host == "" {
		cfg.Host = "0.0.0.0"
	}
	if cfg.SessionKey == "" {
		cfg.SessionKey = "familyflix-dev-session-key"
	}
	return cfg
}
