package github

import (
	"context"
	"fmt"
	"sync"
	"time"

	gh "github.com/google/go-github/v57/github"
	"github.com/user/githubbot/internal/storage"
	"github.com/user/githubbot/pkg/logger"
)

// Poller periodically checks GitHub repositories for updates.
type Poller struct {
	client    *Client
	store     *storage.SubscriptionStore
	eventsCh  chan<- *WebhookEvent
	interval  time.Duration
	startTime time.Time // 记录启动时间，只推送启动后的新事件

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewPoller creates a new repository poller.
func NewPoller(client *Client, store *storage.SubscriptionStore, eventsCh chan<- *WebhookEvent, intervalSeconds int) *Poller {
	ctx, cancel := context.WithCancel(context.Background())

	interval := time.Duration(intervalSeconds) * time.Second
	if interval < 60*time.Second {
		interval = 60 * time.Second // Minimum 1 minute to respect rate limits
	}

	return &Poller{
		client:    client,
		store:     store,
		eventsCh:  eventsCh,
		interval:  interval,
		startTime: time.Now(), // 记录启动时间
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start begins the polling loop.
func (p *Poller) Start() {
	p.wg.Add(1)
	go p.pollLoop()
	logger.Info().Dur("interval", p.interval).Msg("Poller started")
}

// Stop gracefully stops the poller.
func (p *Poller) Stop() {
	logger.Info().Msg("Stopping poller")
	p.cancel()
	p.wg.Wait()
}

// pollLoop is the main polling loop.
func (p *Poller) pollLoop() {
	defer p.wg.Done()

	// 首次轮询：只记录当前状态，不推送通知（静默初始化）
	p.initializeRepos()

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.pollAllRepos()
		}
	}
}

// initializeRepos 首次运行时记录已有事件，避免推送历史数据
func (p *Poller) initializeRepos() {
	repos, err := p.store.GetAllSubscribedRepos()
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get subscribed repos")
		return
	}

	if len(repos) == 0 {
		return
	}

	logger.Info().Int("count", len(repos)).Msg("Initializing repos (recording existing events, no notifications)")

	for _, repo := range repos {
		select {
		case <-p.ctx.Done():
			return
		default:
			p.recordExistingEvents(repo[0], repo[1])
		}
	}

	logger.Info().Msg("Initialization complete, will only notify new events from now on")
}

// recordExistingEvents 记录现有事件但不推送通知
func (p *Poller) recordExistingEvents(owner, name string) {
	ctx, cancel := context.WithTimeout(p.ctx, 30*time.Second)
	defer cancel()

	// 记录现有 commits
	commits, _, err := p.client.client.Repositories.ListCommits(ctx, owner, name, &gh.CommitsListOptions{
		ListOptions: gh.ListOptions{PerPage: 10},
	})
	if err == nil {
		for _, commit := range commits {
			sha := commit.GetSHA()
			if sha != "" {
				p.store.RecordEvent(owner, name, "push", sha)
			}
		}
	}

	// 记录现有 releases
	releases, _, err := p.client.client.Repositories.ListReleases(ctx, owner, name, &gh.ListOptions{PerPage: 5})
	if err == nil {
		for _, release := range releases {
			if !release.GetDraft() {
				eventID := fmt.Sprintf("release-%s", release.GetTagName())
				p.store.RecordEvent(owner, name, "release", eventID)
			}
		}
	}

	// 记录现有 issues (只记录 issue 编号，不再使用 UpdatedAt)
	issues, _, err := p.client.client.Issues.ListByRepo(ctx, owner, name, &gh.IssueListByRepoOptions{
		State:       "all",
		Sort:        "created",
		Direction:   "desc",
		ListOptions: gh.ListOptions{PerPage: 20},
	})
	if err == nil {
		for _, issue := range issues {
			if !issue.IsPullRequest() {
				// 使用 "issue-创建" 作为唯一标识，只通知新创建的 issue
				eventID := fmt.Sprintf("issue-%d-created", issue.GetNumber())
				p.store.RecordEvent(owner, name, "issues", eventID)
				// 同时记录关闭事件（如果已关闭）
				if issue.GetState() == "closed" {
					eventID = fmt.Sprintf("issue-%d-closed", issue.GetNumber())
					p.store.RecordEvent(owner, name, "issues", eventID)
				}
			}
		}
	}

	// 记录现有 PRs
	prs, _, err := p.client.client.PullRequests.List(ctx, owner, name, &gh.PullRequestListOptions{
		State:       "all",
		Sort:        "created",
		Direction:   "desc",
		ListOptions: gh.ListOptions{PerPage: 20},
	})
	if err == nil {
		for _, pr := range prs {
			// 使用 "pr-创建" 作为唯一标识
			eventID := fmt.Sprintf("pr-%d-created", pr.GetNumber())
			p.store.RecordEvent(owner, name, "pull_request", eventID)
			// 记录合并/关闭事件（如果已完成）
			if pr.GetState() == "closed" {
				if pr.GetMerged() {
					eventID = fmt.Sprintf("pr-%d-merged", pr.GetNumber())
				} else {
					eventID = fmt.Sprintf("pr-%d-closed", pr.GetNumber())
				}
				p.store.RecordEvent(owner, name, "pull_request", eventID)
			}
		}
	}

	logger.Debug().Str("repo", owner+"/"+name).Msg("Recorded existing events")
}

