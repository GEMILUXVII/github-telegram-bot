// Package notifier handles sending notifications to subscribers.
package notifier

import (
	"encoding/json"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/user/githubbot/internal/github"
	"github.com/user/githubbot/internal/storage"
	"github.com/user/githubbot/internal/telegram"
	"github.com/user/githubbot/pkg/logger"
)

// Notifier sends notifications to Telegram chats.
type Notifier struct {
	bot        *tgbotapi.BotAPI
	store      *storage.SubscriptionStore
	msgBuilder *telegram.MessageBuilder
}

// NewNotifier creates a new notifier instance.
func NewNotifier(bot *tgbotapi.BotAPI, store *storage.SubscriptionStore) *Notifier {
	return &Notifier{
		bot:        bot,
		store:      store,
		msgBuilder: telegram.NewMessageBuilder(),
	}
}

// HandleWebhookEvent processes a webhook event and sends notifications.
func (n *Notifier) HandleWebhookEvent(event *github.WebhookEvent) error {
	// Get all subscribers for this repo
	subs, err := n.store.GetSubscriptionsByRepo(event.RepoOwner, event.RepoName)
	if err != nil {
		return fmt.Errorf("failed to get subscribers: %w", err)
	}

	if len(subs) == 0 {
		logger.Debug().
			Str("repo", fmt.Sprintf("%s/%s", event.RepoOwner, event.RepoName)).
			Msg("No subscribers for this repository")
		return nil
	}

	// Generate event ID for deduplication
	eventID := n.generateEventID(event)

	// Check if event was already processed
	processed, err := n.store.IsEventProcessed(event.RepoOwner, event.RepoName, event.Type, eventID)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to check event processing status")
	}
	if processed {
		logger.Debug().Str("event_id", eventID).Msg("Event already processed, skipping")
		return nil
	}

	// Build the notification message
	message := n.buildMessage(event)
	if message == "" {
		return nil
	}

	// Send to all subscribers who want this event type
	eventType := storage.EventType(event.Type)
	for _, sub := range subs {
		if n.isEventEnabled(sub, eventType) {
			if err := n.sendNotification(sub.ChatID, message); err != nil {
				logger.Error().
					Err(err).
					Int64("chat_id", sub.ChatID).
					Msg("Failed to send notification")
				// Continue sending to other subscribers
			}
		}
	}

	// Record the event as processed
	if err := n.store.RecordEvent(event.RepoOwner, event.RepoName, event.Type, eventID); err != nil {
		logger.Warn().Err(err).Msg("Failed to record event")
	}

	return nil
}

// generateEventID creates a unique ID for an event.
func (n *Notifier) generateEventID(event *github.WebhookEvent) string {
	switch e := event.Payload.(type) {
	case *github.PushEvent:
		return e.After // Use the commit SHA
	case *github.ReleaseEvent:
		return e.TagName
	case *github.IssueEvent:
		return fmt.Sprintf("%d-%s", e.Number, e.Action)
	case *github.PullRequestEvent:
		return fmt.Sprintf("%d-%s", e.Number, e.Action)
	default:
		return fmt.Sprintf("%s-%v", event.Type, event.Payload)
	}
}

// buildMessage creates the notification message for an event.
func (n *Notifier) buildMessage(event *github.WebhookEvent) string {
	switch e := event.Payload.(type) {
	case *github.PushEvent:
		return n.msgBuilder.BuildPushMessage(event.RepoOwner, event.RepoName, e)
	case *github.ReleaseEvent:
		return n.msgBuilder.BuildReleaseMessage(event.RepoOwner, event.RepoName, e)
	case *github.IssueEvent:
		return n.msgBuilder.BuildIssueMessage(event.RepoOwner, event.RepoName, e)
	case *github.PullRequestEvent:
		return n.msgBuilder.BuildPRMessage(event.RepoOwner, event.RepoName, e)
	default:
		logger.Warn().Str("type", event.Type).Msg("Unknown event type")
		return ""
	}
}

// isEventEnabled checks if a subscriber wants this type of event.
func (n *Notifier) isEventEnabled(sub storage.Subscription, eventType storage.EventType) bool {
	var events []storage.EventType
	if err := json.Unmarshal([]byte(sub.Events), &events); err != nil {
		// If we can't parse, assume all events are wanted
		return true
	}

	for _, e := range events {
		if e == eventType {
			return true
		}
	}
	return false
}

// sendNotification sends a message to a chat.
func (n *Notifier) sendNotification(chatID int64, message string) error {
	msg := tgbotapi.NewMessage(chatID, message)
	msg.ParseMode = tgbotapi.ModeMarkdown
	msg.DisableWebPagePreview = true

	_, err := n.bot.Send(msg)
	return err
}
