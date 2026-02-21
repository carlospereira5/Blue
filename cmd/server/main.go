package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"blue/internal/api"
	"blue/internal/config"
	"blue/internal/db"
	"blue/internal/loyverse"
	"blue/internal/repository"
	syncsvc "blue/internal/sync"
)

func main() {
	// 1. Config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// 2. DB — abre conexión y aplica migraciones automáticamente
	database, err := db.Open(cfg.DSN())
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer database.Close()
	log.Println("db: conectado y migraciones aplicadas")

	// 3. Repos
	itemRepo := repository.NewItemRepository(database)
	receiptRepo := repository.NewReceiptRepository(database)
	syncRepo := repository.NewSyncRepository(database)

	// 4. Loyverse client + sync service
	lvClient := loyverse.NewClient(nil, cfg.LoyverseAPIKey)
	svc := syncsvc.New(lvClient, itemRepo, receiptRepo, syncRepo)

	ctx := context.Background()

	// 5. Sincronización inicial del catálogo (solo si es primera ejecución)
	itemsCursor, err := syncRepo.GetSyncCursor(ctx, repository.EntityItems)
	if err != nil {
		log.Fatalf("sync cursor items: %v", err)
	}
	if itemsCursor.IsZero() {
		if err := svc.InitialSync(ctx); err != nil {
			log.Fatalf("initial sync: %v", err)
		}
	}

	// 6. Catch-up de receipts (non-fatal: log warning, no crash)
	if err := svc.CatchUpReceipts(ctx); err != nil {
		log.Printf("WARN catch-up receipts: %v — continuando de todas formas", err)
	}

	// 7. Servidor HTTP
	srv := api.NewServer(database, cfg.WebhookSecret, receiptRepo)
	httpServer := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: srv.Handler(),
	}

	go func() {
		log.Printf("escuchando en :%s", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server: %v", err)
		}
	}()

	// 8. Graceful shutdown — esperar SIGINT o SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("apagando servidor...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown forzado: %v", err)
	}
	log.Println("servidor apagado")
}
