package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/guardian/backend/internal/api"
	"github.com/guardian/backend/internal/config"
	"github.com/guardian/backend/internal/db"
	"github.com/guardian/backend/internal/store"
)

func main() {
	cfg := config.MustLoad()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[guardian] postgres connect: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(pool, cfg.DatabaseURL); err != nil {
		log.Fatalf("[guardian] migrate: %v", err)
	}
	log.Printf("[guardian] migrations ok")

	router := api.NewRouter(api.Deps{
		AccessToken:    cfg.AccessToken,
		ServersStore:   store.NewServers(pool),
		ConsoleBaseURL: cfg.ConsoleBaseURL,
	})
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("[guardian] listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[guardian] http: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("[guardian] shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
