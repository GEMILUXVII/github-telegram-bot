package github

import (
	"fmt"
	"strings"
	"time"
)

// Event represents a generic GitHub event.
type Event struct {
	Type      string
	Repo      RepoInfo
	Timestamp time.Time
	Payload   interface{}
}

// PushEvent represents a push (commits) event.
type PushEvent struct {
	Ref        string       // e.g., "refs/heads/main"
	Before     string       // SHA before push
	After      string       // SHA after push
	Commits    []CommitInfo // List of commits
	Pusher     UserInfo
	Compare    string // Comparison URL
	HeadCommit *CommitInfo
}

// CommitInfo represents commit information.
type CommitInfo struct {
	SHA       string
	Message   string
	Author    UserInfo
	URL       string
	Timestamp time.Time
	Added     []string
	Removed   []string
	Modified  []string
}

// ReleaseEvent represents a release event.
type ReleaseEvent struct {
	Action      string // published, created, edited, deleted, etc.
	TagName     string
	Name        string
	Body        string
	Draft       bool
	Prerelease  bool
	URL         string
	Author      UserInfo
	PublishedAt time.Time
}

// IssueEvent represents an issue event.
type IssueEvent struct {
	Action   string // opened, closed, reopened, edited, etc.
	Number   int
	Title    string
	Body     string
	State    string // open, closed
	URL      string
	User     UserInfo
	Labels   []string
	Assignee *UserInfo
}

// PullRequestEvent represents a pull request event.
type PullRequestEvent struct {
	Action    string // opened, closed, reopened, synchronize, etc.
	Number    int
	Title     string
	Body      string
	State     string // open, closed
	URL       string
	User      UserInfo
	Merged    bool
	MergedBy  *UserInfo
	Base      BranchInfo
	Head      BranchInfo
	Additions int
	Deletions int
	Commits   int
}

// BranchInfo represents branch information in a PR.
type BranchInfo struct {
	Ref  string
	SHA  string
	Repo string
}

// UserInfo represents a GitHub user.
type UserInfo struct {
	Login     string
	AvatarURL string
	URL       string
}

// FormatPushMessage formats a push event as a notification message.
func (e *PushEvent) FormatMessage(repo RepoInfo) string {
	branch := extractBranchName(e.Ref)
	commitCount := len(e.Commits)
	commitWord := "commit"
	if commitCount > 1 {
		commitWord = "commits"
	}

	msg := fmt.Sprintf("🔨 *%s* pushed %d %s to `%s`\n\n",
		e.Pusher.Login, commitCount, commitWord, branch)

	// Show up to 5 commits
	maxCommits := 5
	if len(e.Commits) < maxCommits {
		maxCommits = len(e.Commits)
	}

	for i := 0; i < maxCommits; i++ {
		commit := e.Commits[i]
		shortSHA := commit.SHA[:7]
		shortMsg := escapeMarkdown(truncateString(commit.Message, 50))
		msg += fmt.Sprintf("• [`%s`](%s) %s\n", shortSHA, commit.URL, shortMsg)
	}

	if len(e.Commits) > 5 {
		msg += fmt.Sprintf("\n_...and %d more commits_\n", len(e.Commits)-5)
	}

	msg += fmt.Sprintf("\n[Compare changes](%s)", e.Compare)

	return msg
}

// FormatReleaseMessage formats a release event as a notification message.
func (e *ReleaseEvent) FormatMessage(repo RepoInfo) string {
	emoji := "🎉"
	if e.Prerelease {
		emoji = "🧪"
	}

	name := e.Name
	if name == "" {
		name = e.TagName
	}

	msg := fmt.Sprintf("%s *New Release: %s*\n\n", emoji, name)
	msg += fmt.Sprintf("📦 Tag: `%s`\n", e.TagName)
	msg += fmt.Sprintf("👤 Author: %s\n", e.Author.Login)

	if e.Body != "" {
		body := truncateString(e.Body, 300)
		msg += fmt.Sprintf("\n%s\n", body)
	}

	msg += fmt.Sprintf("\n[View Release](%s)", e.URL)

	return msg
}

// FormatIssueMessage formats an issue event as a notification message.
func (e *IssueEvent) FormatMessage(repo RepoInfo) string {
	actionEmoji := map[string]string{
		"opened":   "📝",
		"closed":   "✅",
		"reopened": "🔄",
	}

	emoji := actionEmoji[e.Action]
	if emoji == "" {
		emoji = "📋"
	}

	msg := fmt.Sprintf("%s *Issue #%d %s*\n\n", emoji, e.Number, e.Action)
	msg += fmt.Sprintf("📌 %s\n", escapeMarkdown(e.Title))
	msg += fmt.Sprintf("👤 By: %s\n", escapeMarkdown(e.User.Login))

	if len(e.Labels) > 0 {
		msg += fmt.Sprintf("🏷️ Labels: %v\n", e.Labels)
	}

	msg += fmt.Sprintf("\n[View Issue](%s)", e.URL)

	return msg
}

// FormatPRMessage formats a pull request event as a notification message.
func (e *PullRequestEvent) FormatMessage(repo RepoInfo) string {
	actionEmoji := map[string]string{
		"opened":   "🔀",
		"closed":   "❌",
		"merged":   "🎊",
		"reopened": "🔄",
	}

	action := e.Action
	if e.Action == "closed" && e.Merged {
		action = "merged"
	}

	emoji := actionEmoji[action]
	if emoji == "" {
		emoji = "🔀"
	}

	msg := fmt.Sprintf("%s *PR #%d %s*\n\n", emoji, e.Number, action)
	msg += fmt.Sprintf("📌 %s\n", escapeMarkdown(e.Title))
	msg += fmt.Sprintf("👤 By: %s\n", escapeMarkdown(e.User.Login))
	msg += fmt.Sprintf("🔀 %s → %s\n", escapeMarkdown(e.Head.Ref), escapeMarkdown(e.Base.Ref))

	if e.Commits > 0 {
		msg += fmt.Sprintf("📊 %d commits, +%d/-%d lines\n", e.Commits, e.Additions, e.Deletions)
	}

	msg += fmt.Sprintf("\n[View PR](%s)", e.URL)

	return msg
}

// Helper functions

func extractBranchName(ref string) string {
	// refs/heads/main -> main
	if len(ref) > 11 && ref[:11] == "refs/heads/" {
		return ref[11:]
	}
	return ref
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// escapeMarkdown escapes special Markdown characters to prevent parsing errors.
func escapeMarkdown(s string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"~", "\\~",
		"`", "\\`",
		">", "\\>",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		"=", "\\=",
		"|", "\\|",
		"{", "\\{",
		"}", "\\}",
		".", "\\.",
		"!", "\\!",
	)
	return replacer.Replace(s)
}
