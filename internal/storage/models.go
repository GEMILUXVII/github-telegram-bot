// Package storage provides database operations and data models.
package storage

import "time"

// Subscription represents a repository subscription.
type Subscription struct {
	ID        int64     `db:"id"`
	ChatID    int64     `db:"chat_id"`
	RepoOwner string    `db:"repo_owner"`
	RepoName  string    `db:"repo_name"`
	Events    string    `db:"events"` // JSON array of event types
	CreatedAt time.Time `db:"created_at"`
}

// EventRecord stores processed events for deduplication.
type EventRecord struct {
	ID        int64     `db:"id"`
	RepoOwner string    `db:"repo_owner"`
	RepoName  string    `db:"repo_name"`
	EventType string    `db:"event_type"`
	EventID   string    `db:"event_id"`
	CreatedAt time.Time `db:"created_at"`
}

// Chat represents a Telegram chat (user or group).
type Chat struct {
	ID        int64     `db:"id"`
	ChatID    int64     `db:"chat_id"`
	ChatType  string    `db:"chat_type"` // private, group, supergroup, channel
	Title     string    `db:"title"`
	CreatedAt time.Time `db:"created_at"`
}

// EventType represents the type of GitHub event.
type EventType string

const (
	EventTypePush        EventType = "push"
	EventTypeRelease     EventType = "release"
	EventTypeIssue       EventType = "issues"
	EventTypePullRequest EventType = "pull_request"
	EventTypeStar        EventType = "star"
	EventTypeFork        EventType = "fork"
)

// AllEventTypes returns all supported event types.
func AllEventTypes() []EventType {
	return []EventType{
		EventTypePush,
		EventTypeRelease,
		EventTypeIssue,
		EventTypePullRequest,
	}
}

// DefaultEvents returns the default event types for new subscriptions.
func DefaultEvents() []EventType {
	return []EventType{
		EventTypePush,
		EventTypeRelease,
		EventTypeIssue,
		EventTypePullRequest,
	}
}
