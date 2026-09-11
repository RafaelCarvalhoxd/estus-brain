// Command api is the Estus Vault backend: a single Go binary that owns the
// ledger, applies its own migrations on boot, and serves a small JSON API
// consumed exclusively by the Next.js frontend running alongside it.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rafael/estus-vault/backend/internal/config"
	"github.com/rafael/estus-vault/backend/internal/httpapi"
	"github.com/rafael/estus-vault/backend/internal/service"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	db, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.Migrate(ctx, "migrations"); err != nil {
		return err
	}

	categoryRepo := postgres.NewCategoryRepo(db)
	cardRepo := postgres.NewCreditCardRepo(db)
	transactionRepo := postgres.NewTransactionRepo(db)

	transactionService := service.NewTransactionService(transactionRepo, categoryRepo, cardRepo)
	dashboardService := service.NewDashboardService(transactionRepo)

	handlers := httpapi.NewHandlers(categoryRepo, cardRepo, transactionService, dashboardService)
	router := httpapi.NewRouter(handlers)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("estus vault api listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
