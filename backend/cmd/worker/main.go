// Worker SubSenpai: job latar berjalan sebagai proses terpisah dari API.
//
// Fase 1: menghitung ulang skor tren (O10) secara berkala.
// Fase 2: cek kesehatan registri sumber (F40/O9) + sinkronisasi resolver
// sumber (mengambil episode, aliran & trek subtitle siap pakai).
// Fase berikutnya menambah: polling pembayaran GoPay, pembersihan
// retensi, dan pengiriman notifikasi.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"subsenpai/backend/internal/config"
	"subsenpai/backend/internal/db"
	"subsenpai/backend/internal/jobs"
)

func main() {
	cfg := config.Load()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		cancel()
	}()

	log.Println("worker SubSenpai jalan")
	if err := jobs.RecomputeTrending(ctx, pool); err != nil {
		log.Printf("tren awal: %v", err)
	}
	runSources := func() {
		if err := jobs.CheckSources(ctx, pool); err != nil {
			log.Printf("cek sumber: %v", err)
		}
		if err := jobs.SyncSources(ctx, pool); err != nil {
			log.Printf("sinkron sumber: %v", err)
		}
	}
	runSources()
	trendTicker := time.NewTicker(time.Hour)
	defer trendTicker.Stop()
	sourceTicker := time.NewTicker(30 * time.Minute)
	defer sourceTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Println("worker berhenti")
			return
		case <-trendTicker.C:
			if err := jobs.RecomputeTrending(ctx, pool); err != nil {
				log.Printf("hitung tren: %v", err)
			}
		case <-sourceTicker.C:
			runSources()
		}
	}
}
