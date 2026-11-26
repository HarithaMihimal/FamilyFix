package config

import (
	"os"
	"strconv"
)

type Config struct {
	Host          string
	Port          int
	DataDir       string
	FrontendDir   string
	SessionKey    string
	DBHost        string
	DBPort        int
	DBUser        string
	DBPassword    string
	DBName        string
	AdminUsername string
	AdminPassword string
}

func Load() Config {
	port, _ := strconv.Atoi(os.Getenv("FAMILYFLIX_PORT"))
	if port == 0 {
		port = 3000
	}

	dbPort, _ := strconv.Atoi(os.Getenv("DB_PORT"))
	if dbPort == 0 {
		dbPort = 3306
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
		Host:          os.Getenv("FAMILYFLIX_HOST"),
		Port:          port,
		DataDir:       dataDir,
		FrontendDir:   frontendDir,
		SessionKey:    os.Getenv("FAMILYFLIX_SESSION_KEY"),
		DBHost:        os.Getenv("DB_HOST"),
		DBPort:        dbPort,
		DBUser:        os.Getenv("DB_USER"),
		DBPassword:    os.Getenv("DB_PASSWORD"),
		DBName:        os.Getenv("DB_NAME"),
		AdminUsername: os.Getenv("FAMILYFLIX_ADMIN_USERNAME"),
		AdminPassword: os.Getenv("FAMILYFLIX_ADMIN_PASSWORD"),
	}
	if cfg.Host == "" {
		cfg.Host = "0.0.0.0"
	}
	return cfg
}
