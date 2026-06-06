# max2tg

Мост между мессенджером [MAX](https://max.ru) и [Telegram](https://telegram.org). Подключается к MAX через WebSocket от имени вашего аккаунта и пересылает сообщения в указанные Telegram-чаты или топики в реальном времени.

## Возможности

- **Пересылка сообщений** — текст, фото, аудио, голосовые, видео, файлы. Каждое сообщение отображается с именем исходного отправителя.
- **Синхронизация правок** — при редактировании сообщения в MAX соответствующее сообщение в Telegram обновляется автоматически.
- **Обработка удалений** — удалённые сообщения либо удаляются из Telegram, либо помечаются маркером `[Удалено ...]` (настраивается).
- **Системные события** — вступление в чат, добавление и удаление участников, создание чата.
- **Синхронизация истории** — при запуске и после переподключения пропущенные сообщения подгружаются и пересылаются.
- **Маршрутизация** — каждый MAX-чат можно направить в отдельный Telegram-чат или топик.
- **Автопереподключение** — при обрыве соединения бот переподключается с экспоненциальной задержкой.
- **Debug-уведомления** — опционально присылает личные сообщения в Telegram при отключении и переподключении.
- **Логирование** — каждый запуск пишет отдельный лог-файл.

## Требования

- Аккаунт в MAX
- Токен Telegram-бота (получить у [@BotFather](https://t.me/BotFather))
- Go 1.21+ (только при сборке из исходников; для готовых бинарников не нужен)
- Минимальные системные требования: обычно менее 48 МБ ОЗУ и менее 3% загрузки одного ядра CPU

## Установка

### Вариант 1: Скачать готовую сборку (рекомендуется)

Перейдите на [страницу релизов](https://github.com/mochensky/max2tg/releases/latest) и скачайте бинарник для вашей ОС:

- `max2tg-linux-amd64` — Linux (x86-64)
- `max2tg-linux-arm64` — Linux (ARM64)
- `max2tg-windows-amd64.exe` — Windows

### Вариант 2: Собрать из исходников

```bash
git clone https://github.com/mochensky/max2tg
cd max2tg
go build -o max2tg .
```

На Windows через PowerShell:

```powershell
.\build.ps1
```

## Получение учётных данных MAX

Для работы нужны два значения: **токен** и **device ID**. Оба хранятся в локальном хранилище браузера после авторизации.

1. Откройте [web.max.ru](https://web.max.ru) и войдите в аккаунт.
2. Нажмите `F12` (или `Ctrl+Shift+I`), чтобы открыть DevTools.
3. Перейдите на вкладку **Application** (Chrome/Edge) или **Storage** (Firefox).
4. Откройте **Local Storage** → `https://web.max.ru`.
5. Найдите ключ `__oneme_auth` — внутри него найдите поле `token`. Это ваш `MAX_TOKEN`.
6. Найдите ключ `__oneme_device_id`. Это ваш `MAX_DEVICE_ID`.

> **Важно:** токен привязан к сессии браузера. Если вы выйдете из MAX в браузере, токен станет недействительным — нужно будет получить новый.

## Настройка

### Шаг 1: Первый запуск

При первом запуске программа создаёт файлы конфигурации и завершается.

**Linux:**

```bash
chmod +x max2tg-linux-amd64
./max2tg-linux-amd64
```

**Windows:** запускайте через командную строку, а не двойным кликом — иначе окно закроется сразу после завершения и вы не увидите сообщений:

```cmd
cd C:\путь\до\папки\с\max2tg
max2tg-windows-amd64.exe
```

После первого запуска появятся сообщения:

```
Created default config at data/config.yml
Created .env file at data/.env, please fill in your credentials
```

### Шаг 2: Заполните `data/.env`

Откройте `data/.env` в любом текстовом редакторе и заполните значения:

```env
MAX_TOKEN=ваш_токен_из_local_storage
MAX_DEVICE_ID=ваш_device_id_из_local_storage
TG_TOKEN=токен_вашего_telegram_бота
TG_DEBUG_USER_ID=ваш_telegram_user_id
```

`TG_DEBUG_USER_ID` — необязательное поле. Если указан, бот будет присылать вам личные сообщения при отключении и переподключении. Узнать свой ID можно через [@userinfobot](https://t.me/userinfobot).

### Шаг 3: Создайте Telegram-бота

1. Откройте [@BotFather](https://t.me/BotFather) в Telegram.
2. Отправьте `/newbot` и следуйте инструкциям.
3. Скопируйте полученный токен в поле `TG_TOKEN` в файле `.env`.
4. Добавьте бота в целевую Telegram-группу или канал.
5. Выдайте боту права администратора.

### Шаг 4: Настройте маршруты в `data/config.yml`

Секция `chats` связывает каждый MAX-чат с Telegram-назначением. Нужен хотя бы один маршрут.

```yaml
chats:
  # Маршрутизировать MAX-чат 123456789 в Telegram-группу/канал (без топика)
  - max_chat_id: 123456789
    telegram_chat_id: -1001234567890
    telegram_topic_id: 0

  # Маршрутизировать MAX-чат 987654321 в топик №5 группы
  - max_chat_id: 987654321
    telegram_chat_id: -1009876543210
    telegram_topic_id: 5
```

**Как узнать `max_chat_id`:** откройте нужный чат на [web.max.ru](https://web.max.ru) — ID будет в URL.

**Как узнать `telegram_chat_id`:** добавьте [@getidsbot](https://t.me/getidsbot) в группу или используйте сторонний Telegram-клиент. ID групп и каналов — отрицательные числа вида `-1001234567890`.

### Шаг 5: Запуск

**Linux:**

```bash
./max2tg-linux-amd64
```

**Windows:**

```cmd
cd C:\путь\до\папки\с\max2tg
max2tg-windows-amd64.exe
```

При успешном старте вы увидите:

```
[16.04.2026 12:00:00] Starting max2tg 1.2.0...
[16.04.2026 12:00:00] Application is up to date (1.2.0)
[16.04.2026 12:00:01] Connected to WebSocket
[16.04.2026 12:00:01] Connected as Иван (ID: 12345678)
```

Для остановки нажмите `Ctrl+C` в терминале.

## Развёртывание на VPS / сервере

Запуск max2tg на Linux-сервере обеспечивает работу 24/7. Ниже приведена минимальная настройка через `systemd`.

### 1. Загрузите бинарник на сервер

```bash
scp max2tg-linux-amd64 user@ваш-сервер:/opt/max2tg/max2tg-linux-amd64
ssh user@ваш-сервер
chmod +x /opt/max2tg/max2tg-linux-amd64
```

### 2. Выполните первый запуск для генерации файлов конфигурации

```bash
cd /opt/max2tg
./max2tg-linux-amd64
```

Затем заполните `/opt/max2tg/data/.env` и `/opt/max2tg/data/config.yml` по инструкции выше.

### 3. Создайте systemd-сервис

Создайте файл `/etc/systemd/system/max2tg.service`:

```ini
[Unit]
Description=max2tg
After=network.target

[Service]
Type=simple
User=имя_вашего_linux_пользователя
WorkingDirectory=/opt/max2tg
ExecStart=/opt/max2tg/max2tg-linux-amd64
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

### 4. Включите и запустите сервис

```bash
sudo systemctl daemon-reload
sudo systemctl enable max2tg
sudo systemctl start max2tg
```

### Команды

```bash
# Проверить статус
sudo systemctl status max2tg

# Смотреть логи в реальном времени
sudo journalctl -u max2tg -f

# Перезапустить после изменения конфига
sudo systemctl restart max2tg

# Остановить
sudo systemctl stop max2tg
```

Лог-файлы также записываются в `data/logs/` — каждый запуск получает отдельный файл.

## Запуск на Windows как служба

Чтобы бот продолжал работать после закрытия терминала, зарегистрируйте его как Windows-службу через [NSSM](https://nssm.cc):

```cmd
nssm install max2tg C:\путь\до\папки\с\max2tg\max2tg-windows-amd64.exe
nssm start max2tg
```

## Конфигурация

Все настройки хранятся в `data/config.yml`. Токены (`MAX_TOKEN`, `MAX_DEVICE_ID`, `TG_TOKEN`, `TG_DEBUG_USER_ID`) указываются в `data/.env`.

```yaml
# Пути к данным
env_path: "data/.env"
db_path: "data/database.db"
log_path: "data/logs"
download_path: "data/downloads"

# Часовой пояс для временных меток в логах и сообщениях (формат IANA)
# Примеры: Europe/Moscow, America/New_York, Europe/Berlin, UTC
timezone: "Europe/Moscow"

# Сколько последних сообщений проверять при синхронизации истории
sync_history_depth: 30

# Если true — удалённые сообщения помечаются маркером [Удалено ...] вместо удаления
save_deleted: true

# Если true — сообщения, превышающие лимиты Telegram, обрезаются, а не пропускаются
# Лимит подписи: 1024 символа, лимит сообщения: 4096 символов
truncate_long_messages: true

# Настройки повторных попыток для запросов к Telegram API
max_retries: 5
base_retry_delay: 1s

# Настройки повторных попыток для скачивания медиа из MAX
media_download_max_retries: 5
media_download_retry_delay: 1s

# Сколько ждать ответ на WebSocket ping перед переподключением
ping_timeout: 1m30s
```

### Настройка прокси

Поддерживаются только **SOCKS5**-прокси. Поля `username` и `password` необязательны — указывайте их только если прокси требует аутентификацию. Каждый блок отвечает за одно подключение: укажите `max: true` для MAX или `telegram: true` для Telegram.

```yaml
proxy:
  # Прокси для подключения к MAX
  - max: false
    host: "proxy.example.com"
    port: 1080
    username: "proxyuser"   # необязательно
    password: "proxypass"   # необязательно

  # Прокси для подключения к Telegram
  - telegram: false
    host: "proxy.example.com"
    port: 1080
    username: "proxyuser"   # необязательно
    password: "proxypass"   # необязательно
```

Чтобы включить прокси, поменяйте `false` на `true` в нужном блоке.

Секции `user_agent`, `video_headers` и `audio_headers` заполнены рабочими значениями по умолчанию и обычно не требуют изменений.

## Структура проекта

```
├── src/
│   ├── client.go          — клиент MAX
│   ├── config.go          — загрузка и валидация конфига
│   ├── connection.go      — WebSocket-соединение и переподключение
│   ├── database.go        — SQLite: хранение маппинга ID сообщений
│   ├── enums.go           — константы и типы
│   ├── errors.go          — типы ошибок
│   ├── logger.go          — логирование
│   ├── models.go          — структуры данных
│   ├── parser.go          — парсинг ответов WebSocket API
│   ├── proxy.go           — прокси
│   ├── request_builder.go — построение WebSocket-запросов
│   ├── safemap.go         — потокобезопасные map
│   ├── sender.go          — запросы к Telegram API
│   ├── tzdata.go          — встроенные данные часовых поясов
│   ├── utils.go           — скачивание медиафайлов
│   └── version.go         — проверка версий
└── main.go                — точка входа, обработчики событий
```