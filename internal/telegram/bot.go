// Package telegram provides Telegram bot functionality.
package telegram

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/user/githubbot/internal/github"
	"github.com/user/githubbot/internal/storage"
	"github.com/user/githubbot/pkg/logger"
	"golang.org/x/net/proxy"
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
func NewBot(token string, debug bool, proxyURL string, store *storage.SubscriptionStore, ghClient *github.Client) (*Bot, error) {
	var api *tgbotapi.BotAPI
	var err error

	// Configure proxy if provided
	if proxyURL != "" {
		httpClient, proxyErr := createProxyClient(proxyURL)
		if proxyErr != nil {
			return nil, fmt.Errorf("failed to create proxy client: %w", proxyErr)
		}
		api, err = tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, httpClient)
		logger.Info().Str("proxy", proxyURL).Msg("Using proxy for Telegram API")
	} else {
		api, err = tgbotapi.NewBotAPI(token)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	api.Debug = debug

	logger.Info().Str("username", api.Self.UserName).Msg("Telegram bot authorized")

	// Set bot commands menu
	commands := []tgbotapi.BotCommand{
		{Command: "start", Description: "查看帮助信息"},
		{Command: "help", Description: "显示帮助"},
		{Command: "subscribe", Description: "订阅仓库 (格式: owner/repo)"},
		{Command: "unsubscribe", Description: "取消订阅"},
		{Command: "list", Description: "查看订阅列表"},
		{Command: "status", Description: "查看机器人状态"},
	}
	setCommandsConfig := tgbotapi.NewSetMyCommands(commands...)
	if _, err := api.Request(setCommandsConfig); err != nil {
		logger.Warn().Err(err).Msg("Failed to set bot commands menu")
	}

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

// createProxyClient creates an HTTP client with proxy support.
// Supports both HTTP(S) and SOCKS5 proxies.
func createProxyClient(proxyURL string) (*http.Client, error) {
	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}

	var transport *http.Transport

	switch parsedURL.Scheme {
	case "socks5", "socks5h":
		// SOCKS5 proxy
		var auth *proxy.Auth
		if parsedURL.User != nil {
			auth = &proxy.Auth{
				User: parsedURL.User.Username(),
			}
			auth.Password, _ = parsedURL.User.Password()
		}

		dialer, err := proxy.SOCKS5("tcp", parsedURL.Host, auth, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
		}

		// Type assert to get ContextDialer if available
		contextDialer, ok := dialer.(proxy.ContextDialer)
		if ok {
			transport = &http.Transport{
				DialContext: contextDialer.DialContext,
			}
		} else {
			transport = &http.Transport{
				Dial: dialer.Dial,
			}
		}

	case "http", "https":
		// HTTP(S) proxy
		transport = &http.Transport{
			Proxy: http.ProxyURL(parsedURL),
		}

	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s (use http, https, or socks5)", parsedURL.Scheme)
	}

	return &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}, nil
}
