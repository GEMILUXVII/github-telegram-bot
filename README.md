# <div align="center">GitHub Telegram Bot</div>

<div align="center">
  <strong>Monitor any public GitHub repository via Telegram</strong>
</div>

<br>

<div align="center">
  <a href="#"><img src="https://img.shields.io/badge/version-v1.0.0-9644F4?style=for-the-badge" alt="Version"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-E53935?style=for-the-badge" alt="License"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.23+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go"></a>
  <a href="https://hub.docker.com/r/gemiluxvii/github-telegram-bot"><img src="https://img.shields.io/badge/Docker-Ready-2496ED?style=for-the-badge&logo=docker&logoColor=white" alt="Docker"></a>
</div>

<div align="center">
  <a href="https://github.com/"><img src="https://img.shields.io/badge/GitHub-API-181717?style=for-the-badge&logo=github&logoColor=white" alt="GitHub"></a>
  <a href="https://telegram.org/"><img src="https://img.shields.io/badge/Telegram-Bot-26A5E4?style=for-the-badge&logo=telegram&logoColor=white" alt="Telegram"></a>
  <a href="https://www.sqlite.org/"><img src="https://img.shields.io/badge/SQLite-Storage-003B57?style=for-the-badge&logo=sqlite&logoColor=white" alt="SQLite"></a>
</div>

<br>

<div align="center">
  <a href="#features">Features</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#bot-commands">Commands</a> •
  <a href="README_CN.md">中文文档</a>
</div>

---

## Features

- **Push Notifications** - Receive alerts for new commits
- **Release Notifications** - Get notified when new versions are published
- **Issue Tracking** - Monitor issue creation, closure, and reopening
- **Pull Request Tracking** - Track PR status changes
- **Monitor Any Public Repo** - No repository admin access required
- **Persistent Storage** - SQLite database for subscription management

## Project Structure

```
githubbot/
├── cmd/bot/              # Application entry point
├── internal/
│   ├── config/           # Configuration management
│   ├── github/           # GitHub API, Webhook, and Polling
│   ├── notifier/         # Notification service
│   ├── storage/          # Data persistence
│   └── telegram/         # Telegram Bot handlers
├── pkg/logger/           # Logging utilities
├── configs/              # Configuration files
├── Dockerfile
└── docker-compose.yml
```

## Monitoring Modes

| Mode | Description | Use Case |
|------|-------------|----------|
| **polling** (recommended) | Periodically polls GitHub API | Monitor any public repository |
| **webhook** | Receives GitHub push events | Only for repos you administer |
| **both** | Enables both modes | Mixed scenarios |

> **Note:** Polling mode is recommended as it can monitor any public repository without webhook configuration.

## Quick Start

### Prerequisites

- Go 1.21+
- Telegram Bot Token (from [@BotFather](https://t.me/BotFather))
- GitHub Personal Access Token (optional but strongly recommended)

### Installation

1. **Clone the repository**
   ```bash
   git clone https://github.com/your-username/githubbot.git
   cd githubbot
   ```

2. **Install dependencies**
   ```bash
   go mod download
   ```

3. **Configure**
   ```bash
   cp configs/config.example.yaml configs/config.yaml
   # Edit config.yaml with your settings
   ```

4. **Run**
   ```bash
   go run ./cmd/bot -config configs/config.yaml
   ```

### Docker Deployment

**Using Docker Hub image:**

```bash
# 1. Pull the image
docker pull gemiluxvii/github-telegram-bot:latest

# 2. Create config file (IMPORTANT: must exist before mounting)
mkdir -p configs data
docker run --rm gemiluxvii/github-telegram-bot:latest \
  cat /app/configs/config.example.yaml > configs/config.yaml

# 3. Edit config.yaml with your tokens
vi configs/config.yaml

# 4. Run the container
docker run -d --name github-bot \
  -v $(pwd)/configs/config.yaml:/app/configs/config.yaml \
  -v $(pwd)/data:/app/data \
  gemiluxvii/github-telegram-bot:latest
```

**Using docker-compose:**

```bash
mkdir -p configs data
docker run --rm gemiluxvii/github-telegram-bot:latest \
  cat /app/configs/config.example.yaml > configs/config.yaml
vi configs/config.yaml
docker-compose up -d
```

## Configuration

```yaml
telegram:
  token: "YOUR_BOT_TOKEN"
  debug: false

github:
  token: "ghp_xxxx"           # Strongly recommended
  mode: "polling"             # polling / webhook / both
  poll_interval: 300          # Seconds

database:
  path: "./data/bot.db"

server:
  host: "0.0.0.0"
  port: 8080
```

### Environment Variables

Format: `GHBOT_<SECTION>_<KEY>`

```bash
export GHBOT_TELEGRAM_TOKEN="your-bot-token"
export GHBOT_GITHUB_TOKEN="ghp_xxxx"
export GHBOT_GITHUB_MODE="polling"
```

### GitHub Token

- **Without Token**: 60 requests/hour
- **With Token**: 5000 requests/hour

Get one at: https://github.com/settings/tokens

## Bot Commands

| Command | Description |
|---------|-------------|
| `/start` | Display welcome message |
| `/help` | Show help documentation |
| `/subscribe <owner/repo>` | Subscribe to a repository |
| `/unsubscribe <owner/repo>` | Unsubscribe from a repository |
| `/list` | View current subscriptions |
| `/status` | Show bot status and API quota |

**Shortcuts:** `/sub`, `/unsub`

## Usage Examples

```
/subscribe torvalds/linux
/subscribe microsoft/vscode
/subscribe golang/go
```

## License

MIT License