// pollAllRepos checks all subscribed repositories for updates.
func (p *Poller) pollAllRepos() {
	repos, err := p.store.GetAllSubscribedRepos()
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get subscribed repos")
		return
	}

	if len(repos) == 0 {
		return
	}

	logger.Debug().Int("count", len(repos)).Msg("Polling repositories")

	for _, repo := range repos {
		select {
		case <-p.ctx.Done():
			return
		default:
			p.pollRepo(repo[0], repo[1])
		}
	}
}

// pollRepo checks a single repository for updates.
func (p *Poller) pollRepo(owner, name string) {
	ctx, cancel := context.WithTimeout(p.ctx, 30*time.Second)
	defer cancel()

	// Check for new commits
	p.pollCommits(ctx, owner, name)

	// Check for new releases
	p.pollReleases(ctx, owner, name)

	// Check for new issues
	p.pollIssues(ctx, owner, name)

	// Check for new pull requests
	p.pollPullRequests(ctx, owner, name)
}

// pollCommits checks for new commits.
func (p *Poller) pollCommits(ctx context.Context, owner, name string) {
	commits, _, err := p.client.client.Repositories.ListCommits(ctx, owner, name, &gh.CommitsListOptions{
		Since:       p.startTime, // 只获取启动后的 commits
		ListOptions: gh.ListOptions{PerPage: 10},
	})
	if err != nil {
		logger.Debug().Err(err).Str("repo", owner+"/"+name).Msg("Failed to fetch commits")
		return
	}

	for _, commit := range commits {
		sha := commit.GetSHA()
		if sha == "" {
			continue
		}

		// Check if already processed
		processed, _ := p.store.IsEventProcessed(owner, name, "push", sha)
		if processed {
			continue
		}

		// Create push event
		event := &WebhookEvent{
			Type:      "push",
			RepoOwner: owner,
			RepoName:  name,
			Payload: &PushEvent{
				Ref:    "refs/heads/main",
				After:  sha,
				Pusher: UserInfo{Login: commit.GetAuthor().GetLogin()},
				Commits: []CommitInfo{{
					SHA:     sha,
					Message: commit.GetCommit().GetMessage(),
					URL:     commit.GetHTMLURL(),
					Author:  UserInfo{Login: commit.GetCommit().GetAuthor().GetName()},
				}},
				Compare: commit.GetHTMLURL(),
			},
		}

		select {
		case p.eventsCh <- event:
			logger.Debug().Str("repo", owner+"/"+name).Str("sha", sha[:7]).Msg("New commit detected")
		default:
			logger.Warn().Msg("Event channel full")
		}
	}
}

