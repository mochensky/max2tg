package src

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

const (
	AppName    = "max2tg"
	AppVersion = "1.0.9"

	DefaultEnvPath      = "data/.env"
	DefaultDBPath       = "data/database.db"
	DefaultLogPath      = "data/logs"
	DefaultDownloadPath = "data/downloads"

	DefaultLogTimezone          = "Europe/Moscow"
	DefaultSyncHistoryDepth     = 30
	DefaultSaveDeleted          = true
	DefaultTruncateLongMessages = true

	DefaultMaxRetries     = 5
	DefaultBaseRetryDelay = 1 * time.Second

	DefaultMediaDownloadMaxRetries = 5
	DefaultMediaDownloadRetryDelay = 1 * time.Second

	DefaultPingTimeout = 90 * time.Second

	DefaultUserAgent    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 YaBrowser/26.3.3.886 Safari/537.36"
	DefaultLocale       = "ru"
	DefaultDeviceLocale = "ru"
	DefaultOSVersion    = "Windows"
	DefaultDeviceName   = "YandexBrowser"
	DefaultUAAppVersion = "26.4.7"
	DefaultScreen       = "1920x1080 1.0x"
	DefaultUATimezone   = "Europe/Moscow"

	DefaultVideoHeaders = `Accept: video/webm,video/ogg,video/*;q=0.9,application/ogg;q=0.7,audio/*;q=0.6,*/*;q=0.5
Accept-Language: ru-RU,ru;q=0.5
Range: bytes=0-
Connection: keep-alive
Referer: https://web.max.ru/
Cookie: tstc=p
Sec-Fetch-Dest: video
Sec-Fetch-Mode: no-cors
Sec-Fetch-Site: cross-site
DNT: 1
Sec-GPC: 1
Accept-Encoding: identity
Priority: u=4
Pragma: no-cache
Cache-Control: no-cache`

	DefaultAudioHeaders = `Accept: audio/webm,audio/ogg,audio/wav,audio/*;q=0.9,application/ogg;q=0.7,video/*;q=0.6,*/*;q=0.5
Accept-Language: ru-RU,ru;q=0.9
Range: bytes=0-
Sec-Fetch-Storage-Access: none
Connection: keep-alive
Referer: https://web.max.ru/
Sec-Fetch-Dest: audio
Sec-Fetch-Mode: no-cors
Sec-Fetch-Site: cross-site
DNT: 1
Sec-GPC: 1
Accept-Encoding: identity
Priority: u=4
Pragma: no-cache
Cache-Control: no-cache`
)

var DefaultConfig = &Config{
	Token:    "",
	DeviceID: "",

	EnvPath:                 DefaultEnvPath,
	DBPath:                  DefaultDBPath,
	LogPath:                 DefaultLogPath,
	DownloadPath:            DefaultDownloadPath,
	LogTimezone:             DefaultLogTimezone,
	SyncHistoryDepth:        DefaultSyncHistoryDepth,
	SaveDeleted:             DefaultSaveDeleted,
	TruncateLongMessages:    DefaultTruncateLongMessages,
	MaxRetries:              DefaultMaxRetries,
	BaseRetryDelay:          DefaultBaseRetryDelay,
	MediaDownloadMaxRetries: DefaultMediaDownloadMaxRetries,
	MediaDownloadRetryDelay: DefaultMediaDownloadRetryDelay,
	PingTimeout:             DefaultPingTimeout,
	VideoHeaders:            DefaultVideoHeaders,
	AudioHeaders:            DefaultAudioHeaders,
	UserAgent: &UserAgentConfig{
		UserAgent:    DefaultUserAgent,
		Locale:       DefaultLocale,
		DeviceLocale: DefaultDeviceLocale,
		OSVersion:    DefaultOSVersion,
		DeviceName:   DefaultDeviceName,
		AppVersion:   DefaultUAAppVersion,
		Screen:       DefaultScreen,
		Timezone:     DefaultUATimezone,
	},
}

