package telegram

import (
	"github.com/user/githubbot/internal/github"
)

// MessageBuilder helps construct formatted notification messages.
type MessageBuilder struct{}

// NewMessageBuilder creates a new message builder.
func NewMessageBuilder() *MessageBuilder {
	return &MessageBuilder{}
}

// BuildPushMessage creates a notification message for push events.
func (m *MessageBuilder) BuildPushMessage(repoOwner, repoName string, event *github.PushEvent) string {
	return event.FormatMessage(github.RepoInfo{Owner: repoOwner, Name: repoName})
}

// BuildReleaseMessage creates a notification message for release events.
func (m *MessageBuilder) BuildReleaseMessage(repoOwner, repoName string, event *github.ReleaseEvent) string {
	return event.FormatMessage(github.RepoInfo{Owner: repoOwner, Name: repoName})
}

// BuildIssueMessage creates a notification message for issue events.
func (m *MessageBuilder) BuildIssueMessage(repoOwner, repoName string, event *github.IssueEvent) string {
	return event.FormatMessage(github.RepoInfo{Owner: repoOwner, Name: repoName})
}

// BuildPRMessage creates a notification message for pull request events.
func (m *MessageBuilder) BuildPRMessage(repoOwner, repoName string, event *github.PullRequestEvent) string {
	return event.FormatMessage(github.RepoInfo{Owner: repoOwner, Name: repoName})
}
