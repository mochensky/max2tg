package src

type RequestBuilder struct {
	config  *Config
	version int
	seq     int
}

func NewRequestBuilder(config *Config) *RequestBuilder {
	return &RequestBuilder{
		config:  config,
		version: 11,
		seq:     0,
	}
}

func (rb *RequestBuilder) buildBaseRequest(opcode Opcode, payload map[string]interface{}) WebSocketPayload {
	request := WebSocketPayload{
		"ver":     rb.version,
		"cmd":     0,
		"seq":     rb.seq,
		"opcode":  int(opcode),
		"payload": payload,
	}
	rb.incrementSeq()
	return request
}

func (rb *RequestBuilder) incrementSeq() {
	rb.seq++
}

func (rb *RequestBuilder) Init() WebSocketPayload {
	ua := rb.config.UserAgent
	payload := map[string]interface{}{
		"userAgent": map[string]interface{}{
			"deviceType":      "WEB",
			"locale":          ua.Locale,
			"deviceLocale":    ua.DeviceLocale,
			"osVersion":       ua.OSVersion,
			"deviceName":      ua.DeviceName,
			"headerUserAgent": ua.UserAgent,
			"appVersion":      ua.AppVersion,
			"screen":          ua.Screen,
			"timezone":        ua.Timezone,
		},
		"deviceId": rb.config.DeviceID,
	}
	return rb.buildBaseRequest(INIT, payload)
}

func (rb *RequestBuilder) Ping() WebSocketPayload {
	payload := map[string]interface{}{
		"interactive": true,
	}
	return rb.buildBaseRequest(PING, payload)
}

func (rb *RequestBuilder) Authenticate(chatsCount int) WebSocketPayload {
	payload := map[string]interface{}{
		"interactive":  true,
		"token":        rb.config.Token,
		"chatsCount":   chatsCount,
		"chatsSync":    0,
		"contactsSync": 0,
		"presenceSync": 0,
		"draftsSync":   0,
	}
	return rb.buildBaseRequest(AUTH, payload)
}

func (rb *RequestBuilder) GetContacts(contactIDs []int) WebSocketPayload {
	payload := map[string]interface{}{
		"contactIds": contactIDs,
	}
	return rb.buildBaseRequest(GET_CONTACTS, payload)
}

func (rb *RequestBuilder) GetChatMessages(chatID int, fromTime int64, forward, backward int) WebSocketPayload {
	payload := map[string]interface{}{
		"chatId":      chatID,
		"from":        fromTime,
		"forward":     forward,
		"backward":    backward,
		"getMessages": true,
	}
	return rb.buildBaseRequest(GET_MESSAGES, payload)
}

func (rb *RequestBuilder) GetFolders() WebSocketPayload {
	payload := map[string]interface{}{
		"folderSync": 0,
	}
	return rb.buildBaseRequest(GET_FOLDERS, payload)
}

func (rb *RequestBuilder) GetVideoLink(videoID, chatID, messageID int) WebSocketPayload {
	payload := map[string]interface{}{
		"videoId":   videoID,
		"chatId":    chatID,
		"messageId": messageID,
	}
	return rb.buildBaseRequest(GET_VIDEO, payload)
}

func (rb *RequestBuilder) GetFileLink(fileID, chatID, messageID int) WebSocketPayload {
	payload := map[string]interface{}{
		"fileId":    fileID,
		"chatId":    chatID,
		"messageId": messageID,
	}
	return rb.buildBaseRequest(GET_FILE, payload)
}

func (rb *RequestBuilder) SubscribeToChat(chatID int, subscribe bool) WebSocketPayload {
	payload := map[string]interface{}{
		"chatId":    chatID,
		"subscribe": subscribe,
	}
	return rb.buildBaseRequest(SUBSCRIBE_CHAT, payload)
}
