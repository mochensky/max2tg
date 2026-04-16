package src

const (
	WEBSOCKET_URL = "wss://ws-api.oneme.ru/websocket"
)

type WebSocketPayload map[string]interface{}

type Opcode int32

const (
	PING           Opcode = 1
	INIT           Opcode = 6
	AUTH           Opcode = 19
	GET_CONTACTS   Opcode = 32
	GET_MESSAGES   Opcode = 49
	SUBSCRIBE_CHAT Opcode = 75
	GET_VIDEO      Opcode = 83
	GET_FILE       Opcode = 88
	ON_MESSAGE     Opcode = 128
	GET_FOLDERS    Opcode = 272
)

type MessageStatus int32

const (
	MessageStatusNORMAL  MessageStatus = 0
	MessageStatusEDITED  MessageStatus = 1
	MessageStatusREMOVED MessageStatus = 2
)

type AttachmentType string

const (
	AttachmentTypeControl AttachmentType = "control"
	AttachmentTypeAudio   AttachmentType = "audio"
	AttachmentTypeFile    AttachmentType = "file"
	AttachmentTypePhoto   AttachmentType = "photo"
	AttachmentTypeVideo   AttachmentType = "video"
)

type ChatType string

const (
	ChatTypeChat    ChatType = "chat"
	ChatTypeChannel ChatType = "channel"
	ChatTypeGroup   ChatType = "group"
)

type MessageType string

const (
	MessageTypeText    MessageType = "text"
	MessageTypeVoice   MessageType = "voice"
	MessageTypeSticker MessageType = "sticker"
	MessageTypeGif     MessageType = "gif"
)
