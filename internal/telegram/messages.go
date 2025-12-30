package telegram

import (
	"fmt"

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
	header := fmt.Sprintf("🔔 *%s/%s*\n\n", repoOwner, repoName)
	return header + event.FormatMessage(github.RepoInfo{Owner: repoOwner, Name: repoName})
}

// BuildReleaseMessage creates a notification message for release events.
func (m *MessageBuilder) BuildReleaseMessage(repoOwner, repoName string, event *github.ReleaseEvent) string {
	header := fmt.Sprintf("🔔 *%s/%s*\n\n", repoOwner, repoName)
	return header + event.FormatMessage(github.RepoInfo{Owner: repoOwner, Name: repoName})
}

// BuildIssueMessage creates a notification message for issue events.
func (m *MessageBuilder) BuildIssueMessage(repoOwner, repoName string, event *github.IssueEvent) string {
	header := fmt.Sprintf("🔔 *%s/%s*\n\n", repoOwner, repoName)
	return header + event.FormatMessage(github.RepoInfo{Owner: repoOwner, Name: repoName})
}

// BuildPRMessage creates a notification message for pull request events.
func (m *MessageBuilder) BuildPRMessage(repoOwner, repoName string, event *github.PullRequestEvent) string {
	header := fmt.Sprintf("🔔 *%s/%s*\n\n", repoOwner, repoName)
	return header + event.FormatMessage(github.RepoInfo{Owner: repoOwner, Name: repoName})
}

// FormatRepoLink creates a markdown link to a repository.
func FormatRepoLink(owner, name string) string {
	return fmt.Sprintf("[%s/%s](https://github.com/%s/%s)", owner, name, owner, name)
}

// FormatUserLink creates a markdown link to a user profile.
func FormatUserLink(username string) string {
	return fmt.Sprintf("[@%s](https://github.com/%s)", username, username)
}
