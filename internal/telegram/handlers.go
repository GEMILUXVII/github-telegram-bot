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
		h.sendReply(msg.Chat.ID, "未知命令。使用 /help 查看可用命令。")
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
	text := `🤖 *欢迎使用 GitHub 监控机器人！*

我可以帮助你监控 *任意 GitHub 公有仓库* 的变动，包括：
• 📨 新的提交 (Push)
• 🎉 版本发布 (Release)
• 📝 Issue 变动
• 🔀 Pull Request 变动

*快速开始：*
使用 ` + "`/subscribe owner/repo`" + ` 订阅仓库即可！

*示例：*
` + "`/subscribe torvalds/linux`" + `
` + "`/subscribe microsoft/vscode`" + `

使用 /help 查看所有命令。`

	h.sendMarkdown(msg.Chat.ID, text)
}

// handleHelp sends help information.
func (h *Handlers) handleHelp(msg *tgbotapi.Message) {
	text := `📚 *命令帮助*

*订阅管理：*
• ` + "`/subscribe <owner/repo>`" + ` - 订阅仓库
• ` + "`/unsubscribe <owner/repo>`" + ` - 取消订阅
• ` + "`/list`" + ` - 查看当前订阅

*快捷命令：*
• ` + "`/sub`" + ` - 订阅仓库的简写
• ` + "`/unsub`" + ` - 取消订阅的简写

*示例：*
` + "```" + `
/subscribe torvalds/linux
/subscribe microsoft/vscode
/sub golang/go
/list
/unsub torvalds/linux
` + "```" + `

💡 订阅后，当仓库有新的 commit、release、issue 或 PR 时，你将自动收到通知。`

	h.sendMarkdown(msg.Chat.ID, text)
}

// handleSubscribe handles the subscribe command.
func (h *Handlers) handleSubscribe(msg *tgbotapi.Message, args string) {
	if args == "" {
		h.sendReply(msg.Chat.ID, "❌ 请指定仓库，格式: `/subscribe owner/repo`")
		return
	}

	owner, repo, err := parseRepoArg(args)
	if err != nil {
		h.sendReply(msg.Chat.ID, "❌ 仓库格式错误，请使用: `owner/repo`")
		return
	}

	// Validate repository exists (if GitHub client is set)
	if h.ghClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		exists, err := h.ghClient.ValidateRepository(ctx, owner, repo)
		if err != nil {
			h.sendReply(msg.Chat.ID, "⚠️ 验证仓库时出错，请稍后重试")
			logger.Error().Err(err).Str("repo", args).Msg("Failed to validate repository")
			return
		}
		if !exists {
			h.sendReply(msg.Chat.ID, fmt.Sprintf("❌ 仓库 `%s/%s` 不存在或不可访问", owner, repo))
			return
		}
	}

	// Subscribe with default events
	events := storage.DefaultEvents()
	if err := h.store.Subscribe(msg.Chat.ID, owner, repo, events); err != nil {
		h.sendReply(msg.Chat.ID, "❌ 订阅失败，请稍后重试")
		logger.Error().Err(err).Str("repo", args).Msg("Failed to subscribe")
		return
	}

	text := fmt.Sprintf(`✅ *成功订阅 %s/%s*

监控事件：
• 📨 Push (提交)
• 🎉 Release (发布)
• 📝 Issues
• 🔀 Pull Requests

当仓库有新动态时，你将自动收到通知！`, owner, repo)

	h.sendMarkdown(msg.Chat.ID, text)
}

// handleUnsubscribe handles the unsubscribe command.
func (h *Handlers) handleUnsubscribe(msg *tgbotapi.Message, args string) {
	if args == "" {
		h.sendReply(msg.Chat.ID, "❌ 请指定仓库，格式: `/unsubscribe owner/repo`")
		return
	}

	owner, repo, err := parseRepoArg(args)
	if err != nil {
		h.sendReply(msg.Chat.ID, "❌ 仓库格式错误，请使用: `owner/repo`")
		return
	}

	if err := h.store.Unsubscribe(msg.Chat.ID, owner, repo); err != nil {
		if err.Error() == "subscription not found" {
			h.sendReply(msg.Chat.ID, fmt.Sprintf("❌ 未找到 `%s/%s` 的订阅", owner, repo))
		} else {
			h.sendReply(msg.Chat.ID, "❌ 取消订阅失败，请稍后重试")
			logger.Error().Err(err).Str("repo", args).Msg("Failed to unsubscribe")
		}
		return
	}

	h.sendReply(msg.Chat.ID, fmt.Sprintf("✅ 已取消订阅 `%s/%s`", owner, repo))
}

// handleUnsubscribeCallback handles inline unsubscribe button.
func (h *Handlers) handleUnsubscribeCallback(callback *tgbotapi.CallbackQuery, owner, repo string) {
	chatID := callback.Message.Chat.ID

	if err := h.store.Unsubscribe(chatID, owner, repo); err != nil {
		h.sendReply(chatID, "❌ 取消订阅失败")
		return
	}

	h.sendReply(chatID, fmt.Sprintf("✅ 已取消订阅 `%s/%s`", owner, repo))
}

// handleList shows all current subscriptions.
func (h *Handlers) handleList(msg *tgbotapi.Message) {
	subs, err := h.store.GetSubscriptionsByChat(msg.Chat.ID)
	if err != nil {
		h.sendReply(msg.Chat.ID, "❌ 获取订阅列表失败")
		logger.Error().Err(err).Msg("Failed to get subscriptions")
		return
	}

	if len(subs) == 0 {
		h.sendReply(msg.Chat.ID, "📭 当前没有任何订阅\n\n使用 `/subscribe owner/repo` 来订阅仓库")
		return
	}

	text := fmt.Sprintf("📋 *当前订阅 (%d 个)*\n\n", len(subs))
	for i, sub := range subs {
		text += fmt.Sprintf("%d. [`%s/%s`](https://github.com/%s/%s)\n",
			i+1, sub.RepoOwner, sub.RepoName, sub.RepoOwner, sub.RepoName)
	}

	text += "\n使用 `/unsubscribe owner/repo` 取消订阅"

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
	rateLimitInfo := "未知"
	if h.ghClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		limits, err := h.ghClient.GetRateLimit(ctx)
		if err == nil && limits != nil && limits.Core != nil {
			remaining := limits.Core.Remaining
			limit := limits.Core.Limit
			resetTime := limits.Core.Reset.Time
			resetIn := time.Until(resetTime)
			rateLimitInfo = fmt.Sprintf("%d/%d (重置于 %s)", remaining, limit, formatDuration(resetIn))
		}
	}

	text := fmt.Sprintf(`📊 *Bot 状态*

⏱️ *运行时间:* %s
📡 *监控模式:* Polling

📦 *全局统计:*
• 监控仓库数: %d

👤 *你的订阅:*
• 订阅数: %d

🔗 *GitHub API:*
• 配额: %s
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
		return fmt.Sprintf("%d天 %d小时 %d分钟", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%d小时 %d分钟", hours, minutes)
	} else if minutes > 0 {
		return fmt.Sprintf("%d分钟 %d秒", minutes, seconds)
	}
	return fmt.Sprintf("%d秒", seconds)
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
