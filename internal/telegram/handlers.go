package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/user/githubbot/internal/github"
	"github.com/user/githubbot/internal/storage"
	"github.com/user/githubbot/pkg/logger"
)

// Handlers manages command handling for the bot.
type Handlers struct {
	api       *tgbotapi.BotAPI
	store     *storage.SubscriptionStore
	ghClient  *github.Client
	startTime time.Time
}

// NewHandlers creates a new handlers instance.
func NewHandlers(api *tgbotapi.BotAPI, store *storage.SubscriptionStore) *Handlers {
	return &Handlers{
		api:   api,
		store: store,
	}
}

// SetGitHubClient sets the GitHub client for repository validation.
func (h *Handlers) SetGitHubClient(client *github.Client) {
	h.ghClient = client
}

// SetStartTime sets the bot start time for uptime calculation.
func (h *Handlers) SetStartTime(t time.Time) {
	h.startTime = t
}

// HandleCommand routes commands to appropriate handlers.
func (h *Handlers) HandleCommand(msg *tgbotapi.Message) {
	command := msg.Command()
	args := msg.CommandArguments()

	logger.Debug().
		Str("command", command).
		Str("args", args).
		Int64("chat_id", msg.Chat.ID).
		Msg("Received command")

	// Track chat for future notifications
	h.trackChat(msg.Chat)

	switch command {
	case "start":
		h.handleStart(msg)
	case "help":
		h.handleHelp(msg)
	case "subscribe", "sub":
		h.handleSubscribe(msg, args)
	case "unsubscribe", "unsub":
		h.handleUnsubscribe(msg, args)
	case "list":
		h.handleList(msg)
	case "status":
		h.handleStatus(msg)
	default:
		h.sendReply(msg.Chat.ID, "Unknown command. Use /help to see available commands.")
	}
}

// HandleCallback handles inline keyboard callbacks.
func (h *Handlers) HandleCallback(callback *tgbotapi.CallbackQuery) {
	// Acknowledge the callback
	callbackCfg := tgbotapi.NewCallback(callback.ID, "")
	h.api.Send(callbackCfg)

	// Parse callback data
	parts := strings.Split(callback.Data, ":")
	if len(parts) < 1 {
		return
	}

	switch parts[0] {
	case "unsub":
		if len(parts) == 3 {
			h.handleUnsubscribeCallback(callback, parts[1], parts[2])
		}
	}
}

// trackChat stores chat information for notifications.
func (h *Handlers) trackChat(chat *tgbotapi.Chat) {
	chatType := string(chat.Type)
	title := chat.Title
	if chat.Type == "private" {
		title = chat.FirstName
		if chat.LastName != "" {
			title += " " + chat.LastName
		}
	}

	if err := h.store.CreateOrUpdateChat(chat.ID, chatType, title); err != nil {
		logger.Error().Err(err).Int64("chat_id", chat.ID).Msg("Failed to track chat")
	}
}

// handleStart sends a welcome message.
func (h *Handlers) handleStart(msg *tgbotapi.Message) {
	text := `*Welcome to GitHub Monitor Bot*

I can help you monitor *any public GitHub repository*, including:
- New commits (Push)
- Version releases (Release)
- Issue updates
- Pull Request updates

*Quick Start:*
Use ` + "`/subscribe owner/repo`" + ` to subscribe!

*Examples:*
` + "`/subscribe torvalds/linux`" + `
` + "`/subscribe microsoft/vscode`" + `

Use /help to see all commands.`

	h.sendMarkdown(msg.Chat.ID, text)
}

// handleHelp sends help information.
func (h *Handlers) handleHelp(msg *tgbotapi.Message) {
	text := `*Command Help*

*Subscription Management:*
- ` + "`/subscribe <owner/repo>`" + ` - Subscribe to a repository
- ` + "`/unsubscribe <owner/repo>`" + ` - Unsubscribe from a repository
- ` + "`/list`" + ` - View current subscriptions

*Shortcuts:*
- ` + "`/sub`" + ` - Short for subscribe
- ` + "`/unsub`" + ` - Short for unsubscribe

*Examples:*
` + "```" + `
/subscribe torvalds/linux
/subscribe microsoft/vscode
/sub golang/go
/list
/unsub torvalds/linux
` + "```" + `

After subscribing, you will receive notifications when the repository has new commits, releases, issues, or PRs.`

	h.sendMarkdown(msg.Chat.ID, text)
}

// handleSubscribe handles the subscribe command.
func (h *Handlers) handleSubscribe(msg *tgbotapi.Message, args string) {
	if args == "" {
		h.sendReply(msg.Chat.ID, "[Error] Please specify a repository: `/subscribe owner/repo`")
		return
	}

	owner, repo, err := parseRepoArg(args)
	if err != nil {
		h.sendReply(msg.Chat.ID, "[Error] Invalid format, use: `owner/repo`")
		return
	}

	// Validate repository exists (if GitHub client is set)
	if h.ghClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		exists, err := h.ghClient.ValidateRepository(ctx, owner, repo)
		if err != nil {
			h.sendReply(msg.Chat.ID, "[Warning] Error validating repository, please try again later")
			logger.Error().Err(err).Str("repo", args).Msg("Failed to validate repository")
			return
		}
		if !exists {
			h.sendReply(msg.Chat.ID, fmt.Sprintf("[Error] Repository `%s/%s` does not exist or is not accessible", owner, repo))
			return
		}
	}

	// Subscribe with default events
	events := storage.DefaultEvents()
	if err := h.store.Subscribe(msg.Chat.ID, owner, repo, events); err != nil {
		h.sendReply(msg.Chat.ID, "[Error] Subscription failed, please try again later")
		logger.Error().Err(err).Str("repo", args).Msg("Failed to subscribe")
		return
	}

	text := fmt.Sprintf(`[OK] *Subscribed to %s/%s*

Monitored events:
- Push (Commits)
- Release (Versions)
- Issues
- Pull Requests

You will receive notifications when the repository has updates.`, owner, repo)

	h.sendMarkdown(msg.Chat.ID, text)
}

