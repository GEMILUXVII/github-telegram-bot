package github

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
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

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>[PUSH]</b> %s/%s\n\n", escapeHTML(repo.Owner), escapeHTML(repo.Name)))
	sb.WriteString(fmt.Sprintf("Branch: %s\n", escapeHTML(branch)))
	sb.WriteString(fmt.Sprintf("Author: %s\n", escapeHTML(e.Pusher.Login)))
	sb.WriteString(fmt.Sprintf("Commits: %d\n\n", commitCount))

	// Show up to 5 commits
	maxCommits := 5
	if len(e.Commits) < maxCommits {
		maxCommits = len(e.Commits)
	}

	for i := 0; i < maxCommits; i++ {
		commit := e.Commits[i]
		shortSHA := commit.SHA[:7]
		shortMsg := escapeHTML(sanitizeUTF8(truncateString(getFirstLine(commit.Message), 50)))
		sb.WriteString(fmt.Sprintf("<code>%s</code> %s\n", shortSHA, shortMsg))
	}

	if len(e.Commits) > 5 {
		sb.WriteString(fmt.Sprintf("\n... and %d more\n", len(e.Commits)-5))
	}

	sb.WriteString(fmt.Sprintf("\n<a href=\"%s\">View Changes</a>", escapeHTMLAttr(e.Compare)))

	return sb.String()
}

// FormatReleaseMessage formats a release event as a notification message.
func (e *ReleaseEvent) FormatMessage(repo RepoInfo) string {
	name := e.Name
	if name == "" {
		name = e.TagName
	}

	label := "RELEASE"
	if e.Prerelease {
		label = "PRE-RELEASE"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>[%s]</b> %s/%s\n\n", label, escapeHTML(repo.Owner), escapeHTML(repo.Name)))
	sb.WriteString(fmt.Sprintf("Version: %s\n", escapeHTML(name)))
	sb.WriteString(fmt.Sprintf("Tag: %s\n", escapeHTML(e.TagName)))
	sb.WriteString(fmt.Sprintf("Author: %s\n", escapeHTML(e.Author.Login)))

	if e.Body != "" {
		body := escapeHTML(sanitizeUTF8(truncateString(e.Body, 200)))
		sb.WriteString(fmt.Sprintf("\n%s\n", body))
	}

	sb.WriteString(fmt.Sprintf("\n<a href=\"%s\">View Release</a>", escapeHTMLAttr(e.URL)))

	return sb.String()
}

// FormatIssueMessage formats an issue event as a notification message.
func (e *IssueEvent) FormatMessage(repo RepoInfo) string {
	label := fmt.Sprintf("ISSUE %s", strings.ToUpper(e.Action))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>[%s]</b> %s/%s\n\n", label, escapeHTML(repo.Owner), escapeHTML(repo.Name)))
	sb.WriteString(fmt.Sprintf("#%d %s\n", e.Number, escapeHTML(sanitizeUTF8(e.Title))))
	sb.WriteString(fmt.Sprintf("Author: %s\n", escapeHTML(e.User.Login)))

	if len(e.Labels) > 0 {
		escapedLabels := make([]string, len(e.Labels))
		for i, l := range e.Labels {
			escapedLabels[i] = escapeHTML(l)
		}
		sb.WriteString(fmt.Sprintf("Labels: %s\n", strings.Join(escapedLabels, ", ")))
	}

	sb.WriteString(fmt.Sprintf("\n<a href=\"%s\">View Issue</a>", escapeHTMLAttr(e.URL)))

	return sb.String()
}

// FormatPRMessage formats a pull request event as a notification message.
func (e *PullRequestEvent) FormatMessage(repo RepoInfo) string {
	action := e.Action
	if e.Action == "closed" && e.Merged {
		action = "merged"
	}
	label := fmt.Sprintf("PR %s", strings.ToUpper(action))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>[%s]</b> %s/%s\n\n", label, escapeHTML(repo.Owner), escapeHTML(repo.Name)))
	sb.WriteString(fmt.Sprintf("#%d %s\n", e.Number, escapeHTML(sanitizeUTF8(e.Title))))
	sb.WriteString(fmt.Sprintf("Author: %s\n", escapeHTML(e.User.Login)))
	sb.WriteString(fmt.Sprintf("Branch: %s -> %s\n", escapeHTML(e.Head.Ref), escapeHTML(e.Base.Ref)))

	if e.Commits > 0 {
		sb.WriteString(fmt.Sprintf("Stats: %d commits, +%d/-%d lines\n", e.Commits, e.Additions, e.Deletions))
	}

	sb.WriteString(fmt.Sprintf("\n<a href=\"%s\">View Pull Request</a>", escapeHTMLAttr(e.URL)))

	return sb.String()
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

// escapeHTML escapes HTML special characters for text content.
func escapeHTML(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	)
	return replacer.Replace(s)
}

// escapeHTMLAttr escapes HTML special characters for use in attributes (like href).
func escapeHTMLAttr(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
	)
	return replacer.Replace(s)
}

// sanitizeUTF8 removes invalid UTF-8 sequences from a string.
func sanitizeUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	var sb strings.Builder
	for _, r := range s {
		if r != utf8.RuneError {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// getFirstLine returns the first line of a string.
func getFirstLine(s string) string {
	if idx := strings.Index(s, "\n"); idx != -1 {
		return s[:idx]
	}
	return s
}
