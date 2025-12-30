# GitHub Telegram Bot 🤖

一个使用 Go 语言开发的 Telegram 机器人，用于监控 **任意 GitHub 公有仓库** 的变动。

## ✨ 功能特性

- 📨 **Push 监控** - 实时接收新提交通知
- 🎉 **Release 监控** - 新版本发布提醒
- 📝 **Issue 监控** - Issue 创建/关闭/重开通知
- 🔀 **Pull Request 监控** - PR 状态变更提醒
- 🌍 **监控任意公有仓库** - 不需要仓库管理权限
- 💾 **持久化存储** - SQLite 数据库存储订阅信息

## 📁 项目结构

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

## 🔄 监控模式

| 模式 | 说明 | 适用场景 |
|------|------|----------|
| **polling** (推荐) | 定时轮询 GitHub API | 监控任意公有仓库 |
| **webhook** | 接收 GitHub 推送 | 仅限有管理权限的仓库 |
| **both** | 同时启用两种模式 | 混合场景 |

> 💡 **推荐使用 polling 模式**，因为它可以监控任何公有仓库，无需配置 Webhook。

## 🚀 快速开始

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

```bash
# 准备配置文件
cp configs/config.example.yaml configs/config.yaml
# 编辑配置...

# 启动
docker-compose up -d

# 查看日志
docker-compose logs -f
```

## ⚙️ 配置说明

```yaml
telegram:
  token: "YOUR_BOT_TOKEN"      # Telegram Bot Token
  debug: false                  # 调试模式

github:
  token: "ghp_xxxx"             # GitHub Token (强烈建议设置)
  mode: "polling"               # polling / webhook / both
  poll_interval: 300            # 轮询间隔 (秒)
  webhook_secret: ""            # Webhook 密钥 (webhook模式)

database:
  path: "./data/bot.db"         # 数据库路径

server:
  host: "0.0.0.0"               # 监听地址
  port: 8080                    # 监听端口
```

### 环境变量配置

所有配置项都可以通过环境变量设置，格式为 `GHBOT_<SECTION>_<KEY>`：

```bash
export GHBOT_TELEGRAM_TOKEN="your-bot-token"
export GHBOT_GITHUB_TOKEN="ghp_xxxx"
export GHBOT_GITHUB_MODE="polling"
export GHBOT_GITHUB_POLL_INTERVAL="300"
```

### GitHub Token

强烈建议配置 GitHub Token：
- **无 Token**: 60 次请求/小时
- **有 Token**: 5000 次请求/小时

获取地址: https://github.com/settings/tokens

## 🤖 Bot 命令

| 命令 | 说明 |
|------|------|
| `/start` | 显示欢迎信息 |
| `/help` | 显示帮助文档 |
| `/subscribe <owner/repo>` | 订阅仓库 |
| `/unsubscribe <owner/repo>` | 取消订阅 |
| `/list` | 查看当前订阅 |

**快捷命令：**
- `/sub` = `/subscribe`
- `/unsub` = `/unsubscribe`

## 📝 使用示例

1. 在 Telegram 中搜索你的 Bot 并开始对话
2. 发送 `/subscribe torvalds/linux` 订阅 Linux 内核仓库
3. 等待通知！当仓库有新的活动时，你将收到消息

**可以订阅任何公有仓库，例如：**
```
/subscribe microsoft/vscode
/subscribe golang/go
/subscribe facebook/react
/subscribe kubernetes/kubernetes
```

## 🛠️ 开发

```bash
# 运行测试
go test ./...

# 构建
go build -o bot ./cmd/bot

# 运行 (开发模式)
go run ./cmd/bot -config configs/config.yaml
```

## 📄 License

MIT License
