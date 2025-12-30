// Package telegram provides Telegram bot functionality.
package telegram

import (
	"context"
	"fmt"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/user/githubbot/internal/github"
	"github.com/user/githubbot/internal/storage"
	"github.com/user/githubbot/pkg/logger"
)

// Bot represents the Telegram bot.
type Bot struct {
	api      *tgbotapi.BotAPI
	handlers *Handlers
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewBot creates a new Telegram bot instance.
func NewBot(token string, debug bool, store *storage.SubscriptionStore, ghClient *github.Client) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	api.Debug = debug

	logger.Info().Str("username", api.Self.UserName).Msg("Telegram bot authorized")

	ctx, cancel := context.WithCancel(context.Background())

	handlers := NewHandlers(api, store)
	if ghClient != nil {
		handlers.SetGitHubClient(ghClient)
	}
	handlers.SetStartTime(time.Now())

	return &Bot{
		api:      api,
		handlers: handlers,
		ctx:      ctx,
		cancel:   cancel,
	}, nil
}

// Start begins listening for updates.
func (b *Bot) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		for {
			select {
			case <-b.ctx.Done():
				return
			case update := <-updates:
				if update.Message != nil {
					b.handleMessage(update.Message)
				} else if update.CallbackQuery != nil {
					b.handleCallback(update.CallbackQuery)
				}
			}
		}
	}()

	logger.Info().Msg("Telegram bot started, listening for updates")
}

// Stop gracefully stops the bot.
func (b *Bot) Stop() {
	logger.Info().Msg("Stopping Telegram bot")
	b.cancel()
	b.api.StopReceivingUpdates()
	b.wg.Wait()
}

// handleMessage processes incoming messages.
func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	if msg.IsCommand() {
		b.handlers.HandleCommand(msg)
	}
}

// handleCallback processes callback queries from inline keyboards.
func (b *Bot) handleCallback(callback *tgbotapi.CallbackQuery) {
	b.handlers.HandleCallback(callback)
}

// SendMessage sends a message to a chat.
func (b *Bot) SendMessage(chatID int64, text string, parseMode string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	if parseMode != "" {
		msg.ParseMode = parseMode
	}
	msg.DisableWebPagePreview = true

	_, err := b.api.Send(msg)
	if err != nil {
		logger.Error().Err(err).Int64("chat_id", chatID).Msg("Failed to send message")
		return err
	}
	return nil
}

// SendMarkdownMessage sends a markdown-formatted message.
func (b *Bot) SendMarkdownMessage(chatID int64, text string) error {
	return b.SendMessage(chatID, text, tgbotapi.ModeMarkdown)
}

// GetAPI returns the underlying bot API for direct access.
func (b *Bot) GetAPI() *tgbotapi.BotAPI {
	return b.api
}
