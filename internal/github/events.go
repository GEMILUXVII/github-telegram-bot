package github

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Timezone is the global timezone setting for formatting times.
// Set this from config at startup.
var Timezone *time.Location = time.UTC

// SetTimezone sets the global timezone for notifications.
func SetTimezone(tz string) error {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return err
	}
	Timezone = loc
	return nil
}

// formatTime formats a time in the configured timezone.
func formatTime(t time.Time) string {
	return t.In(Timezone).Format("2006-01-02 15:04 MST")
}

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
	Timestamp  time.Time // Push timestamp
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
	Additions int // Lines added
	Deletions int // Lines deleted
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
	Assets      []ReleaseAsset // Download assets
}

// ReleaseAsset represents a downloadable asset in a release.
type ReleaseAsset struct {
	Name          string
	DownloadURL   string
	Size          int64 // Size in bytes
	DownloadCount int
}

// IssueEvent represents an issue event.
type IssueEvent struct {
	Action    string // opened, closed, reopened, edited, etc.
	Number    int
	Title     string
	Body      string
	State     string // open, closed
	URL       string
	User      UserInfo
	Labels    []string
	Assignee  *UserInfo
	CreatedAt time.Time
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
	CreatedAt time.Time
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

	// Header with emoji tag
	sb.WriteString(fmt.Sprintf("<b>[📦 PUSH]</b> %s/%s\n", escapeHTML(repo.Owner), escapeHTML(repo.Name)))
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━\n")

	// Time - use first commit timestamp if available, or the push timestamp
	var pushTime time.Time
	if !e.Timestamp.IsZero() {
		pushTime = e.Timestamp
	} else if len(e.Commits) > 0 && !e.Commits[0].Timestamp.IsZero() {
		pushTime = e.Commits[0].Timestamp
	}
	if !pushTime.IsZero() {
		sb.WriteString(fmt.Sprintf("\n<i>%s</i>\n", formatTime(pushTime)))
	}

	// Calculate total changes
	var totalAdd, totalDel int
	for _, c := range e.Commits {
		totalAdd += c.Additions
		totalDel += c.Deletions
	}

	sb.WriteString(fmt.Sprintf("\n<b>Branch:</b> <code>%s</code>\n", escapeHTML(branch)))
	sb.WriteString(fmt.Sprintf("<b>Author:</b> %s\n", escapeHTML(e.Pusher.Login)))
	if totalAdd > 0 || totalDel > 0 {
		sb.WriteString(fmt.Sprintf("<b>Changes:</b> %d commit(s), <code>+%d</code> / <code>-%d</code>\n", commitCount, totalAdd, totalDel))
	} else {
		sb.WriteString(fmt.Sprintf("<b>Commits:</b> %d\n", commitCount))
	}

	// Show commits with better formatting
	maxCommits := 5
	if len(e.Commits) < maxCommits {
		maxCommits = len(e.Commits)
	}

	sb.WriteString("\n<b>Commit Details:</b>\n")
	for i := 0; i < maxCommits; i++ {
		commit := e.Commits[i]
		shortSHA := commit.SHA
		if len(shortSHA) > 7 {
			shortSHA = shortSHA[:7]
		}
		// Clean and format commit message
		commitMsg := stripMarkdown(sanitizeUTF8(getFirstLine(commit.Message)))
		commitMsg = truncateString(commitMsg, 50)
		sb.WriteString(fmt.Sprintf("• <code>%s</code> %s\n", shortSHA, escapeHTML(commitMsg)))
	}

	if len(e.Commits) > 5 {
		sb.WriteString(fmt.Sprintf("\n<i>... and %d more commit(s)</i>\n", len(e.Commits)-5))
	}

	sb.WriteString(fmt.Sprintf("\n<a href=\"%s\">View Changes on GitHub</a>", escapeHTMLAttr(e.Compare)))

	return sb.String()
}

