package src

import (
	"time"
)

type Me struct {
	ID               int                    `json:"id"`
	FirstName        string                 `json:"first_name"`
	LastName         string                 `json:"last_name"`
	Name             string                 `json:"name"`
	NameType         string                 `json:"name_type"`
	Phone            int                    `json:"phone"`
	AccountStatus    int                    `json:"account_status"`
	UpdateTime       *int64                 `json:"update_time,omitempty"`
	Options          []string               `json:"options,omitempty"`
	ProfileOptions   []string               `json:"profile_options,omitempty"`
	VideoChatHistory bool                   `json:"video_chat_history,omitempty"`
	ChatMarker       int                    `json:"chat_marker,omitempty"`
	Time             *int64                 `json:"time,omitempty"`
	Presence         map[string]interface{} `json:"presence,omitempty"`
	Config           map[string]interface{} `json:"config,omitempty"`
}

type Contact struct {
	ID            int      `json:"id"`
	FirstName     string   `json:"first_name"`
	LastName      string   `json:"last_name"`
	AccountStatus int      `json:"account_status"`
	UpdateTime    *int64   `json:"update_time,omitempty"`
	Options       []string `json:"options,omitempty"`
	BaseURL       *string  `json:"base_url,omitempty"`
	BaseRawURL    *string  `json:"base_raw_url,omitempty"`
	PhotoID       *int     `json:"photo_id,omitempty"`
	Link          *string  `json:"link,omitempty"`
	Gender        *int     `json:"gender,omitempty"`
	Description   *string  `json:"description,omitempty"`
	WebApp        *string  `json:"web_app,omitempty"`
}

type Attachment struct {
	Type AttachmentType `json:"type"`

	PreviewData *string `json:"preview_data,omitempty"`
	BaseURL     string  `json:"base_url,omitempty"`
	PhotoToken  string  `json:"photo_token,omitempty"`
	PhotoID     int     `json:"photo_id,omitempty"`
	Width       *int    `json:"width,omitempty"`
	Height      *int    `json:"height,omitempty"`

	VideoID   int    `json:"video_id,omitempty"`
	VideoType int    `json:"video_type,omitempty"`
	Thumbnail string `json:"thumbnail,omitempty"`
	Duration  *int   `json:"duration,omitempty"`

	FileName string  `json:"file_name,omitempty"`
	FileSize int     `json:"file_size,omitempty"`
	FileID   int     `json:"file_id,omitempty"`
	Token    string  `json:"token,omitempty"`
	MimeType *string `json:"mime_type,omitempty"`

	AudioID             int    `json:"audio_id,omitempty"`
	AudioURL            string `json:"url,omitempty"`
	AudioToken          string `json:"audio_token,omitempty"`
	AudioDuration       *int   `json:"audio_duration,omitempty"`
	TranscriptionStatus string `json:"transcription_status,omitempty"`

	Event   string `json:"event,omitempty"`
	UserIDs []int  `json:"user_ids,omitempty"`
	UserID  *int   `json:"user_id,omitempty"`
}

type Channel struct {
	ID         int     `json:"id"`
	Name       string  `json:"name"`
	Link       *string `json:"link,omitempty"`
	AccessType *string `json:"access_type,omitempty"`
	IconURL    *string `json:"icon_url,omitempty"`
}

type ForwardedMessage struct {
	ID                int                      `json:"id"`
	Text              string                   `json:"text"`
	SenderID          *int                     `json:"sender_id,omitempty"`
	Channel           *Channel                 `json:"channel,omitempty"`
	FormattedHTMLText *string                  `json:"formatted_html_text,omitempty"`
	Time              *int64                   `json:"time,omitempty"`
	Type              *string                  `json:"type,omitempty"`
	Status            MessageStatus            `json:"status"`
	Attaches          []Attachment             `json:"attaches,omitempty"`
	Link              map[string]interface{}   `json:"link,omitempty"`
	Elements          []map[string]interface{} `json:"elements,omitempty"`
	CID               *int                     `json:"cid,omitempty"`
}

