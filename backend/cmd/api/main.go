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

	billRepo := postgres.NewBillRepo(db)
	billService := service.NewBillService(billRepo, categoryRepo)
	billHandlers := httpapi.NewBillHandlers(billService)

	noteRepo := postgres.NewNoteRepo(db)
	noteService := service.NewNoteService(noteRepo)
	noteHandlers := httpapi.NewNoteHandlers(noteService)

	reminderRepo := postgres.NewReminderRepo(db)
	reminderService := service.NewReminderService(reminderRepo)
	reminderHandlers := httpapi.NewReminderHandlers(reminderService)

	eventRepo := postgres.NewEventRepo(db)
	googleTokenRepo := postgres.NewGoogleTokenRepo(db)
	googleService := service.NewGoogleService(googleTokenRepo)
	eventService := service.NewEventService(eventRepo, googleService)
	eventHandlers := httpapi.NewEventHandlers(eventService, googleService)

	// The vault module needs VAULT_ENCRYPTION_KEY and the WEBAUTHN_* env vars
	// to exist at all — treat their absence as "module disabled" rather than
	// a fatal boot error, so the rest of the app still comes up on a fresh
	// checkout before those are configured.
	var vaultHandlers *httpapi.VaultHandlers
	vaultRepo := postgres.NewVaultRepo(db)
	webauthnCredRepo := postgres.NewWebAuthnCredentialRepo(db)
	vaultService, vaultErr := service.NewVaultService(vaultRepo)
	webauthnService, webauthnErr := service.NewWebAuthnService(webauthnCredRepo)
	if vaultErr != nil || webauthnErr != nil {
		slog.Warn("vault module disabled: set VAULT_ENCRYPTION_KEY and WEBAUTHN_* to enable /senhas", "vault_error", vaultErr, "webauthn_error", webauthnErr)
	} else {
		vaultHandlers = httpapi.NewVaultHandlers(vaultService, webauthnService)
	}

	router := httpapi.NewRouter(handlers, httpapi.Modules{
		Bills:     billHandlers,
		Vault:     vaultHandlers,
		Notes:     noteHandlers,
		Reminders: reminderHandlers,
		Events:    eventHandlers,
	})

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
