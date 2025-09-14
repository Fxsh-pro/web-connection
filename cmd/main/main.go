package main

import (
	"context"
	"log"
	"net/http"

	"webConnector/internal/config"
	"webConnector/internal/httpserver"
	"webConnector/internal/rooms"
	"webConnector/internal/store"
	"webConnector/internal/ws"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg := config.Load()

	manager := rooms.NewManager()

	var dbpool *pgxpool.Pool
	if cfg.DatabaseURL != "" {
		p, err := store.Connect(context.Background(), cfg.DatabaseURL)
		if err != nil {
			log.Fatalf("db connect: %v", err)
		}
		log.Printf("db connected")
		if err := store.Migrate(p); err != nil {
			log.Fatalf("migrate: %v", err)
		}
		log.Printf("db migrated")
		dbpool = p
		defer dbpool.Close()
	}

	if cfg.AllowedOrigin != "" {
		ws.SetAllowedOrigin(cfg.AllowedOrigin)
	}

	http.HandleFunc("/ws", ws.Handler(manager, dbpool))
	webDir := http.FileServer(http.Dir("web"))
	http.Handle("/", http.StripPrefix("/", httpserver.NeuterIndex(webDir)))

	log.Printf("listening on %s", cfg.ListenAddr)
	log.Fatal(http.ListenAndServe(cfg.ListenAddr, nil))
}