func LoadConfig(configPath string) (*Config, error) {
	cfg := &Config{}

	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	} else if os.IsNotExist(err) {
		if err := CreateDefaultConfig(configPath); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read newly created config: %w", err)
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse newly created config: %w", err)
		}
		fmt.Printf("Created default config at %s\n", configPath)
	} else {
		return nil, fmt.Errorf("error checking config file: %w", err)
	}

	envPath := cfg.EnvPath
	if envPath == "" {
		envPath = DefaultEnvPath
	}
	if _, err := os.Stat(envPath); err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(envPath), 0755); err != nil {
				return nil, fmt.Errorf("failed to create directory for .env file: %w", err)
			}
			envContent := `MAX_TOKEN=your_max_token_here
MAX_DEVICE_ID=your_device_id_here
TG_TOKEN=your_telegram_bot_token_here
TG_DEBUG_USER_ID=your_telegram_user_id_for_debug_messages
`
			if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
				return nil, fmt.Errorf("failed to create .env file: %w", err)
			}
			return nil, fmt.Errorf("created .env file at %s, please fill in your credentials", envPath)
		}
		return nil, fmt.Errorf("error checking .env file: %w", err)
	}

	if err := godotenv.Load(envPath); err != nil {
		fmt.Printf("Warning: .env file found but could not be loaded: %v\n", err)
	}

	if token := os.Getenv("MAX_TOKEN"); token != "" {
		cfg.Token = token
	}
	if deviceID := os.Getenv("MAX_DEVICE_ID"); deviceID != "" {
		cfg.DeviceID = deviceID
	}
	if tgToken := os.Getenv("TG_TOKEN"); tgToken != "" {
		cfg.TGToken = tgToken
	}
	if tgDebugUserID := os.Getenv("TG_DEBUG_USER_ID"); tgDebugUserID != "" {
		fmt.Sscanf(tgDebugUserID, "%d", &cfg.TGDebugUserID)
	}

	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	if cfg.MediaDownloadMaxRetries == 0 {
		cfg.MediaDownloadMaxRetries = DefaultMediaDownloadMaxRetries
	}
	if cfg.MediaDownloadRetryDelay == 0 {
		cfg.MediaDownloadRetryDelay = DefaultMediaDownloadRetryDelay
	}

	if err := ensureDirs(cfg); err != nil {
		return nil, fmt.Errorf("failed to create directories: %w", err)
	}

	return cfg, nil
}

func validateConfig(cfg *Config) error {
	required := []struct {
		field, value, name string
	}{
		{"MAX_TOKEN", cfg.Token, "MAX_TOKEN"},
		{"MAX_DEVICE_ID", cfg.DeviceID, "MAX_DEVICE_ID"},
		{"TG_TOKEN", cfg.TGToken, "TG_TOKEN"},
	}

	for _, req := range required {
		if req.value == "" {
			return fmt.Errorf("missing required configuration: %s", req.name)
		}
	}

	if len(cfg.ChatRoutes) == 0 {
		return fmt.Errorf("no chat routes configured: at least one route must be defined in config.yml 'chats' section")
	}

	for i, route := range cfg.ChatRoutes {
		if route.TelegramChatID == 0 {
			return fmt.Errorf("invalid route at index %d: telegram_chat_id cannot be zero", i)
		}
	}

	return nil
}

