# max2tg

A bridge that mirrors messages from [MAX](https://max.ru) to [Telegram](https://telegram.org) in real time.

## Documentation: 
 - ### [English](README/en/README.md)
 - ### [Русский](README/ru/README.md)

## Overview

max2tg connects to the MAX messenger WebSocket API using your account credentials and forwards incoming messages to one or more Telegram chats or topics. It handles text, photos, audio, video, files, forwarded messages, edits, and deletions.

## Quick start

You can download a ready-made build for your OS from the releases.

Or build the project manually:

```bash
git clone https://github.com/mochensky/max2tg
cd max2tg
go build -o max2tg .
./max2tg
```

On first run the program creates `data/config.yml` and `data/.env`. Fill in your credentials and re-run. See the full setup guide in the documentation linked above.

## License

MIT