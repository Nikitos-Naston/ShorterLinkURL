package app

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"LinkShrink/internal/config"
	"LinkShrink/internal/handler"
	"LinkShrink/internal/repository"
	"LinkShrink/internal/service"
	"LinkShrink/internal/storage"
)

func Run(cfg config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	var repo service.Repository
	switch cfg.Storage {
	case "memory":
		repo = repository.NewMemory()
	case "postgres":
		if cfg.DatabaseURL == "" {
			return errors.New("DATABASE_URL is required for postgres")
		}
		startup, cancel := context.WithTimeout(ctx, 10*time.Second)
		pool, err := storage.NewPostgres(startup, cfg.DatabaseURL)
		cancel()
		if err != nil {
			return err
		}
		defer pool.Close()
		repo = repository.NewPostgres(pool)
	default:
		return errors.New("storage must be memory or postgres")
	}
	server := &http.Server{
		Addr:              ":8080",
		Handler:           handler.New(service.New(repo), cfg.BaseURL),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	errs := make(chan error, 1)
	go func() { errs <- server.ListenAndServe() }()
	log.Printf("listening on :8080, storage=%s", cfg.Storage)
	select {
	case err := <-errs:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		stop()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			_ = server.Close()
			return err
		}
		return nil
	}
}
