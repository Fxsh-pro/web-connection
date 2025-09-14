package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AllowedOrigin string
	DatabaseURL   string
	ListenAddr    string
}

func Load() Config {
	_ = godotenv.Load()
	allowedOrigin := strings.TrimSpace(os.Getenv("ALLOWED_ORIGIN"))
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	listenAddr := strings.TrimSpace(os.Getenv("LISTEN_ADDR"))
	if listenAddr == "" {
		listenAddr = ":8080"
	}
	return Config{AllowedOrigin: allowedOrigin, DatabaseURL: databaseURL, ListenAddr: listenAddr}
}
