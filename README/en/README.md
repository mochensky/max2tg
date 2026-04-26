# max2tg

Bridge between the [MAX](https://max.ru) messenger and [Telegram](https://telegram.org). The bot connects to MAX via WebSocket on behalf of your account and forwards messages to the specified Telegram chats or topics in real time.

## Features

- **Message forwarding** - text, photos, audio, voice messages, videos, files
- **Forwarded messages** - displayed with the original sender
- **Editing** - when a message is edited in MAX, the corresponding message in Telegram is updated
- **Deletion** - deleted messages are either removed from Telegram or marked with `[Deleted ...]` (configurable)
- **System events** - joining, adding/removing participants, chat creation
- **History synchronization** - on startup and after reconnection, the latest missed messages are loaded
- **Routing** - each MAX chat can be routed to a separate Telegram chat or topic
- **Auto-reconnect** - on connection loss, the bot reconnects with exponential backoff
- **Debug notifications** - optionally sends private messages to Telegram about disconnections and reconnections
- **Logging** - each launch creates a separate log file

## Requirements

- Go 1.21+
- MAX account
- Telegram bot (token from [@BotFather](https://t.me/BotFather))
- Very low server resource requirements:
  - Runs on 48 MB of RAM (most commonly used less than 16 MB)
  - CPU load less than 1% of a single Intel Core I9-12900K processor core

## Installation and Build

You can download a ready-made build for your OS from the Releases section.

Or build the project manually:

```bash
git clone https://github.com/mochensky/max2tg
cd max2tg
go build -o max2tg .
```

Using the PowerShell script (Windows):

```powershell
.\build.ps1
```

## Obtaining MAX Credentials

The bot needs two values from MAX: **token** and **device ID**. You can get them through the browser DevTools.

1. Open [web.max.ru](https://web.max.ru) and log into your account.
2. Open DevTools (`F12` or `Ctrl+Shift+I`) → **Application** tab (Chrome/Edge) or **Storage** tab (Firefox).
3. Go to **Local Storage** → `https://web.max.ru`.
4. Find the key `__oneme_auth`.
5. Inside it, find `token` - this is your `MAX_TOKEN`.
6. Find the key `__oneme_device_id` - this is your `MAX_DEVICE_ID`.

> **Important:** The token is tied to the browser session. If you log out of your MAX account in the browser, the token will become invalid and you will need to obtain it again.

## Setup

### 1. First Launch

```bash
./max2tg
```

On the first launch, the program creates two files:

- `data/config.yml` - main configuration
- `data/.env` - secret variables (tokens)

After creation, the program will exit and ask you to fill in the credentials.

### 2. Fill in `data/.env`

```env
MAX_TOKEN=your_token_from_local_storage
MAX_DEVICE_ID=your_device_id_from_local_storage
TG_TOKEN=your_telegram_bot_token
TG_DEBUG_USER_ID=your_telegram_user_id
```

`TG_DEBUG_USER_ID` - your numeric Telegram user ID. If specified, the bot will send you private messages about disconnections and reconnections. Optional field.

### 3. Configure Routes in `data/config.yml`

The most important part is the `chats` section. Each route links one MAX chat to one Telegram chat (or topic).

```yaml
chats:
  # Route without topic: MAX chat → Telegram channel/group
  - max_chat_id: 123456789
    telegram_chat_id: -1001234567890
    telegram_topic_id: 0

  # Route with topic: MAX chat → topic in Telegram group
  - max_chat_id: 987654321
    telegram_chat_id: -1001234567890
    telegram_topic_id: 5
```

**How to get `max_chat_id`:** open the desired chat in [web.max.ru](https://web.max.ru) - the chat ID will be in the URL.

**How to get `telegram_chat_id`:** add the bot to the chat and use [@getidsbot](https://t.me/getidsbot) or any third-party Telegram client.

**Note:** The bot must have permissions to send messages, delete messages, send media, and manage topics.

### 4. Other Parameters in `config.yml`

```yaml
# data paths
log_path: "data/logs"
db_path: "data/database.db"
download_path: "data/downloads"

# how many recent chat messages will be checked for chat sync?
sync_history_depth: 30

# mark deleted messages with a marker instead of deleting them
save_deleted: true

# truncate long messages instead of skipping them (caption limit: 1024 chars, message limit: 4096 chars)
truncate_long_messages: true

# reconnection settings
max_retries: 5
base_retry_delay: 1s
ping_timeout: 90s
```

The `video_headers`, `audio_headers`, and `user_agent` sections contain working default values and usually do not need to be changed.

## Launch

```bash
./max2tg
```

On successful start you will see in the logs:

```
[16.04.2026 12:00:00] Starting max2tg 1.0.0...
[16.04.2026 12:00:00] Application is up to date (1.0.0)
...
[16.04.2026 12:00:01] Connected to WebSocket
...
[16.04.2026 12:00:01] Connected as Иван (ID: 12345678)
```

## Project Structure

```
├── src/
│   ├── client.go          - high-level MAX client
│   ├── config.go          - config loading and validation
│   ├── connection.go      - WebSocket connection and reconnection
│   ├── database.go        - SQLite: storing message IDs
│   ├── enums.go           - constants and types
│   ├── errors.go          - error types
│   ├── logger.go          - logging
│   ├── models.go          - data structures
│   ├── parser.go          - WebSocket API response parsing
│   ├── request_builder.go - request building
│   ├── sender.go          - sending to Telegram
│   ├── utils.go           - media file downloading
│   └── version.go         - working with versions
└── main.go                - entry point, event handlers
```

## License

MIT