type Message struct {
	ID                int                      `json:"id"`
	ChatID            int                      `json:"chat_id"`
	SenderID          int                      `json:"sender_id"`
	Text              string                   `json:"text"`
	FormattedHTMLText *string                  `json:"formatted_html_text,omitempty"`
	Time              *int64                   `json:"time,omitempty"`
	UpdateTime        *int64                   `json:"update_time,omitempty"`
	Type              *string                  `json:"type,omitempty"`
	Status            MessageStatus            `json:"status"`
	Attaches          []Attachment             `json:"attaches,omitempty"`
	Link              map[string]interface{}   `json:"link,omitempty"`
	ReactionInfo      map[string]interface{}   `json:"reaction_info,omitempty"`
	Elements          []map[string]interface{} `json:"elements,omitempty"`
	CID               *int                     `json:"cid,omitempty"`
	ForwardedMessage  *ForwardedMessage        `json:"forwarded_message,omitempty"`
}

type Chat struct {
	ID           int         `json:"id"`
	Title        string      `json:"title"`
	Type         ChatType    `json:"type"`
	Status       string      `json:"status"`
	Participants map[int]int `json:"participants"`
	LastMessage  *Message    `json:"last_message,omitempty"`
	Created      *int64      `json:"created,omitempty"`
	Modified     *int64      `json:"modified,omitempty"`
	JoinTime     *int64      `json:"join_time,omitempty"`
	Owner        *int        `json:"owner,omitempty"`
}

type ProxyConfig struct {
	ForMax      bool   `yaml:"max"`
	ForTelegram bool   `yaml:"telegram"`
	Host        string `yaml:"host"`
	Port        int    `yaml:"port"`
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
}

type UserAgentConfig struct {
	UserAgent    string `yaml:"user_agent"`
	Locale       string `yaml:"locale"`
	DeviceLocale string `yaml:"device_locale"`
	OSVersion    string `yaml:"os_version"`
	DeviceName   string `yaml:"device_name"`
	AppVersion   string `yaml:"app_version"`
	Screen       string `yaml:"screen"`
	Timezone     string `yaml:"timezone"`
}

type ChatRoute struct {
	MaxChatID       int   `yaml:"max_chat_id"`
	TelegramChatID  int64 `yaml:"telegram_chat_id"`
	TelegramTopicID int   `yaml:"telegram_topic_id"`
}

type Config struct {
	Token    string `yaml:"token"`
	DeviceID string `yaml:"device_id"`

	TGToken       string `yaml:"tg_token"`
	TGDebugUserID int64  `yaml:"tg_debug_user_id"`

	EnvPath      string `yaml:"env_path"`
	DBPath       string `yaml:"db_path"`
	LogPath      string `yaml:"log_path"`
	DownloadPath string `yaml:"download_path"`

	LogTimezone          string `yaml:"log_timezone"`
	SyncHistoryDepth     int    `yaml:"sync_history_depth"`
	SaveDeleted          bool   `yaml:"save_deleted"`
	TruncateLongMessages bool   `yaml:"truncate_long_messages"`

	ChatRoutes []ChatRoute `yaml:"chats"`

	MaxRetries     int           `yaml:"max_retries"`
	BaseRetryDelay time.Duration `yaml:"base_retry_delay"`

	MediaDownloadMaxRetries int           `yaml:"media_download_max_retries"`
	MediaDownloadRetryDelay time.Duration `yaml:"media_download_retry_delay"`

	PingTimeout time.Duration `yaml:"ping_timeout"`

	Proxies      []ProxyConfig    `yaml:"proxy"`
	UserAgent    *UserAgentConfig `yaml:"user_agent"`
	VideoHeaders string           `yaml:"video_headers"`
	AudioHeaders string           `yaml:"audio_headers"`
}

func (c *Config) GetTimeZoneLocation() *time.Location {
	loc, _ := time.LoadLocation(c.UserAgent.Timezone)
	return loc
}