func ensureDirs(cfg *Config) error {
	dirs := []string{
		cfg.LogPath,
		filepath.Dir(cfg.DBPath),
		cfg.DownloadPath,
		filepath.Join(cfg.DownloadPath, "audio"),
		filepath.Join(cfg.DownloadPath, "files"),
		filepath.Join(cfg.DownloadPath, "images"),
		filepath.Join(cfg.DownloadPath, "videos"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

func CreateDefaultConfig(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	yamlContent := fmt.Sprintf(`# %s %s
# GitHub: https://github.com/mochensky/max2tg

# EN: paths to data
# RU: пути к данным
env_path: "%s"
db_path: "%s"
log_path: "%s"
download_path: "%s"

# EN: timezone for log timestamps (IANA format, e.g. Europe/Moscow, America/New_York, UTC)
# RU: часовой пояс для времени в логах (формат IANA, например, Europe/Moscow, America/New_York, UTC)
log_timezone: "%s"

# EN: how many recent chat messages will be checked for chat sync?
# RU: сколько последних сообщений в чате будет проверено на синхронизацию?
sync_history_depth: %d

# EN: will deleted messages from MAX be saved in Telegram with a special mark?
# RU: будут ли удаленные сообщения из MAX сохранены в Telegram с особым маркером?
save_deleted: %t

# EN: truncate long messages instead of skipping them (caption limit: 1024 chars, message limit: 4096 chars)
# RU: обрезать длинные сообщения вместо их пропуска (предел заголовка: 1024 символа, предел сообщения: 4096 символов)
truncate_long_messages: %t

# EN: configuration of retries for sending Telegram messages
# RU: конфигурация повторных попыток отправлять Telegram сообщения
max_retries: %d
base_retry_delay: %s

# EN: configuration of retries for downloading media from MAX (photos, videos, files, audio)
# RU: конфигурация повторных попыток скачивания медиа из MAX (фото, видео, файлы, аудио)
media_download_max_retries: %d
media_download_retry_delay: %s

# EN: WebSocket ping timeout
# RU: таймаут для ping WebSocket
ping_timeout: %s

# EN: chat routing configuration
# RU: конфигурация маршрутизации чатов
chats:
  # EN: e.g. 1: route MAX chat 0 to Telegram group / channel without topic
  # RU: например, 1: маршрутизировать чат MAX с id 0 в Telegram группу / канал без топика
  # - max_chat_id: 0
  #   telegram_chat_id: -1001234567890
  #   telegram_topic_id: 0

  # EN: e.g. 2: route MAX chat 123456789 to Telegram group -1001234567890 topic id 1
  # RU: например, 2: маршрутизировать чат MAX с id 123456789 в Telegram группу -1001234567890 в топик id 1
  # - max_chat_id: 123456789
  #   telegram_chat_id: -1001234567890
  #   telegram_topic_id: 1

  # EN: e.g. 3: route another MAX chat to different Telegram group / channel
  # RU: например, 3: маршрутизировать другой чат MAX в другую Telegram группу / канал
  # - max_chat_id: 987654321
  #   telegram_chat_id: -1009876543210
  #   telegram_topic_id: 0

# EN: proxy configuration (optional, can be used to access Telegram if it's blocked in your region)
# only SOCKS5 proxies are supported
# username and password fields are optional and only needed if your proxy requires authentication
# RU: конфигурация прокси (опционально, может быть использована для доступа к Telegram, если он заблокирован в вашем регионе)
# поддерживаются только прокси SOCKS5
# поля username и password являются необязательными и нужны только если ваш прокси требует аутентификацию
proxy:
  # EN: proxy for MAX
  # RU: прокси для MAX
  - max: false
    host: "proxy.example.com"
    port: 1080
    username: "proxyuser"
    password: "proxypass"
  # EN: proxy for Telegram
  # RU: прокси для Telegram
  - telegram: false
    host: "proxy.example.com"
    port: 1080
    username: "proxyuser"
    password: "proxypass"

user_agent:
  user_agent: "%s"
  locale: "%s"
  device_locale: "%s"
  os_version: "%s"
  device_name: "%s"
  app_version: "%s"
  screen: "%s"
  timezone: "%s"

video_headers: |
  Accept: video/webm,video/ogg,video/*;q=0.9,application/ogg;q=0.7,audio/*;q=0.6,*/*;q=0.5
  Accept-Language: ru-RU,ru;q=0.5
  Range: bytes=0-
  Connection: keep-alive
  Referer: https://web.max.ru/
  Cookie: tstc=p
  Sec-Fetch-Dest: video
  Sec-Fetch-Mode: no-cors
  Sec-Fetch-Site: cross-site
  DNT: 1
  Sec-GPC: 1
  Accept-Encoding: identity
  Priority: u=4
  Pragma: no-cache
  Cache-Control: no-cache

audio_headers: |
  Accept: audio/webm,audio/ogg,audio/wav,audio/*;q=0.9,application/ogg;q=0.7,video/*;q=0.6,*/*;q=0.5
  Accept-Language: ru-RU,ru;q=0.9
  Range: bytes=0-
  Sec-Fetch-Storage-Access: none
  Connection: keep-alive
  Referer: https://web.max.ru/
  Sec-Fetch-Dest: audio
  Sec-Fetch-Mode: no-cors
  Sec-Fetch-Site: cross-site
  DNT: 1
  Sec-GPC: 1
  Accept-Encoding: identity
  Priority: u=4
  Pragma: no-cache
  Cache-Control: no-cache
`,
		AppName, AppVersion,
		DefaultEnvPath, DefaultDBPath, DefaultLogPath, DefaultDownloadPath,
		DefaultLogTimezone,
		DefaultSyncHistoryDepth,
		DefaultSaveDeleted,
		DefaultTruncateLongMessages,
		DefaultMaxRetries, DefaultBaseRetryDelay,
		DefaultMediaDownloadMaxRetries, DefaultMediaDownloadRetryDelay,
		DefaultPingTimeout,
		DefaultUserAgent,
		DefaultLocale,
		DefaultDeviceLocale,
		DefaultOSVersion,
		DefaultDeviceName,
		DefaultUAAppVersion,
		DefaultScreen,
		DefaultUATimezone,
	)

	return os.WriteFile(path, []byte(yamlContent), 0644)
}
