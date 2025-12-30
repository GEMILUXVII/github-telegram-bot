// Package github provides GitHub API client and webhook handling.
package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v57/github"
	"golang.org/x/oauth2"
)

// Client wraps the GitHub API client.
type Client struct {
	client *github.Client
}

// NewClient creates a new GitHub API client.
// If token is empty, an unauthenticated client is created (with lower rate limits).
func NewClient(token string) *Client {
	var client *github.Client

	if token != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		tc := oauth2.NewClient(context.Background(), ts)
		client = github.NewClient(tc)
	} else {
		client = github.NewClient(nil)
	}

	return &Client{client: client}
}

// RepoInfo contains basic repository information.
type RepoInfo struct {
	Owner       string
	Name        string
	FullName    string
	Description string
	Stars       int
	Forks       int
	URL         string
}

// GetRepository retrieves information about a repository.
func (c *Client) GetRepository(ctx context.Context, owner, repo string) (*RepoInfo, error) {
	r, _, err := c.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}

	return &RepoInfo{
		Owner:       owner,
		Name:        repo,
		FullName:    r.GetFullName(),
		Description: r.GetDescription(),
		Stars:       r.GetStargazersCount(),
		Forks:       r.GetForksCount(),
		URL:         r.GetHTMLURL(),
	}, nil
}

// ValidateRepository checks if a repository exists and is accessible.
func (c *Client) ValidateRepository(ctx context.Context, owner, repo string) (bool, error) {
	_, _, err := c.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		if _, ok := err.(*github.RateLimitError); ok {
			return false, fmt.Errorf("rate limit exceeded")
		}
		// Repository not found or private
		return false, nil
	}
	return true, nil
}

// GetRateLimit returns the current rate limit status.
func (c *Client) GetRateLimit(ctx context.Context) (*github.RateLimits, error) {
	limits, _, err := c.client.RateLimit.Get(ctx)
	if err != nil {
		return nil, err
	}
	return limits, nil
}
