// Command api is the Estus Brain backend: a single Go binary that owns the
// ledger, applies its own migrations on boot, and serves a small JSON API
// consumed exclusively by the Next.js frontend running alongside it.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	// Bundled so the owner's time zone resolves even on a minimal container.
	_ "time/tzdata"

	"github.com/rafael/estus-vault/backend/internal/assistant"
	"github.com/rafael/estus-vault/backend/internal/config"
	"github.com/rafael/estus-vault/backend/internal/domain"
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
	billService := service.NewBillService(billRepo, categoryRepo, cardRepo)
	dashboardService.WithOpenFixedBills(billService.OpenFixedTotal)
	billHandlers := httpapi.NewBillHandlers(billService)

	noteRepo := postgres.NewNoteRepo(db)
	noteService := service.NewNoteService(noteRepo)
	noteCategoryRepo := postgres.NewNoteCategoryRepo(db)
	noteCategoryService := service.NewNoteCategoryService(noteCategoryRepo)
	noteHandlers := httpapi.NewNoteHandlers(noteService, noteCategoryService)

	location, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return fmt.Errorf("ASSISTANT_TZ: %w", err)
	}

	cardSpendingService := service.NewCardSpendingService(postgres.NewCardSpendingRepo(db), cardRepo, location)

	reminderRepo := postgres.NewReminderRepo(db)
	reminderService := service.NewReminderService(reminderRepo, location)
	reminderHandlers := httpapi.NewReminderHandlers(reminderService)

	eventRepo := postgres.NewEventRepo(db)
	eventService := service.NewEventService(eventRepo)
	eventHandlers := httpapi.NewEventHandlers(eventService)

	trainingService := service.NewTrainingService(postgres.NewWorkoutRepo(db))
	trainingHandlers := httpapi.NewTrainingHandlers(trainingService)

	dietService := service.NewDietService(postgres.NewMealRepo(db))
	dietHandlers := httpapi.NewDietHandlers(dietService)

	habitService := service.NewHabitService(postgres.NewHabitRepo(db))
	habitHandlers := httpapi.NewHabitHandlers(habitService)

	boardService := service.NewBoardService(postgres.NewBoardRepo(db))
	boardHandlers := httpapi.NewBoardHandlers(boardService)

	// Documents keep their bytes on disk; if that directory can't be opened
	// the module switches off rather than taking the whole API down with it.
	var documentHandlers *httpapi.DocumentHandlers
	documentFolderRepo := postgres.NewDocumentFolderRepo(db)
	documentRepo := postgres.NewDocumentRepo(db)
	documentService, documentErr := service.NewDocumentService(documentFolderRepo, documentRepo, cfg.DocumentsDir)
	if documentErr != nil {
		slog.Warn("documents module disabled: storage directory unavailable", "dir", cfg.DocumentsDir, "error", documentErr)
	} else {
		slog.Info("documents storage ready", "dir", documentService.Root())
		documentHandlers = httpapi.NewDocumentHandlers(documentService)
	}

	// The vault module needs VAULT_ENCRYPTION_KEY to exist at all — treat its
	// absence as "module disabled" rather than a fatal boot error, so the
	// rest of the app still comes up on a fresh checkout before it's
	// configured.
	var vaultHandlers *httpapi.VaultHandlers
	vaultRepo := postgres.NewVaultRepo(db)
	vaultService, vaultErr := service.NewVaultService(vaultRepo)
	if vaultErr != nil {
		slog.Warn("vault module disabled: set VAULT_ENCRYPTION_KEY to enable /senhas", "vault_error", vaultErr)
	} else {
		vaultHandlers = httpapi.NewVaultHandlers(vaultService, cfg.AppPassword)
	}

	// The assistant: every action as a tool, shared by the chat's agent and
	// MCP. The vault stays out of it.
	mcpToken, err := assistant.LoadOrCreateToken(cfg.AssistantDir, cfg.MCPToken)
	if err != nil {
		return fmt.Errorf("assistant token: %w", err)
	}
	chatCfg := assistant.ChatConfig{
		MCPURL:    "http://127.0.0.1:" + cfg.Port + "/mcp",
		MCPToken:  mcpToken,
		Env:       assistant.NewAgentEnv(cfg.AgentURL, cfg.AgentToken, cfg.AgentModel),
		Documents: documentServiceOrNil(documentService, documentErr),
	}
	if key, err := domain.LoadEncryptionKeyFromEnv(); err == nil {
		chatCfg.VaultKey = &key
	}
	assistantRepo := postgres.NewAssistantRepo(db)
	tools := assistant.New(assistant.Deps{
		Categories:     categoryRepo,
		Cards:          cardRepo,
		Transactions:   transactionService,
		TransactionLog: transactionRepo,
		Dashboard:      dashboardService,
		Bills:          billService,
		CardSpending:   cardSpendingService,
		Notes:          noteService,
		NoteCategories: noteCategoryService,
		Reminders:      reminderService,
		Events:         eventService,
		Habits:         habitService,
		Training:       trainingService,
		Diet:           dietService,
		Documents:      documentServiceOrNil(documentService, documentErr),
		Boards:         boardService,
		Location:       location,
	})
	chat := assistant.NewChat(tools, assistantRepo, chatCfg)
	assistantHandlers := httpapi.NewAssistantHandlers(tools, chat, documentServiceOrNil(documentService, documentErr))
	mcpHandler := tools.MCPHandler(func() []string { return []string{mcpToken} })
	slog.Info("assistant ready", "tools", len(tools.Tools()), "mcp", "/mcp")

	router := httpapi.NewRouter(handlers, httpapi.Modules{
		Bills:        billHandlers,
		Vault:        vaultHandlers,
		Notes:        noteHandlers,
		Reminders:    reminderHandlers,
		Events:       eventHandlers,
		Documents:    documentHandlers,
		Training:     trainingHandlers,
		Diet:         dietHandlers,
		Boards:       boardHandlers,
		Habits:       habitHandlers,
		Assistant:    assistantHandlers,
		MCP:          mcpHandler,
		CardSpending: httpapi.NewCardSpendingHandlers(cardSpendingService),
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

	var serveErr error
	select {
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		serveErr = srv.Shutdown(shutdownCtx)
	case serveErr = <-errCh:
	}
	return serveErr
}

// documentServiceOrNil keeps a disabled documents module out of the assistant.
func documentServiceOrNil(s *service.DocumentService, err error) *service.DocumentService {
	if err != nil {
		return nil
	}
	return s
}
