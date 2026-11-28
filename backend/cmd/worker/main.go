// Worker SubSenpai: job latar berjalan sebagai proses terpisah dari API.
//
// Fase 1: menghitung ulang skor tren (O10) secara berkala.
// Fase 2: cek kesehatan registri sumber (F40/O9) + sinkronisasi resolver
// sumber (mengambil episode, aliran & trek subtitle siap pakai).
// Fase 4: polling pembayaran GoPay (pencocokan nominal unik sesi).
// Fase berikutnya menambah: pembersihan retensi dan pengiriman notifikasi.
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
	"subsenpai/backend/internal/payments"
	"subsenpai/backend/internal/sources"
)

func main() {
	cfg := config.Load()
	sources.LogBaseOverrides()
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
		jobs.ReportProgress(ctx, pool, "Mengecek sumber", 0, 0, "")
		if err := jobs.CheckSources(ctx, pool); err != nil {
			log.Printf("cek sumber: %v", err)
		}
		// Enrich + Sync DIDAHULUKAN sebelum penemuan: penemuan katalog
		// lambat (ratusan judul + fetch yang bisa timeout 30 dtk per
		// situs), dan terbukti (log 2026-11-22) membuat enrich/sync
		// kelaparan — siklus selalu keburu mati oleh restart sebelum
		// giliran mereka. Data yang terlihat pengguna diisi dulu.
		if err := jobs.EnrichTitles(ctx, pool); err != nil {
			log.Printf("enrich judul: %v", err)
		}
		if err := jobs.SyncSources(ctx, pool); err != nil {
			log.Printf("sinkron sumber: %v", err)
		}
		// Audit kelengkapan: judul yang masih bolong (metadata,
		// episode, sumber putar) diperbaiki satu per satu menurut
		// prioritas kekurangan + permintaan pengguna.
		if err := jobs.RepairCatalog(ctx, pool); err != nil {
			log.Printf("perbaikan katalog: %v", err)
		}
		if err := jobs.DiscoverTitles(ctx, pool); err != nil {
			log.Printf("penemuan judul: %v", err)
		}
		if err := jobs.PurgePosterless(ctx, pool); err != nil {
			log.Printf("purge poster: %v", err)
		}
		// Pengumuman update APK otomatis: versi rilis GitHub
		// terbaru yang belum diumumkan diposting ke #update-apk
		// (penanda versi di app_settings; no-op bila token kosong).
		jobs.PostUpdateToDiscordIfNew(ctx, pool, cfg)
		jobs.ReportProgress(ctx, pool, "Selesai", 0, 0, "")
	}
	// Klien pembayaran dibangun dulu: job pembayaran jalan SEBELUM
	// siklus katalog. runSources() lambat (satu siklus penuh bisa
	// puluhan menit — fetch ratusan judul) dan sebelumnya job
	// pembayaran baru dipanggil sesudahnya, sehingga sesudah restart
	// worker transaksi basi tetap pending lama padahal kode sweeper
	// sudah live — dan journalctl pun kosong karena sweeper belum
	// pernah dieksekusi sama sekali (kejadian 2026-11-26).
	var gopayClient *payments.GoPayClient
	if cfg.GoPayBaseURL != "" {
		gopayClient = payments.NewGoPayClient(cfg.GoPayBaseURL, cfg.GoPayUsername, cfg.GoPayPassword)
	}
	var tokopayClient *payments.TokopayClient
	if cfg.TokopayMerchantID != "" && cfg.TokopaySecret != "" {
		tokopayClient = payments.NewTokopayClient(cfg.TokopayAPIURL, cfg.TokopayMerchantID, cfg.TokopaySecret)
	}
	runPayments := func() {
		jobs.PollGoPay(ctx, pool, gopayClient)
		jobs.ExpireStaleSessions(ctx, pool, tokopayClient)
		jobs.ReconcileTokopayPayments(ctx, pool, tokopayClient)
	}
	runPayments()
	// Ticker pembayaran di goroutine sendiri: siklus katalog memblokir
	// loop utama selama berjalan, dan tick payTicker yang tiba saat
	// itu hilang — sweeper "tiap 1 menit" nyatanya hanya jalan di
	// sela-sela siklus. Goroutine ini menjaga jadwalnya apa pun yang
	// dikerjakan loop utama. Job-jobnya independen dari katalog (hanya
	// menyentuh payment_sessions + profiles/subscriptions) dan
	// aktivasinya atomik, jadi aman jalan bersamaan.
	go func() {
		payTicker := time.NewTicker(time.Minute)
		defer payTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-payTicker.C:
				runPayments()
			}
		}
	}()
	// Ticker jadwal rilis: pengiriman per judul harus tepat waktu
	// (sesudah jam rilis, WIB) — tidak bisa menunggu akhir siklus
	// katalog yang lamanya puluhan menit. Goroutine sendiri seperti
	// payTicker; penanda per judul di jadwal_notified membuat
	// pemanggilan ganda aman (idempotent).
	go func() {
		jadwalTicker := time.NewTicker(5 * time.Minute)
		defer jadwalTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-jadwalTicker.C:
				jobs.PostJadwalRilis(ctx, pool, cfg)
			}
		}
	}()
	// Ticker health-check kanal TV: probe tiap stream tiap 30 menit;
	// 3x gagal beruntun kanal disembunyikan otomatis sampai hidup
	// lagi. Sekali jalan di awal supaya data kesehatan seed langsung
	// terisi (bukan menunggu 30 menit pertama).
	go func() {
		tvTicker := time.NewTicker(30 * time.Minute)
		defer tvTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tvTicker.C:
				if err := jobs.CheckTVChannels(ctx, pool); err != nil {
					log.Printf("tv health: %v", err)
				}
			}
		}
	}()
	if err := jobs.CheckTVChannels(ctx, pool); err != nil {
		log.Printf("tv health awal: %v", err)
	}
	runSources()
	trendTicker := time.NewTicker(time.Hour)
	defer trendTicker.Stop()
	sourceTicker := time.NewTicker(10 * time.Minute)
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
			// Rapikan perangkat login yang kelebihan batas (mis. akun
			// yang premiumnya habis: 2 → 1) — perangkat tertua dicabut.
			if n, err := jobs.TrimDevices(ctx, pool); err != nil {
				log.Printf("rapikan perangkat: %v", err)
			} else if n > 0 {
				log.Printf("rapikan perangkat: %d dicabut", n)
			}
		case <-sourceTicker.C:
			runSources()
		}
	}
}