// handleUnsubscribe handles the unsubscribe command.
func (h *Handlers) handleUnsubscribe(msg *tgbotapi.Message, args string) {
	if args == "" {
		h.sendReply(msg.Chat.ID, "[Error] Please specify a repository: `/unsubscribe owner/repo`")
		return
	}

	owner, repo, err := parseRepoArg(args)
	if err != nil {
		h.sendReply(msg.Chat.ID, "[Error] Invalid format, use: `owner/repo`")
		return
	}

	if err := h.store.Unsubscribe(msg.Chat.ID, owner, repo); err != nil {
		if err.Error() == "subscription not found" {
			h.sendReply(msg.Chat.ID, fmt.Sprintf("[Error] No subscription found for `%s/%s`", owner, repo))
		} else {
			h.sendReply(msg.Chat.ID, "[Error] Unsubscribe failed, please try again later")
			logger.Error().Err(err).Str("repo", args).Msg("Failed to unsubscribe")
		}
		return
	}

	h.sendReply(msg.Chat.ID, fmt.Sprintf("[OK] Unsubscribed from `%s/%s`", owner, repo))
}

// handleUnsubscribeCallback handles inline unsubscribe button.
func (h *Handlers) handleUnsubscribeCallback(callback *tgbotapi.CallbackQuery, owner, repo string) {
	chatID := callback.Message.Chat.ID

	if err := h.store.Unsubscribe(chatID, owner, repo); err != nil {
		h.sendReply(chatID, "[Error] Unsubscribe failed")
		return
	}

	h.sendReply(chatID, fmt.Sprintf("[OK] Unsubscribed from `%s/%s`", owner, repo))
}

// handleList shows all current subscriptions.
func (h *Handlers) handleList(msg *tgbotapi.Message) {
	subs, err := h.store.GetSubscriptionsByChat(msg.Chat.ID)
	if err != nil {
		h.sendReply(msg.Chat.ID, "[Error] Failed to get subscription list")
		logger.Error().Err(err).Msg("Failed to get subscriptions")
		return
	}

	if len(subs) == 0 {
		h.sendReply(msg.Chat.ID, "[Info] No subscriptions yet\n\nUse `/subscribe owner/repo` to subscribe")
		return
	}

	text := fmt.Sprintf("*Subscriptions (%d)*\n\n", len(subs))
	for i, sub := range subs {
		text += fmt.Sprintf("%d. [`%s/%s`](https://github.com/%s/%s)\n",
			i+1, sub.RepoOwner, sub.RepoName, sub.RepoOwner, sub.RepoName)
	}

	text += "\nUse `/unsubscribe owner/repo` to unsubscribe"

	h.sendMarkdown(msg.Chat.ID, text)
}

// handleStatus shows bot status information.
func (h *Handlers) handleStatus(msg *tgbotapi.Message) {
	// Calculate uptime
	uptime := time.Since(h.startTime)
	uptimeStr := formatDuration(uptime)

	// Get subscription count
	repos, err := h.store.GetAllSubscribedRepos()
	repoCount := 0
	if err == nil {
		repoCount = len(repos)
	}

	// Get user's subscription count
	userSubs, err := h.store.GetSubscriptionsByChat(msg.Chat.ID)
	userSubCount := 0
	if err == nil {
		userSubCount = len(userSubs)
	}

	// Get GitHub API rate limit
	rateLimitInfo := "Unknown"
	if h.ghClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		limits, err := h.ghClient.GetRateLimit(ctx)
		if err == nil && limits != nil && limits.Core != nil {
			remaining := limits.Core.Remaining
			limit := limits.Core.Limit
			resetTime := limits.Core.Reset.Time
			resetIn := time.Until(resetTime)
			rateLimitInfo = fmt.Sprintf("%d/%d (resets in %s)", remaining, limit, formatDuration(resetIn))
		}
	}

	text := fmt.Sprintf(`*Bot Status*

*Uptime:* %s
*Mode:* Polling

*Global Stats:*
- Monitored repos: %d

*Your Subscriptions:*
- Count: %d

*GitHub API:*
- Quota: %s
`, uptimeStr, repoCount, userSubCount, rateLimitInfo)

	h.sendMarkdown(msg.Chat.ID, text)
}

// formatDuration formats a duration to a human-readable string.
func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// sendReply sends a simple text reply.
func (h *Handlers) sendReply(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown
	if _, err := h.api.Send(msg); err != nil {
		logger.Error().Err(err).Msg("Failed to send reply")
	}
}

// sendMarkdown sends a markdown-formatted message.
func (h *Handlers) sendMarkdown(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.DisableWebPagePreview = true
	if _, err := h.api.Send(msg); err != nil {
		logger.Error().Err(err).Msg("Failed to send markdown message")
	}
}

// parseRepoArg parses "owner/repo" format.
func parseRepoArg(arg string) (owner, repo string, err error) {
	arg = strings.TrimSpace(arg)
	parts := strings.Split(arg, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid format")
	}

	owner = strings.TrimSpace(parts[0])
	repo = strings.TrimSpace(parts[1])

	if owner == "" || repo == "" {
		return "", "", fmt.Errorf("empty owner or repo")
	}

	return owner, repo, nil
}
