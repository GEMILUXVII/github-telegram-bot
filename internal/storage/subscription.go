package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// SubscriptionStore handles subscription-related database operations.
type SubscriptionStore struct {
	db *Database
}

// NewSubscriptionStore creates a new subscription store.
func NewSubscriptionStore(db *Database) *SubscriptionStore {
	return &SubscriptionStore{db: db}
}

// CreateOrUpdateChat creates or updates a chat record.
func (s *SubscriptionStore) CreateOrUpdateChat(chatID int64, chatType, title string) error {
	query := `
		INSERT INTO chats (chat_id, chat_type, title)
		VALUES (?, ?, ?)
		ON CONFLICT(chat_id) DO UPDATE SET
			chat_type = excluded.chat_type,
			title = excluded.title
	`
	_, err := s.db.Exec(query, chatID, chatType, title)
	return err
}

// Subscribe creates a new subscription for a chat.
func (s *SubscriptionStore) Subscribe(chatID int64, repoOwner, repoName string, events []EventType) error {
	eventsJSON, err := json.Marshal(events)
	if err != nil {
		return fmt.Errorf("failed to marshal events: %w", err)
	}

	query := `
		INSERT INTO subscriptions (chat_id, repo_owner, repo_name, events)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(chat_id, repo_owner, repo_name) DO UPDATE SET
			events = excluded.events
	`
	_, err = s.db.Exec(query, chatID, repoOwner, repoName, string(eventsJSON))
	return err
}

// Unsubscribe removes a subscription.
func (s *SubscriptionStore) Unsubscribe(chatID int64, repoOwner, repoName string) error {
	query := `DELETE FROM subscriptions WHERE chat_id = ? AND repo_owner = ? AND repo_name = ?`
	result, err := s.db.Exec(query, chatID, repoOwner, repoName)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.New("subscription not found")
	}
	return nil
}

// GetSubscriptionsByChat returns all subscriptions for a chat.
func (s *SubscriptionStore) GetSubscriptionsByChat(chatID int64) ([]Subscription, error) {
	var subs []Subscription
	query := `SELECT * FROM subscriptions WHERE chat_id = ? ORDER BY created_at DESC`
	err := s.db.Select(&subs, query, chatID)
	return subs, err
}

// GetSubscriptionsByRepo returns all subscriptions for a repository.
func (s *SubscriptionStore) GetSubscriptionsByRepo(repoOwner, repoName string) ([]Subscription, error) {
	var subs []Subscription
	query := `SELECT * FROM subscriptions WHERE repo_owner = ? AND repo_name = ?`
	err := s.db.Select(&subs, query, repoOwner, repoName)
	return subs, err
}

// GetSubscription returns a specific subscription.
func (s *SubscriptionStore) GetSubscription(chatID int64, repoOwner, repoName string) (*Subscription, error) {
	var sub Subscription
	query := `SELECT * FROM subscriptions WHERE chat_id = ? AND repo_owner = ? AND repo_name = ?`
	err := s.db.Get(&sub, query, chatID, repoOwner, repoName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &sub, err
}

// GetAllSubscribedRepos returns all unique repositories with subscriptions.
func (s *SubscriptionStore) GetAllSubscribedRepos() ([][2]string, error) {
	var repos []struct {
		RepoOwner string `db:"repo_owner"`
		RepoName  string `db:"repo_name"`
	}
	query := `SELECT DISTINCT repo_owner, repo_name FROM subscriptions`
	err := s.db.Select(&repos, query)
	if err != nil {
		return nil, err
	}

	result := make([][2]string, len(repos))
	for i, r := range repos {
		result[i] = [2]string{r.RepoOwner, r.RepoName}
	}
	return result, nil
}

// RecordEvent records a processed event for deduplication.
func (s *SubscriptionStore) RecordEvent(repoOwner, repoName, eventType, eventID string) error {
	query := `
		INSERT OR IGNORE INTO event_records (repo_owner, repo_name, event_type, event_id)
		VALUES (?, ?, ?, ?)
	`
	_, err := s.db.Exec(query, repoOwner, repoName, eventType, eventID)
	return err
}

// IsEventProcessed checks if an event has already been processed.
func (s *SubscriptionStore) IsEventProcessed(repoOwner, repoName, eventType, eventID string) (bool, error) {
	var count int
	query := `
		SELECT COUNT(*) FROM event_records 
		WHERE repo_owner = ? AND repo_name = ? AND event_type = ? AND event_id = ?
	`
	err := s.db.Get(&count, query, repoOwner, repoName, eventType, eventID)
	return count > 0, err
}

// CleanupOldEvents removes old event records to prevent database bloat.
func (s *SubscriptionStore) CleanupOldEvents(daysToKeep int) (int64, error) {
	query := `DELETE FROM event_records WHERE created_at < datetime('now', '-' || ? || ' days')`
	result, err := s.db.Exec(query, daysToKeep)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetSubscribedEvents returns the event types for a subscription.
func (s *SubscriptionStore) GetSubscribedEvents(chatID int64, repoOwner, repoName string) ([]EventType, error) {
	sub, err := s.GetSubscription(chatID, repoOwner, repoName)
	if err != nil {
		return nil, err
	}
	if sub == nil {
		return nil, nil
	}

	var events []EventType
	if err := json.Unmarshal([]byte(sub.Events), &events); err != nil {
		return nil, fmt.Errorf("failed to unmarshal events: %w", err)
	}
	return events, nil
}
