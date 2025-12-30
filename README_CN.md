# <div align="center">GitHub Telegram Bot</div>

<div align="center">
  <strong>通过 Telegram 监控任意 GitHub 公有仓库</strong>
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
  <a href="#功能特性">功能特性</a> •
  <a href="#快速开始">快速开始</a> •
  <a href="#bot-命令">命令列表</a> •
  <a href="README.md">English</a>
</div>

---

## 功能特性

- **Push 监控** - 实时接收新提交通知
- **Release 监控** - 新版本发布提醒
- **Issue 监控** - Issue 创建/关闭/重开通知
- **Pull Request 监控** - PR 状态变更提醒
- **监控任意公有仓库** - 不需要仓库管理权限
- **持久化存储** - SQLite 数据库存储订阅信息

## 项目结构

```
githubbot/
├── cmd/bot/              # 程序入口
├── internal/
│   ├── config/           # 配置管理
│   ├── github/           # GitHub API、Webhook 和轮询
│   ├── notifier/         # 通知服务
│   ├── storage/          # 数据存储
│   └── telegram/         # Telegram Bot
├── pkg/logger/           # 日志工具
├── configs/              # 配置文件
├── Dockerfile
└── docker-compose.yml
```

## 监控模式

| 模式 | 说明 | 适用场景 |
|------|------|----------|
| **polling** (推荐) | 定时轮询 GitHub API | 监控任意公有仓库 |
| **webhook** | 接收 GitHub 推送 | 仅限有管理权限的仓库 |
| **both** | 同时启用两种模式 | 混合场景 |

> **推荐使用 polling 模式**，因为它可以监控任何公有仓库，无需配置 Webhook。

## 快速开始

### 前置要求

- Go 1.21+
- Telegram Bot Token (从 [@BotFather](https://t.me/BotFather) 获取)
- GitHub Personal Access Token (可选，但强烈建议)

### 安装步骤

1. **克隆仓库**
   ```bash
   git clone https://github.com/your-username/githubbot.git
   cd githubbot
   ```

2. **安装依赖**
   ```bash
   go mod download
   ```

3. **配置**
   ```bash
   cp configs/config.example.yaml configs/config.yaml
   # 编辑 config.yaml，填入你的配置
   ```

4. **运行**
   ```bash
   go run ./cmd/bot -config configs/config.yaml
   ```

### Docker 部署

**使用 Docker Hub 镜像：**

```bash
# 1. 拉取镜像
docker pull gemiluxvii/github-telegram-bot:latest

# 2. 创建配置文件（重要：挂载前必须先创建文件）
mkdir -p configs data
docker run --rm gemiluxvii/github-telegram-bot:latest \
  cat /app/configs/config.example.yaml > configs/config.yaml

# 3. 编辑配置文件，填入你的 Token
vi configs/config.yaml

# 4. 启动容器
docker run -d --name github-bot \
  -v $(pwd)/configs/config.yaml:/app/configs/config.yaml \
  -v $(pwd)/data:/app/data \
  gemiluxvii/github-telegram-bot:latest
```

**使用 docker-compose：**

```bash
mkdir -p configs data
docker run --rm gemiluxvii/github-telegram-bot:latest \
  cat /app/configs/config.example.yaml > configs/config.yaml
vi configs/config.yaml
docker-compose up -d
```

## 配置说明

```yaml
telegram:
  token: "YOUR_BOT_TOKEN"
  debug: false

github:
  token: "ghp_xxxx"           # 强烈建议设置
  mode: "polling"             # polling / webhook / both
  poll_interval: 300          # 轮询间隔 (秒)

database:
  path: "./data/bot.db"

server:
  host: "0.0.0.0"
  port: 8080
```

### 环境变量配置

格式: `GHBOT_<SECTION>_<KEY>`

```bash
export GHBOT_TELEGRAM_TOKEN="your-bot-token"
export GHBOT_GITHUB_TOKEN="ghp_xxxx"
export GHBOT_GITHUB_MODE="polling"
```

### GitHub Token

- **无 Token**: 60 次请求/小时
- **有 Token**: 5000 次请求/小时

获取地址: https://github.com/settings/tokens

## Bot 命令

| 命令 | 说明 |
|------|------|
| `/start` | 显示欢迎信息 |
| `/help` | 显示帮助文档 |
| `/subscribe <owner/repo>` | 订阅仓库 |
| `/unsubscribe <owner/repo>` | 取消订阅 |
| `/list` | 查看当前订阅 |
| `/status` | 显示 Bot 状态和 API 配额 |

**快捷命令：** `/sub`, `/unsub`

## 使用示例

```
/subscribe torvalds/linux
/subscribe microsoft/vscode
/subscribe golang/go
```

## License

MIT License