// pollReleases checks for new releases.
func (p *Poller) pollReleases(ctx context.Context, owner, name string) {
	releases, _, err := p.client.client.Repositories.ListReleases(ctx, owner, name, &gh.ListOptions{PerPage: 5})
	if err != nil {
		logger.Debug().Err(err).Str("repo", owner+"/"+name).Msg("Failed to fetch releases")
		return
	}

	for _, release := range releases {
		if release.GetDraft() {
			continue
		}

		// 只推送启动后发布的 release
		if release.GetPublishedAt().Time.Before(p.startTime) {
			continue
		}

		tagName := release.GetTagName()
		eventID := fmt.Sprintf("release-%s", tagName)

		processed, _ := p.store.IsEventProcessed(owner, name, "release", eventID)
		if processed {
			continue
		}

		event := &WebhookEvent{
			Type:      "release",
			RepoOwner: owner,
			RepoName:  name,
			Payload: &ReleaseEvent{
				Action:     "published",
				TagName:    tagName,
				Name:       release.GetName(),
				Body:       release.GetBody(),
				Prerelease: release.GetPrerelease(),
				URL:        release.GetHTMLURL(),
				Author:     UserInfo{Login: release.GetAuthor().GetLogin()},
			},
		}

		select {
		case p.eventsCh <- event:
			logger.Debug().Str("repo", owner+"/"+name).Str("tag", tagName).Msg("New release detected")
		default:
		}
	}
}

// pollIssues checks for NEW issues (created after bot start).
func (p *Poller) pollIssues(ctx context.Context, owner, name string) {
	// 只获取最近创建的 issues
	issues, _, err := p.client.client.Issues.ListByRepo(ctx, owner, name, &gh.IssueListByRepoOptions{
		State:       "all",
		Sort:        "created", // 按创建时间排序
		Direction:   "desc",
		Since:       p.startTime, // 只获取启动后的
		ListOptions: gh.ListOptions{PerPage: 10},
	})
	if err != nil {
		logger.Debug().Err(err).Str("repo", owner+"/"+name).Msg("Failed to fetch issues")
		return
	}

	for _, issue := range issues {
		// Skip pull requests
		if issue.IsPullRequest() {
			continue
		}

		// 只推送启动后创建的 issue
		if issue.GetCreatedAt().Time.Before(p.startTime) {
			// 但如果是关闭事件且在启动后关闭，也推送
			if issue.GetState() == "closed" {
				closedAt := issue.GetClosedAt()
				if !closedAt.IsZero() && closedAt.Time.After(p.startTime) {
					p.notifyIssueClosed(owner, name, issue)
				}
			}
			continue
		}

		number := issue.GetNumber()
		eventID := fmt.Sprintf("issue-%d-created", number)

		processed, _ := p.store.IsEventProcessed(owner, name, "issues", eventID)
		if processed {
			continue
		}

		labels := make([]string, len(issue.Labels))
		for i, l := range issue.Labels {
			labels[i] = l.GetName()
		}

		event := &WebhookEvent{
			Type:      "issues",
			RepoOwner: owner,
			RepoName:  name,
			Payload: &IssueEvent{
				Action: "opened",
				Number: number,
				Title:  issue.GetTitle(),
				Body:   issue.GetBody(),
				State:  issue.GetState(),
				URL:    issue.GetHTMLURL(),
				User:   UserInfo{Login: issue.GetUser().GetLogin()},
				Labels: labels,
			},
		}

		select {
		case p.eventsCh <- event:
			logger.Debug().Str("repo", owner+"/"+name).Int("issue", number).Msg("New issue detected")
		default:
		}
	}
}

// notifyIssueClosed 通知 issue 关闭
func (p *Poller) notifyIssueClosed(owner, name string, issue *gh.Issue) {
	number := issue.GetNumber()
	eventID := fmt.Sprintf("issue-%d-closed", number)

	processed, _ := p.store.IsEventProcessed(owner, name, "issues", eventID)
	if processed {
		return
	}

	labels := make([]string, len(issue.Labels))
	for i, l := range issue.Labels {
		labels[i] = l.GetName()
	}

	event := &WebhookEvent{
		Type:      "issues",
		RepoOwner: owner,
		RepoName:  name,
		Payload: &IssueEvent{
			Action: "closed",
			Number: number,
			Title:  issue.GetTitle(),
			Body:   issue.GetBody(),
			State:  "closed",
			URL:    issue.GetHTMLURL(),
			User:   UserInfo{Login: issue.GetUser().GetLogin()},
			Labels: labels,
		},
	}

	select {
	case p.eventsCh <- event:
		logger.Debug().Str("repo", owner+"/"+name).Int("issue", number).Msg("Issue closed detected")
	default:
	}
}