// FormatReleaseMessage formats a release event as a notification message.
func (e *ReleaseEvent) FormatMessage(repo RepoInfo) string {
	name := e.Name
	if name == "" {
		name = e.TagName
	}

	label := "🚀 RELEASE"
	if e.Prerelease {
		label = "🧪 PRE-RELEASE"
	}

	var sb strings.Builder

	// Header with emoji tag
	sb.WriteString(fmt.Sprintf("<b>[%s]</b> %s/%s\n", label, escapeHTML(repo.Owner), escapeHTML(repo.Name)))
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━\n")

	// Time
	if !e.PublishedAt.IsZero() {
		sb.WriteString(fmt.Sprintf("\n<i>%s</i>\n", formatTime(e.PublishedAt)))
	}

	sb.WriteString(fmt.Sprintf("\n<b>Version:</b> <code>%s</code>\n", escapeHTML(name)))
	sb.WriteString(fmt.Sprintf("<b>Tag:</b> <code>%s</code>\n", escapeHTML(e.TagName)))
	sb.WriteString(fmt.Sprintf("<b>Author:</b> %s\n", escapeHTML(e.Author.Login)))

	// Body preview - strip markdown and show more content
	if e.Body != "" {
		cleanBody := stripMarkdown(sanitizeUTF8(e.Body))
		cleanBody = truncateString(cleanBody, 300)
		sb.WriteString(fmt.Sprintf("\n<b>Release Notes:</b>\n<i>%s</i>\n", escapeHTML(cleanBody)))
	}

	// Assets
	if len(e.Assets) > 0 {
		sb.WriteString("\n<b>Download Assets:</b>\n")
		maxAssets := 5
		if len(e.Assets) < maxAssets {
			maxAssets = len(e.Assets)
		}
		for i := 0; i < maxAssets; i++ {
			asset := e.Assets[i]
			sizeStr := formatFileSize(asset.Size)
			sb.WriteString(fmt.Sprintf("• %s <i>(%s)</i>\n", escapeHTML(asset.Name), sizeStr))
		}
		if len(e.Assets) > 5 {
			sb.WriteString(fmt.Sprintf("<i>... and %d more file(s)</i>\n", len(e.Assets)-5))
		}
	}

	sb.WriteString(fmt.Sprintf("\n<a href=\"%s\">View Release on GitHub</a>", escapeHTMLAttr(e.URL)))

	return sb.String()
}

// FormatIssueMessage formats an issue event as a notification message.
func (e *IssueEvent) FormatMessage(repo RepoInfo) string {
	// Emoji based on action
	var emoji, actionText string
	switch e.Action {
	case "opened":
		emoji = "🐛"
		actionText = "OPENED"
	case "closed":
		emoji = "✅"
		actionText = "CLOSED"
	case "reopened":
		emoji = "🔄"
		actionText = "REOPENED"
	default:
		emoji = "📋"
		actionText = strings.ToUpper(e.Action)
	}

	label := fmt.Sprintf("%s ISSUE %s", emoji, actionText)

	var sb strings.Builder

	// Header with emoji tag
	sb.WriteString(fmt.Sprintf("<b>[%s]</b> %s/%s\n", label, escapeHTML(repo.Owner), escapeHTML(repo.Name)))
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━\n")

	// Time
	if !e.CreatedAt.IsZero() {
		sb.WriteString(fmt.Sprintf("\n<i>%s</i>\n", formatTime(e.CreatedAt)))
	}

	// Issue title and number
	cleanTitle := stripMarkdown(sanitizeUTF8(e.Title))
	sb.WriteString(fmt.Sprintf("\n<b>#%d</b> %s\n", e.Number, escapeHTML(cleanTitle)))
	sb.WriteString(fmt.Sprintf("<b>Author:</b> %s\n", escapeHTML(e.User.Login)))

	// Labels
	if len(e.Labels) > 0 {
		escapedLabels := make([]string, len(e.Labels))
		for i, l := range e.Labels {
			escapedLabels[i] = fmt.Sprintf("<code>%s</code>", escapeHTML(l))
		}
		sb.WriteString(fmt.Sprintf("<b>Labels:</b> %s\n", strings.Join(escapedLabels, " ")))
	}

	// Assignee
	if e.Assignee != nil && e.Assignee.Login != "" {
		sb.WriteString(fmt.Sprintf("<b>Assignee:</b> %s\n", escapeHTML(e.Assignee.Login)))
	}

	// Body preview - strip markdown and show more content
	if e.Body != "" {
		cleanBody := stripMarkdown(sanitizeUTF8(e.Body))
		cleanBody = truncateString(cleanBody, 250)
		sb.WriteString(fmt.Sprintf("\n<b>Description:</b>\n%s\n", escapeHTML(cleanBody)))
	}

	sb.WriteString(fmt.Sprintf("\n<a href=\"%s\">View Issue on GitHub</a>", escapeHTMLAttr(e.URL)))

	return sb.String()
}

