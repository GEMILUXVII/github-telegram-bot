package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/user/githubbot/internal/config"
	"github.com/user/githubbot/internal/github"
	"github.com/user/githubbot/internal/notifier"
	"github.com/user/githubbot/internal/storage"
	"github.com/user/githubbot/internal/telegram"
	"github.com/user/githubbot/pkg/logger"
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		// Try to initialize basic logger for error output
		logger.Init(true, "")
		logger.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Initialize logger
	debug := cfg.Log.Level == "debug"
	if err := logger.Init(debug, cfg.Log.File); err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}

	logger.Info().Msg("Starting GitHub Telegram Bot")
	logger.Info().Str("mode", cfg.GitHub.Mode).Msg("GitHub monitoring mode")

	// Initialize database
	db, err := storage.NewDatabase(cfg.Database.Path)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize database")
	}
	defer db.Close()

	store := storage.NewSubscriptionStore(db)
	logger.Info().Str("path", cfg.Database.Path).Msg("Database initialized")

	// Initialize GitHub client
	ghClient := github.NewClient(cfg.GitHub.Token)

	// Initialize Telegram bot
	bot, err := telegram.NewBot(cfg.Telegram.Token, cfg.Telegram.Debug, store, ghClient)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize Telegram bot")
	}

	// Set GitHub client in handlers for repo validation
	bot.GetAPI() // ensure bot is ready

	// Create event channel for events (from webhook or poller)
	eventsCh := make(chan *github.WebhookEvent, 100)

	// Create notifier
	notify := notifier.NewNotifier(bot.GetAPI(), store)

	// Start event processing goroutine
	go func() {
		for event := range eventsCh {
			if err := notify.HandleWebhookEvent(event); err != nil {
				logger.Error().Err(err).Msg("Failed to handle event")
			}
		}
	}()

	// Start poller if enabled (polling or both mode)
	var poller *github.Poller
	if cfg.GitHub.Mode == "polling" || cfg.GitHub.Mode == "both" {
		poller = github.NewPoller(ghClient, store, eventsCh, cfg.GitHub.PollInterval)
		poller.Start()
		logger.Info().Int("interval_sec", cfg.GitHub.PollInterval).Msg("Poller started - can monitor ANY public repository")
	}

	// Set up HTTP router for webhooks
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// Health check endpoint
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// GitHub webhook endpoint (if webhook or both mode)
	if cfg.GitHub.Mode == "webhook" || cfg.GitHub.Mode == "both" {
		webhookHandler := github.NewWebhookHandler(cfg.GitHub.WebhookSecret, eventsCh)
		r.Post("/webhook", webhookHandler.ServeHTTP)
		r.Post("/webhook/github", webhookHandler.ServeHTTP)
		logger.Info().Msg("Webhook endpoint enabled at /webhook")
	}

	// Start HTTP server
	server := &http.Server{
		Addr:    cfg.ServerAddress(),
		Handler: r,
	}

	go func() {
		logger.Info().Str("address", cfg.ServerAddress()).Msg("Starting HTTP server")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("HTTP server error")
		}
	}()

	// Start Telegram bot
	bot.Start()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info().Msg("Shutting down...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Stop poller if running
	if poller != nil {
		poller.Stop()
	}

	// Stop HTTP server
	if err := server.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("HTTP server shutdown error")
	}

	// Stop Telegram bot
	bot.Stop()

	// Close event channel
	close(eventsCh)

	logger.Info().Msg("Shutdown complete")
}