// pollPullRequests checks for NEW pull requests.
func (p *Poller) pollPullRequests(ctx context.Context, owner, name string) {
	prs, _, err := p.client.client.PullRequests.List(ctx, owner, name, &gh.PullRequestListOptions{
		State:       "all",
		Sort:        "created",
		Direction:   "desc",
		ListOptions: gh.ListOptions{PerPage: 10},
	})
	if err != nil {
		logger.Debug().Err(err).Str("repo", owner+"/"+name).Msg("Failed to fetch PRs")
		return
	}

	for _, pr := range prs {
		// 只推送启动后创建的 PR
		if pr.GetCreatedAt().Time.Before(p.startTime) {
			// 但如果是合并/关闭事件且在启动后发生，也推送
			if pr.GetState() == "closed" {
				closedAt := pr.GetClosedAt()
				if !closedAt.IsZero() && closedAt.Time.After(p.startTime) {
					p.notifyPRClosed(owner, name, pr)
				}
			}
			continue
		}

		number := pr.GetNumber()
		eventID := fmt.Sprintf("pr-%d-created", number)

		processed, _ := p.store.IsEventProcessed(owner, name, "pull_request", eventID)
		if processed {
			continue
		}

		event := &WebhookEvent{
			Type:      "pull_request",
			RepoOwner: owner,
			RepoName:  name,
			Payload: &PullRequestEvent{
				Action:    "opened",
				Number:    number,
				Title:     pr.GetTitle(),
				Body:      pr.GetBody(),
				State:     pr.GetState(),
				URL:       pr.GetHTMLURL(),
				Merged:    pr.GetMerged(),
				User:      UserInfo{Login: pr.GetUser().GetLogin()},
				Additions: pr.GetAdditions(),
				Deletions: pr.GetDeletions(),
				Commits:   pr.GetCommits(),
				Base:      BranchInfo{Ref: pr.GetBase().GetRef()},
				Head:      BranchInfo{Ref: pr.GetHead().GetRef()},
			},
		}

		select {
		case p.eventsCh <- event:
			logger.Debug().Str("repo", owner+"/"+name).Int("pr", number).Msg("New PR detected")
		default:
		}
	}
}

// notifyPRClosed 通知 PR 关闭/合并
func (p *Poller) notifyPRClosed(owner, name string, pr *gh.PullRequest) {
	number := pr.GetNumber()
	merged := pr.GetMerged()

	var eventID string
	var action string
	if merged {
		eventID = fmt.Sprintf("pr-%d-merged", number)
		action = "merged"
	} else {
		eventID = fmt.Sprintf("pr-%d-closed", number)
		action = "closed"
	}

	processed, _ := p.store.IsEventProcessed(owner, name, "pull_request", eventID)
	if processed {
		return
	}

	event := &WebhookEvent{
		Type:      "pull_request",
		RepoOwner: owner,
		RepoName:  name,
		Payload: &PullRequestEvent{
			Action:    action,
			Number:    number,
			Title:     pr.GetTitle(),
			Body:      pr.GetBody(),
			State:     "closed",
			URL:       pr.GetHTMLURL(),
			Merged:    merged,
			User:      UserInfo{Login: pr.GetUser().GetLogin()},
			Additions: pr.GetAdditions(),
			Deletions: pr.GetDeletions(),
			Commits:   pr.GetCommits(),
			Base:      BranchInfo{Ref: pr.GetBase().GetRef()},
			Head:      BranchInfo{Ref: pr.GetHead().GetRef()},
		},
	}

	select {
	case p.eventsCh <- event:
		logger.Debug().Str("repo", owner+"/"+name).Int("pr", number).Str("action", action).Msg("PR closed/merged detected")
	default:
	}
}
