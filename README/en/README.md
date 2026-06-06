# max2tg

A bridge between [MAX](https://max.ru) messenger and [Telegram](https://telegram.org). Connects to MAX via WebSocket on behalf of your account and forwards messages to specified Telegram chats or topics in real time.

## Features

- **Message forwarding** — text, photos, audio, voice messages, videos, files. Each message is displayed with the original sender's name.
- **Edit sync** — when a message is edited in MAX, the corresponding Telegram message updates automatically.
- **Deletion handling** — deleted messages are either removed from Telegram or marked with a `[Deleted ...]` label (configurable).
- **System events** — joining a chat, adding and removing members, chat creation.
- **History sync** — on startup and after reconnection, missed messages are fetched and forwarded.
- **Routing** — each MAX chat can be directed to a separate Telegram chat or topic.
- **Auto-reconnect** — on connection loss, the bot reconnects with exponential backoff.
- **Debug notifications** — optionally sends personal Telegram messages on disconnect and reconnect.
- **Logging** — each run writes a separate log file.

## Requirements

- A MAX account
- A Telegram bot token (get one from [@BotFather](https://t.me/BotFather))
- Go 1.21+ (only needed when building from source; not required for pre-built binaries)
- Very low resource usage: typically under 48 MB RAM and under 3% of a single CPU core

## Installation

### Option 1: Download a pre-built binary (recommended)

Go to the [releases page](https://github.com/mochensky/max2tg/releases/latest) and download the binary for your OS:

- `max2tg-linux-amd64` — Linux (x86-64)
- `max2tg-linux-arm64` — Linux (ARM64)
- `max2tg-windows-amd64.exe` — Windows

### Option 2: Build from source

```bash
git clone https://github.com/mochensky/max2tg
cd max2tg
go build -o max2tg .
```

On Windows via PowerShell:

```powershell
.\build.ps1
```

## Getting MAX credentials

You need two values: a **token** and a **device ID**. Both are stored in your browser's local storage after logging in.

1. Open [web.max.ru](https://web.max.ru) and sign in to your account.
2. Press `F12` (or `Ctrl+Shift+I`) to open DevTools.
3. Go to the **Application** tab (Chrome/Edge) or **Storage** tab (Firefox).
4. Open **Local Storage** → `https://web.max.ru`.
5. Find the key `__oneme_auth` — inside it, find the `token` field. This is your `MAX_TOKEN`.
6. Find the key `__oneme_device_id`. This is your `MAX_DEVICE_ID`.

> **Important:** the token is tied to your browser session. If you log out of MAX in the browser, the token becomes invalid and you will need to get a new one.

## Setup

### Step 1: First run

On first run, the program creates the configuration files and exits.

**Linux:**

```bash
chmod +x max2tg-linux-amd64
./max2tg-linux-amd64
```

**Windows:** run from the command line rather than double-clicking — otherwise the window closes immediately and you won't see any output:

```cmd
cd C:\path\to\max2tg
max2tg-windows-amd64.exe
```

After the first run you will see:

```
Created default config at data/config.yml
Created .env file at data/.env, please fill in your credentials
```

### Step 2: Fill in `data/.env`

Open `data/.env` in any text editor and fill in the values:

```env
MAX_TOKEN=your_token_from_local_storage
MAX_DEVICE_ID=your_device_id_from_local_storage
TG_TOKEN=your_telegram_bot_token
TG_DEBUG_USER_ID=your_telegram_user_id
```

`TG_DEBUG_USER_ID` is optional. If set, the bot will send you personal messages on disconnect and reconnect. You can find your ID via [@userinfobot](https://t.me/userinfobot).

### Step 3: Create a Telegram bot

1. Open [@BotFather](https://t.me/BotFather) in Telegram.
2. Send `/newbot` and follow the instructions.
3. Copy the token into the `TG_TOKEN` field in your `.env` file.
4. Add the bot to the target Telegram group or channel.
5. Grant the bot administrator rights.

### Step 4: Configure routes in `data/config.yml`

The `chats` section maps each MAX chat to a Telegram destination. At least one route is required.

```yaml
chats:
  # Route MAX chat 123456789 to a Telegram group/channel (no topic)
  - max_chat_id: 123456789
    telegram_chat_id: -1001234567890
    telegram_topic_id: 0

  # Route MAX chat 987654321 to topic #5 of a group
  - max_chat_id: 987654321
    telegram_chat_id: -1009876543210
    telegram_topic_id: 5
```

**How to find `max_chat_id`:** open the chat on [web.max.ru](https://web.max.ru) — the ID will be in the URL.

**How to find `telegram_chat_id`:** add [@getidsbot](https://t.me/getidsbot) to the group, or use a third-party Telegram client. Group and channel IDs are negative numbers like `-1001234567890`.

### Step 5: Run

**Linux:**

```bash
./max2tg-linux-amd64
```

**Windows:**

```cmd
cd C:\path\to\max2tg
max2tg-windows-amd64.exe
```

On a successful start you will see:

```
[16.04.2026 12:00:00] Starting max2tg 1.2.0...
[16.04.2026 12:00:00] Application is up to date (1.2.0)
[16.04.2026 12:00:01] Connected to WebSocket
[16.04.2026 12:00:01] Connected as Ivan (ID: 12345678)
```

Press `Ctrl+C` in the terminal to stop.

## Deploying on a VPS / server

Running max2tg on a Linux server keeps it online 24/7. Below is a minimal `systemd` setup.

### 1. Upload the binary to the server

```bash
scp max2tg-linux-amd64 user@your-server:/opt/max2tg/max2tg-linux-amd64
ssh user@your-server
chmod +x /opt/max2tg/max2tg-linux-amd64
```

### 2. Run once to generate the configuration files

```bash
cd /opt/max2tg
./max2tg-linux-amd64
```

Then fill in `/opt/max2tg/data/.env` and `/opt/max2tg/data/config.yml` as described above.

### 3. Create a systemd service

Create the file `/etc/systemd/system/max2tg.service`:

```ini
[Unit]
Description=max2tg
After=network.target

[Service]
Type=simple
User=your_linux_username
WorkingDirectory=/opt/max2tg
ExecStart=/opt/max2tg/max2tg-linux-amd64
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

### 4. Enable and start the service

```bash
sudo systemctl daemon-reload
sudo systemctl enable max2tg
sudo systemctl start max2tg
```

### Commands

```bash
# Check status
sudo systemctl status max2tg

# Stream logs in real time
sudo journalctl -u max2tg -f

# Restart after a config change
sudo systemctl restart max2tg

# Stop
sudo systemctl stop max2tg
```

Log files are also written to `data/logs/` — each run gets its own file.

## Running on Windows as a service

To keep the bot running after closing the terminal, register it as a Windows service via [NSSM](https://nssm.cc):

```cmd
nssm install max2tg C:\path\to\max2tg\max2tg-windows-amd64.exe
nssm start max2tg
```

## Configuration

All settings are stored in `data/config.yml`. Tokens (`MAX_TOKEN`, `MAX_DEVICE_ID`, `TG_TOKEN`, `TG_DEBUG_USER_ID`) go in `data/.env`.

```yaml
# Paths to data
env_path: "data/.env"
db_path: "data/database.db"
log_path: "data/logs"
download_path: "data/downloads"

# Timezone for timestamps in logs and messages (IANA format)
# Examples: Europe/Moscow, America/New_York, Europe/Berlin, UTC
timezone: "Europe/Moscow"

# How many recent messages to check during history sync
sync_history_depth: 30

# If true — deleted messages are marked [Deleted ...] instead of being removed
save_deleted: true

# If true — messages exceeding Telegram limits are truncated instead of skipped
# Caption limit: 1024 chars, message limit: 4096 chars
truncate_long_messages: true

# Retry settings for Telegram API requests
max_retries: 5
base_retry_delay: 1s

# Retry settings for downloading media from MAX
media_download_max_retries: 5
media_download_retry_delay: 1s

# How long to wait for a WebSocket ping response before reconnecting
ping_timeout: 1m30s
```

### Proxy configuration

Only **SOCKS5** proxies are supported. The `username` and `password` fields are optional — only fill them in if your proxy requires authentication. Each block covers one connection: set `max: true` for MAX or `telegram: true` for Telegram.

```yaml
proxy:
  # Proxy for MAX
  - max: false
    host: "proxy.example.com"
    port: 1080
    username: "proxyuser"   # optional
    password: "proxypass"   # optional

  # Proxy for Telegram
  - telegram: false
    host: "proxy.example.com"
    port: 1080
    username: "proxyuser"   # optional
    password: "proxypass"   # optional
```

To enable a proxy, change `false` to `true` in the relevant block.

The `user_agent`, `video_headers`, and `audio_headers` sections are pre-filled with working defaults and generally do not need to be changed.

## Project structure

```
├── src/
│   ├── client.go          — MAX client
│   ├── config.go          — config loading and validation
│   ├── connection.go      — WebSocket connection and reconnection
│   ├── database.go        — SQLite: message ID mapping storage
│   ├── enums.go           — constants and types
│   ├── errors.go          — error types
│   ├── logger.go          — logging
│   ├── models.go          — data structures
│   ├── parser.go          — WebSocket API response parsing
│   ├── proxy.go           — proxy support
│   ├── request_builder.go — WebSocket request builder
│   ├── safemap.go         — thread-safe maps
│   ├── sender.go          — Telegram API requests
│   ├── tzdata.go          — embedded timezone data
│   ├── utils.go           — media file downloading
│   └── version.go         — version checking
└── main.go                — entry point, event handlers
```