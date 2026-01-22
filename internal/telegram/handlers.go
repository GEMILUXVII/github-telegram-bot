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
	case "test":
		h.handleTest(msg)
	default:
		h.sendReply(msg.Chat.ID, "Unknown command. Use /help to see available commands.")
	}
}

// HandleCallback handles inline keyboard callbacks.
func (h *Handlers) HandleCallback(callback *tgbotapi.CallbackQuery) {
	// Acknowledge the callback
	callbackCfg := tgbotapi.NewCallback(callback.ID, "")
	if _, err := h.api.Send(callbackCfg); err != nil {
		logger.Error().Err(err).Msg("Failed to send callback acknowledgement")
	}

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

	text := fmt.Sprintf("*Subscriptions (%d)*\n\nClick the button below to unsubscribe:", len(subs))

	// Create inline keyboard with unsubscribe buttons
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, sub := range subs {
		repoFullName := fmt.Sprintf("%s/%s", sub.RepoOwner, sub.RepoName)
		callbackData := fmt.Sprintf("unsub:%s:%s", sub.RepoOwner, sub.RepoName)

		row := tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL(
				repoFullName,
				fmt.Sprintf("https://github.com/%s", repoFullName),
			),
			tgbotapi.NewInlineKeyboardButtonData(
				"Unsubscribe",
				callbackData,
			),
		)
		rows = append(rows, row)
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

	msgConfig := tgbotapi.NewMessage(msg.Chat.ID, text)
	msgConfig.ParseMode = tgbotapi.ModeMarkdown
	msgConfig.ReplyMarkup = keyboard

	if _, err := h.api.Send(msgConfig); err != nil {
		logger.Error().Err(err).Msg("Failed to send list message")
	}
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

// handleTest sends sample notifications for all event types.
func (h *Handlers) handleTest(msg *tgbotapi.Message) {
	h.sendReply(msg.Chat.ID, "🧪 *Testing notification formats...*\n\nSending sample notifications for all event types:")

	// Sample Push Event
	pushEvent := &github.PushEvent{
		Ref:       "refs/heads/main",
		Before:    "abc1234567890",
		After:     "def0987654321",
		Pusher:    github.UserInfo{Login: "developer"},
		Timestamp: time.Now(),
		Commits: []github.CommitInfo{
			{
				SHA:       "def0987654321",
				Message:   "feat: add new authentication module\n\nThis commit adds OAuth2 support",
				Author:    github.UserInfo{Login: "developer"},
				URL:       "https://github.com/example/repo/commit/def0987",
				Additions: 150,
				Deletions: 23,
			},
			{
				SHA:       "abc1234567890",
				Message:   "fix: resolve login redirect issue",
				Author:    github.UserInfo{Login: "developer"},
				URL:       "https://github.com/example/repo/commit/abc1234",
				Additions: 12,
				Deletions: 5,
			},
			{
				SHA:       "ghi5678901234",
				Message:   "docs: update README with new API endpoints",
				Author:    github.UserInfo{Login: "contributor"},
				URL:       "https://github.com/example/repo/commit/ghi5678",
				Additions: 45,
				Deletions: 10,
			},
		},
		Compare: "https://github.com/example/repo/compare/abc1234...def0987",
	}
	pushMsg := pushEvent.FormatMessage(github.RepoInfo{Owner: "example", Name: "awesome-project"})
	h.sendHTML(msg.Chat.ID, pushMsg)

	// Sample Release Event
	releaseEvent := &github.ReleaseEvent{
		Action:      "published",
		TagName:     "v2.1.0",
		Name:        "Version 2.1.0 - Major Update",
		Body:        "## What's New\n\n- **New Feature**: Added dark mode support\n- **Improvement**: Performance optimizations (50% faster)\n- **Bug Fix**: Fixed memory leak in background tasks\n\n## Breaking Changes\n\nNone in this release.\n\n## Contributors\n\nThanks to all contributors!",
		Prerelease:  false,
		URL:         "https://github.com/example/repo/releases/tag/v2.1.0",
		Author:      github.UserInfo{Login: "maintainer"},
		PublishedAt: time.Now(),
		Assets: []github.ReleaseAsset{
			{Name: "app-linux-amd64.tar.gz", Size: 15728640, DownloadCount: 1234},
			{Name: "app-darwin-arm64.tar.gz", Size: 14680064, DownloadCount: 567},
			{Name: "app-windows-amd64.zip", Size: 16777216, DownloadCount: 890},
		},
	}
	releaseMsg := releaseEvent.FormatMessage(github.RepoInfo{Owner: "example", Name: "awesome-project"})
	h.sendHTML(msg.Chat.ID, releaseMsg)

	// Sample Issue Opened Event
	issueOpenedEvent := &github.IssueEvent{
		Action:    "opened",
		Number:    1024,
		Title:     "[Bug] Application crashes when using Chinese characters in search",
		Body:      "## Description\n\nWhen I try to search with Chinese characters like \"测试\", the application crashes immediately.\n\n## Steps to Reproduce\n\n1. Open the search dialog\n2. Type any Chinese characters\n3. App crashes\n\n## Expected Behavior\n\nSearch should work with all Unicode characters.\n\n## Environment\n\n- OS: Windows 11\n- Version: 2.0.5",
		State:     "open",
		URL:       "https://github.com/example/repo/issues/1024",
		User:      github.UserInfo{Login: "bug-reporter"},
		Labels:    []string{"bug", "high-priority", "i18n"},
		CreatedAt: time.Now(),
	}
	issueOpenedMsg := issueOpenedEvent.FormatMessage(github.RepoInfo{Owner: "example", Name: "awesome-project"})
	h.sendHTML(msg.Chat.ID, issueOpenedMsg)

	// Sample Issue Closed Event
	issueClosedEvent := &github.IssueEvent{
		Action:    "closed",
		Number:    1020,
		Title:     "Add support for custom themes",
		Body:      "Feature request: Allow users to create and share custom themes.",
		State:     "closed",
		URL:       "https://github.com/example/repo/issues/1020",
		User:      github.UserInfo{Login: "feature-requester"},
		Labels:    []string{"enhancement", "completed"},
		CreatedAt: time.Now().Add(-24 * time.Hour),
	}
	issueClosedMsg := issueClosedEvent.FormatMessage(github.RepoInfo{Owner: "example", Name: "awesome-project"})
	h.sendHTML(msg.Chat.ID, issueClosedMsg)

	// Sample PR Opened Event
	prOpenedEvent := &github.PullRequestEvent{
		Action:    "opened",
		Number:    256,
		Title:     "feat: implement OAuth2 authentication flow",
		Body:      "## Summary\n\nThis PR implements the OAuth2 authentication flow as discussed in #200.\n\n## Changes\n\n- Added OAuth2 provider configuration\n- Implemented token refresh mechanism\n- Added unit tests for auth module\n\n## Testing\n\n- [x] Unit tests pass\n- [x] Integration tests pass\n- [x] Manual testing completed",
		State:     "open",
		URL:       "https://github.com/example/repo/pull/256",
		User:      github.UserInfo{Login: "contributor"},
		Merged:    false,
		Base:      github.BranchInfo{Ref: "main"},
		Head:      github.BranchInfo{Ref: "feature/oauth2-auth"},
		Additions: 523,
		Deletions: 47,
		Commits:   8,
		CreatedAt: time.Now(),
	}
	prOpenedMsg := prOpenedEvent.FormatMessage(github.RepoInfo{Owner: "example", Name: "awesome-project"})
	h.sendHTML(msg.Chat.ID, prOpenedMsg)

	// Sample PR Merged Event
	prMergedEvent := &github.PullRequestEvent{
		Action:    "closed",
		Number:    250,
		Title:     "fix: resolve memory leak in websocket handler",
		Body:      "Fixes the memory leak reported in #245. The issue was caused by unclosed goroutines.",
		State:     "closed",
		URL:       "https://github.com/example/repo/pull/250",
		User:      github.UserInfo{Login: "senior-dev"},
		Merged:    true,
		MergedBy:  &github.UserInfo{Login: "maintainer"},
		Base:      github.BranchInfo{Ref: "main"},
		Head:      github.BranchInfo{Ref: "fix/memory-leak"},
		Additions: 25,
		Deletions: 180,
		Commits:   3,
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}
	prMergedMsg := prMergedEvent.FormatMessage(github.RepoInfo{Owner: "example", Name: "awesome-project"})
	h.sendHTML(msg.Chat.ID, prMergedMsg)

	h.sendReply(msg.Chat.ID, "✅ *Test complete!*\n\nAbove are sample notifications for:\n• 📦 Push (commits)\n• 🚀 Release\n• 🐛 Issue Opened\n• ✅ Issue Closed\n• 🔀 PR Opened\n• 🎉 PR Merged")
}

// sendHTML sends an HTML-formatted message.
func (h *Handlers) sendHTML(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true
	if _, err := h.api.Send(msg); err != nil {
		logger.Error().Err(err).Msg("Failed to send HTML message")
	}
}