// FormatPRMessage formats a pull request event as a notification message.
func (e *PullRequestEvent) FormatMessage(repo RepoInfo) string {
	// Emoji based on action
	var emoji, actionText string
	action := e.Action
	if e.Action == "closed" && e.Merged {
		action = "merged"
	}

	switch action {
	case "opened":
		emoji = "🔀"
		actionText = "OPENED"
	case "merged":
		emoji = "🎉"
		actionText = "MERGED"
	case "closed":
		emoji = "❌"
		actionText = "CLOSED"
	case "reopened":
		emoji = "🔄"
		actionText = "REOPENED"
	default:
		emoji = "📝"
		actionText = strings.ToUpper(action)
	}

	label := fmt.Sprintf("%s PR %s", emoji, actionText)

	var sb strings.Builder

	// Header with emoji tag
	sb.WriteString(fmt.Sprintf("<b>[%s]</b> %s/%s\n", label, escapeHTML(repo.Owner), escapeHTML(repo.Name)))
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━\n")

	// Time
	if !e.CreatedAt.IsZero() {
		sb.WriteString(fmt.Sprintf("\n<i>%s</i>\n", e.CreatedAt.Format("2006-01-02 15:04 UTC")))
	}

	// PR title and number
	cleanTitle := stripMarkdown(sanitizeUTF8(e.Title))
	sb.WriteString(fmt.Sprintf("\n<b>#%d</b> %s\n", e.Number, escapeHTML(cleanTitle)))
	sb.WriteString(fmt.Sprintf("<b>Author:</b> %s\n", escapeHTML(e.User.Login)))
	sb.WriteString(fmt.Sprintf("<b>Branch:</b> <code>%s</code> → <code>%s</code>\n", escapeHTML(e.Head.Ref), escapeHTML(e.Base.Ref)))

	// Stats
	if e.Commits > 0 {
		sb.WriteString(fmt.Sprintf("<b>Stats:</b> %d commit(s), <code>+%d</code> / <code>-%d</code> lines\n", e.Commits, e.Additions, e.Deletions))
	}

	// Merged by
	if e.Merged && e.MergedBy != nil && e.MergedBy.Login != "" {
		sb.WriteString(fmt.Sprintf("<b>Merged by:</b> %s\n", escapeHTML(e.MergedBy.Login)))
	}

	// Body preview - strip markdown and show more content
	if e.Body != "" {
		cleanBody := stripMarkdown(sanitizeUTF8(e.Body))
		cleanBody = truncateString(cleanBody, 200)
		sb.WriteString(fmt.Sprintf("\n<b>Description:</b>\n%s\n", escapeHTML(cleanBody)))
	}

	sb.WriteString(fmt.Sprintf("\n<a href=\"%s\">View Pull Request on GitHub</a>", escapeHTMLAttr(e.URL)))

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

// formatFileSize formats bytes into human-readable size.
func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func truncateString(s string, maxLen int) string {
	// Count runes instead of bytes for proper unicode handling
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}

// stripMarkdown removes common markdown formatting from text.
// This converts markdown to plain text for cleaner Telegram display.
func stripMarkdown(s string) string {
	// Remove code blocks (``` ... ```) but try to keep content
	codeBlockRe := regexp.MustCompile("(?s)```(?:[a-zA-Z]*\n)?([^`]*)```")
	s = codeBlockRe.ReplaceAllString(s, "$1")

	// Remove inline code markers but keep the content inside
	inlineCodeRe := regexp.MustCompile("`([^`]+)`")
	s = inlineCodeRe.ReplaceAllString(s, "$1")

	// Remove headers (# ## ### etc.)
	headerRe := regexp.MustCompile(`(?m)^#{1,6}\s*`)
	s = headerRe.ReplaceAllString(s, "")

	// Remove bold (**text** or __text__) but keep content
	boldRe := regexp.MustCompile(`\*\*([^*]+)\*\*|__([^_]+)__`)
	s = boldRe.ReplaceAllString(s, "$1$2")

	// Remove italic (*text* or _text_) but keep content
	// Be careful not to match underscores in words like snake_case
	italicRe := regexp.MustCompile(`(?:^|[^a-zA-Z0-9])\*([^*]+)\*(?:[^a-zA-Z0-9]|$)|(?:^|[^a-zA-Z0-9])_([^_]+)_(?:[^a-zA-Z0-9]|$)`)
	s = italicRe.ReplaceAllString(s, "$1$2")

	// Remove strikethrough (~~text~~) but keep content
	strikeRe := regexp.MustCompile(`~~([^~]+)~~`)
	s = strikeRe.ReplaceAllString(s, "$1")

	// Remove links [text](url) -> keep text
	linkRe := regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	s = linkRe.ReplaceAllString(s, "$1")

	// Remove images ![alt](url) -> keep alt text
	imageRe := regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`)
	s = imageRe.ReplaceAllString(s, "$1")

	// Remove blockquotes (> text) but keep content
	blockquoteRe := regexp.MustCompile(`(?m)^>\s*`)
	s = blockquoteRe.ReplaceAllString(s, "")

	// Remove horizontal rules (---, ***, ___)
	hrRe := regexp.MustCompile(`(?m)^[-*_]{3,}\s*$`)
	s = hrRe.ReplaceAllString(s, "")

	// Remove list markers (- item, * item, + item, 1. item) but keep content
	listRe := regexp.MustCompile(`(?m)^[\s]*[-*+]\s+|^[\s]*\d+\.\s+`)
	s = listRe.ReplaceAllString(s, "• ")

	// Remove task list markers ([ ] or [x]) but keep content
	taskRe := regexp.MustCompile(`\[[ xX]\]\s*`)
	s = taskRe.ReplaceAllString(s, "")

	// Remove HTML tags
	htmlRe := regexp.MustCompile(`<[^>]+>`)
	s = htmlRe.ReplaceAllString(s, "")

	// Collapse multiple newlines into double newlines
	multiNewlineRe := regexp.MustCompile(`\n{3,}`)
	s = multiNewlineRe.ReplaceAllString(s, "\n\n")

	// Collapse multiple spaces
	multiSpaceRe := regexp.MustCompile(`[ \t]+`)
	s = multiSpaceRe.ReplaceAllString(s, " ")

	// Trim whitespace
	s = strings.TrimSpace(s)

	return s
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
