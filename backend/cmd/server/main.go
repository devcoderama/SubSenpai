// Server API SubSenpai: layanan HTTP Go di VPS (di belakang Caddy).
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"subsenpai/backend/internal/config"
	"subsenpai/backend/internal/db"
	"subsenpai/backend/internal/httpapi"
	"subsenpai/backend/internal/middleware"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("migrasi: %v", err)
	}
	log.Println("migrasi skema beres")

	sessions := middleware.NewSessions(time.Duration(cfg.SessionTTLHours) * time.Hour)
	api := httpapi.New(pool, sessions)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           api.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		log.Printf("API SubSenpai mendengarkan di :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	log.Println("server berhenti")
}
