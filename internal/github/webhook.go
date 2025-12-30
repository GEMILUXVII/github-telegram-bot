package github

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/user/githubbot/pkg/logger"
)

// WebhookHandler handles incoming GitHub webhooks.
type WebhookHandler struct {
	secret   string
	eventsCh chan<- *WebhookEvent
}

// WebhookEvent represents a parsed webhook event.
type WebhookEvent struct {
	Type      string // push, release, issues, pull_request
	RepoOwner string
	RepoName  string
	Payload   interface{} // PushEvent, ReleaseEvent, etc.
}

// NewWebhookHandler creates a new webhook handler.
func NewWebhookHandler(secret string, eventsCh chan<- *WebhookEvent) *WebhookHandler {
	return &WebhookHandler{
		secret:   secret,
		eventsCh: eventsCh,
	}
}

// ServeHTTP handles incoming webhook requests.
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to read webhook body")
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Verify signature if secret is set
	if h.secret != "" {
		signature := r.Header.Get("X-Hub-Signature-256")
		if !h.verifySignature(body, signature) {
			logger.Warn().Msg("Invalid webhook signature")
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// Get event type
	eventType := r.Header.Get("X-GitHub-Event")
	if eventType == "" {
		http.Error(w, "Missing event type", http.StatusBadRequest)
		return
	}

	// Parse and handle event
	event, err := h.parseEvent(eventType, body)
	if err != nil {
		logger.Error().Err(err).Str("event_type", eventType).Msg("Failed to parse event")
		http.Error(w, "Failed to parse event", http.StatusBadRequest)
		return
	}

	if event != nil {
		// Send event to channel for processing
		select {
		case h.eventsCh <- event:
			logger.Info().
				Str("type", event.Type).
				Str("repo", fmt.Sprintf("%s/%s", event.RepoOwner, event.RepoName)).
				Msg("Webhook event received")
		default:
			logger.Warn().Msg("Event channel full, dropping event")
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// verifySignature verifies the GitHub webhook signature.
func (h *WebhookHandler) verifySignature(body []byte, signature string) bool {
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}

	sig, err := hex.DecodeString(signature[7:])
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write(body)
	expected := mac.Sum(nil)

	return hmac.Equal(sig, expected)
}

// parseEvent parses a GitHub webhook event.
func (h *WebhookHandler) parseEvent(eventType string, body []byte) (*WebhookEvent, error) {
	// First, extract repository info common to all events
	var baseEvent struct {
		Repository struct {
			Owner struct {
				Login string `json:"login"`
			} `json:"owner"`
			Name string `json:"name"`
		} `json:"repository"`
	}

	if err := json.Unmarshal(body, &baseEvent); err != nil {
		return nil, fmt.Errorf("failed to parse base event: %w", err)
	}

	repoOwner := baseEvent.Repository.Owner.Login
	repoName := baseEvent.Repository.Name

	var payload interface{}

	switch eventType {
	case "push":
		var pushPayload struct {
			Ref     string `json:"ref"`
			Before  string `json:"before"`
			After   string `json:"after"`
			Compare string `json:"compare"`
			Pusher  struct {
				Name string `json:"name"`
			} `json:"pusher"`
			HeadCommit *struct {
				ID        string `json:"id"`
				Message   string `json:"message"`
				Timestamp string `json:"timestamp"`
				URL       string `json:"url"`
				Author    struct {
					Name  string `json:"name"`
					Email string `json:"email"`
				} `json:"author"`
			} `json:"head_commit"`
			Commits []struct {
				ID        string   `json:"id"`
				Message   string   `json:"message"`
				Timestamp string   `json:"timestamp"`
				URL       string   `json:"url"`
				Added     []string `json:"added"`
				Removed   []string `json:"removed"`
				Modified  []string `json:"modified"`
				Author    struct {
					Name  string `json:"name"`
					Email string `json:"email"`
				} `json:"author"`
			} `json:"commits"`
		}

		if err := json.Unmarshal(body, &pushPayload); err != nil {
			return nil, fmt.Errorf("failed to parse push event: %w", err)
		}

		commits := make([]CommitInfo, len(pushPayload.Commits))
		for i, c := range pushPayload.Commits {
			commits[i] = CommitInfo{
				SHA:      c.ID,
				Message:  c.Message,
				URL:      c.URL,
				Author:   UserInfo{Login: c.Author.Name},
				Added:    c.Added,
				Removed:  c.Removed,
				Modified: c.Modified,
			}
		}

		payload = &PushEvent{
			Ref:     pushPayload.Ref,
			Before:  pushPayload.Before,
			After:   pushPayload.After,
			Compare: pushPayload.Compare,
			Pusher:  UserInfo{Login: pushPayload.Pusher.Name},
			Commits: commits,
		}

	case "release":
		var releasePayload struct {
			Action  string `json:"action"`
			Release struct {
				TagName     string `json:"tag_name"`
				Name        string `json:"name"`
				Body        string `json:"body"`
				Draft       bool   `json:"draft"`
				Prerelease  bool   `json:"prerelease"`
				HTMLURL     string `json:"html_url"`
				PublishedAt string `json:"published_at"`
				Author      struct {
					Login     string `json:"login"`
					AvatarURL string `json:"avatar_url"`
					HTMLURL   string `json:"html_url"`
				} `json:"author"`
			} `json:"release"`
		}

		if err := json.Unmarshal(body, &releasePayload); err != nil {
			return nil, fmt.Errorf("failed to parse release event: %w", err)
		}

		// Only notify for published releases
		if releasePayload.Action != "published" {
			return nil, nil
		}

		payload = &ReleaseEvent{
			Action:     releasePayload.Action,
			TagName:    releasePayload.Release.TagName,
			Name:       releasePayload.Release.Name,
			Body:       releasePayload.Release.Body,
			Draft:      releasePayload.Release.Draft,
			Prerelease: releasePayload.Release.Prerelease,
			URL:        releasePayload.Release.HTMLURL,
			Author: UserInfo{
				Login:     releasePayload.Release.Author.Login,
				AvatarURL: releasePayload.Release.Author.AvatarURL,
				URL:       releasePayload.Release.Author.HTMLURL,
			},
		}

	case "issues":
		var issuePayload struct {
			Action string `json:"action"`
			Issue  struct {
				Number  int    `json:"number"`
				Title   string `json:"title"`
				Body    string `json:"body"`
				State   string `json:"state"`
				HTMLURL string `json:"html_url"`
				User    struct {
					Login     string `json:"login"`
					AvatarURL string `json:"avatar_url"`
					HTMLURL   string `json:"html_url"`
				} `json:"user"`
				Labels []struct {
					Name string `json:"name"`
				} `json:"labels"`
				Assignee *struct {
					Login     string `json:"login"`
					AvatarURL string `json:"avatar_url"`
					HTMLURL   string `json:"html_url"`
				} `json:"assignee"`
			} `json:"issue"`
		}

		if err := json.Unmarshal(body, &issuePayload); err != nil {
			return nil, fmt.Errorf("failed to parse issue event: %w", err)
		}

		// Only notify for specific actions
		if issuePayload.Action != "opened" && issuePayload.Action != "closed" && issuePayload.Action != "reopened" {
			return nil, nil
		}

		labels := make([]string, len(issuePayload.Issue.Labels))
		for i, l := range issuePayload.Issue.Labels {
			labels[i] = l.Name
		}

		var assignee *UserInfo
		if issuePayload.Issue.Assignee != nil {
			assignee = &UserInfo{
				Login:     issuePayload.Issue.Assignee.Login,
				AvatarURL: issuePayload.Issue.Assignee.AvatarURL,
				URL:       issuePayload.Issue.Assignee.HTMLURL,
			}
		}

		payload = &IssueEvent{
			Action: issuePayload.Action,
			Number: issuePayload.Issue.Number,
			Title:  issuePayload.Issue.Title,
			Body:   issuePayload.Issue.Body,
			State:  issuePayload.Issue.State,
			URL:    issuePayload.Issue.HTMLURL,
			User: UserInfo{
				Login:     issuePayload.Issue.User.Login,
				AvatarURL: issuePayload.Issue.User.AvatarURL,
				URL:       issuePayload.Issue.User.HTMLURL,
			},
			Labels:   labels,
			Assignee: assignee,
		}

	case "pull_request":
		var prPayload struct {
			Action      string `json:"action"`
			PullRequest struct {
				Number    int    `json:"number"`
				Title     string `json:"title"`
				Body      string `json:"body"`
				State     string `json:"state"`
				HTMLURL   string `json:"html_url"`
				Merged    bool   `json:"merged"`
				Additions int    `json:"additions"`
				Deletions int    `json:"deletions"`
				Commits   int    `json:"commits"`
				User      struct {
					Login     string `json:"login"`
					AvatarURL string `json:"avatar_url"`
					HTMLURL   string `json:"html_url"`
				} `json:"user"`
				MergedBy *struct {
					Login     string `json:"login"`
					AvatarURL string `json:"avatar_url"`
					HTMLURL   string `json:"html_url"`
				} `json:"merged_by"`
				Base struct {
					Ref string `json:"ref"`
					SHA string `json:"sha"`
				} `json:"base"`
				Head struct {
					Ref string `json:"ref"`
					SHA string `json:"sha"`
				} `json:"head"`
			} `json:"pull_request"`
		}

		if err := json.Unmarshal(body, &prPayload); err != nil {
			return nil, fmt.Errorf("failed to parse pull request event: %w", err)
		}

		// Only notify for specific actions
		if prPayload.Action != "opened" && prPayload.Action != "closed" && prPayload.Action != "reopened" {
			return nil, nil
		}

		var mergedBy *UserInfo
		if prPayload.PullRequest.MergedBy != nil {
			mergedBy = &UserInfo{
				Login:     prPayload.PullRequest.MergedBy.Login,
				AvatarURL: prPayload.PullRequest.MergedBy.AvatarURL,
				URL:       prPayload.PullRequest.MergedBy.HTMLURL,
			}
		}

		payload = &PullRequestEvent{
			Action:    prPayload.Action,
			Number:    prPayload.PullRequest.Number,
			Title:     prPayload.PullRequest.Title,
			Body:      prPayload.PullRequest.Body,
			State:     prPayload.PullRequest.State,
			URL:       prPayload.PullRequest.HTMLURL,
			Merged:    prPayload.PullRequest.Merged,
			MergedBy:  mergedBy,
			Additions: prPayload.PullRequest.Additions,
			Deletions: prPayload.PullRequest.Deletions,
			Commits:   prPayload.PullRequest.Commits,
			User: UserInfo{
				Login:     prPayload.PullRequest.User.Login,
				AvatarURL: prPayload.PullRequest.User.AvatarURL,
				URL:       prPayload.PullRequest.User.HTMLURL,
			},
			Base: BranchInfo{Ref: prPayload.PullRequest.Base.Ref, SHA: prPayload.PullRequest.Base.SHA},
			Head: BranchInfo{Ref: prPayload.PullRequest.Head.Ref, SHA: prPayload.PullRequest.Head.SHA},
		}

	default:
		// Ignore unsupported event types
		logger.Debug().Str("event_type", eventType).Msg("Ignoring unsupported event type")
		return nil, nil
	}

	return &WebhookEvent{
		Type:      eventType,
		RepoOwner: repoOwner,
		RepoName:  repoName,
		Payload:   payload,
	}, nil
}
