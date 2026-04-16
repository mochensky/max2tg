package src

import (
	"html"
	"strconv"
	"strings"
	"time"
)

func ParseID(idValue interface{}) int {
	if idValue == nil {
		return 0
	}

	if idFloat, ok := idValue.(float64); ok {
		return int(idFloat)
	}

	if idStr, ok := idValue.(string); ok {
		if parsed, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			return int(parsed)
		}
	}

	if idInt, ok := idValue.(int); ok {
		return idInt
	}

	if idInt64, ok := idValue.(int64); ok {
		return int(idInt64)
	}

	return 0
}

func parseInt64(value interface{}) (int64, bool) {
	if value == nil {
		return 0, false
	}

	if f, ok := value.(float64); ok {
		return int64(f), true
	}

	if s, ok := value.(string); ok {
		if parsed, err := strconv.ParseInt(s, 10, 64); err == nil {
			return parsed, true
		}
	}

	if i, ok := value.(int64); ok {
		return i, true
	}

	if i, ok := value.(int); ok {
		return int64(i), true
	}

	return 0, false
}

func utf16IndexToPyIndex(text string, utf16Index int) int {
	if utf16Index <= 0 {
		return 0
	}

	codeUnits := 0
	byteOffset := 0
	for _, r := range text {
		units := 1
		if r > 0xFFFF {
			units = 2
		}
		if codeUnits+units > utf16Index {
			break
		}
		codeUnits += units
		switch {
		case r < 0x80:
			byteOffset += 1
		case r < 0x800:
			byteOffset += 2
		case r < 0x10000:
			byteOffset += 3
		default:
			byteOffset += 4
		}
		if codeUnits == utf16Index {
			break
		}
	}
	return byteOffset
}

func parseHTMLText(text string, elements []map[string]interface{}) string {
	if len(elements) == 0 {
		return html.EscapeString(text)
	}

	type Event struct {
		Pos  int
		Kind string
		Tag  string
	}

	var events []Event

	for _, elem := range elements {
		elemType, ok := elem["type"].(string)
		if !ok {
			continue
		}
		startUTF16 := 0
		if v, ok := elem["from"].(float64); ok {
			startUTF16 = int(v)
		}
		lengthUTF16 := 0
		if v, ok := elem["length"].(float64); ok {
			lengthUTF16 = int(v)
		}
		if lengthUTF16 <= 0 {
			continue
		}

		start := utf16IndexToPyIndex(text, startUTF16)
		end := utf16IndexToPyIndex(text, startUTF16+lengthUTF16)

		var tagOpen, tagClose string
		switch elemType {
		case "STRONG":
			tagOpen = "<b>"
			tagClose = "</b>"
		case "EMPHASIZED":
			tagOpen = "<i>"
			tagClose = "</i>"
		case "UNDERLINE":
			tagOpen = "<u>"
			tagClose = "</u>"
		case "STRIKETHROUGH":
			tagOpen = "<s>"
			tagClose = "</s>"
		case "QUOTE":
			tagOpen = "<blockquote>"
			tagClose = "</blockquote>"
		default:
			continue
		}

		events = append(events, Event{Pos: start, Kind: "open", Tag: tagOpen})
		events = append(events, Event{Pos: end, Kind: "close", Tag: tagClose})
	}

	if len(events) == 0 {
		return html.EscapeString(text)
	}

	for i := 0; i < len(events); i++ {
		for j := i + 1; j < len(events); j++ {
			if events[j].Pos < events[i].Pos ||
				(events[j].Pos == events[i].Pos && events[j].Kind == "close" && events[i].Kind == "open") {
				events[i], events[j] = events[j], events[i]
			}
		}
	}

	var result strings.Builder
	pos := 0
	stack := []string{}

	for _, event := range events {
		if event.Pos > pos {
			chunk := text[pos:event.Pos]
			result.WriteString(html.EscapeString(chunk))
			pos = event.Pos
		}

		if event.Kind == "open" {
			result.WriteString(event.Tag)
			tagName := strings.TrimPrefix(strings.TrimSuffix(event.Tag, ">"), "<")
			stack = append(stack, tagName)
		} else {
			tagName := strings.TrimPrefix(strings.TrimSuffix(event.Tag, ">"), "</")
			idx := -1
			for i := len(stack) - 1; i >= 0; i-- {
				if stack[i] == tagName {
					idx = i
					break
				}
			}
			if idx != -1 {
				for i := len(stack) - 1; i > idx; i-- {
					result.WriteString("</" + stack[i] + ">")
				}
				result.WriteString(event.Tag)
				stack = stack[:idx]
			}
		}
	}

	if pos < len(text) {
		result.WriteString(html.EscapeString(text[pos:]))
	}

	for i := len(stack) - 1; i >= 0; i-- {
		result.WriteString("</" + stack[i] + ">")
	}

	return result.String()
}

func parseAttachment(attachData map[string]interface{}) *Attachment {
	attachType, ok := attachData["_type"].(string)
	if !ok {
		return nil
	}

	normalizedType := strings.ToLower(attachType)
	attachment := Attachment{Type: AttachmentType(normalizedType)}

	switch normalizedType {
	case "control":
		if event, ok := attachData["event"].(string); ok {
			attachment.Event = event
		}
		if userIds, ok := attachData["userIds"].([]interface{}); ok {
			for _, uid := range userIds {
				parsedID := ParseID(uid)
				if parsedID != 0 {
					attachment.UserIDs = append(attachment.UserIDs, parsedID)
				}
			}
		}
		if userIdVal := attachData["userId"]; userIdVal != nil {
			uid := ParseID(userIdVal)
			if uid != 0 {
				attachment.UserID = &uid
			}
		}
	case "photo":
		if previewData, ok := attachData["previewData"].(string); ok {
			attachment.PreviewData = &previewData
		}
		if baseUrl, ok := attachData["baseUrl"].(string); ok {
			attachment.BaseURL = baseUrl
		}
		if photoToken, ok := attachData["photoToken"].(string); ok {
			attachment.PhotoToken = photoToken
		}
		attachment.PhotoID = ParseID(attachData["photoId"])
		if width, ok := attachData["width"].(float64); ok {
			w := int(width)
			attachment.Width = &w
		}
		if height, ok := attachData["height"].(float64); ok {
			h := int(height)
			attachment.Height = &h
		}
	case "video":
		attachment.VideoID = ParseID(attachData["videoId"])
		if token, ok := attachData["token"].(string); ok {
			attachment.Token = token
		}
		if videoType, ok := attachData["videoType"].(float64); ok {
			attachment.VideoType = int(videoType)
		}
		if thumbnail, ok := attachData["thumbnail"].(string); ok {
			attachment.Thumbnail = thumbnail
		}
		if previewData, ok := attachData["previewData"].(string); ok {
			attachment.PreviewData = &previewData
		}
		if duration, ok := attachData["duration"].(float64); ok {
			d := int(duration)
			attachment.Duration = &d
		}
		if width, ok := attachData["width"].(float64); ok {
			w := int(width)
			attachment.Width = &w
		}
		if height, ok := attachData["height"].(float64); ok {
			h := int(height)
			attachment.Height = &h
		}
	case "file":
		if fileName, ok := attachData["name"].(string); ok {
			attachment.FileName = fileName
		}
		if fileSize, ok := attachData["size"].(float64); ok {
			attachment.FileSize = int(fileSize)
		}
		attachment.FileID = ParseID(attachData["fileId"])
		if token, ok := attachData["token"].(string); ok {
			attachment.Token = token
		}
		if mimeType, ok := attachData["mimeType"].(string); ok {
			attachment.MimeType = &mimeType
		}
	case "audio":
		attachment.AudioID = ParseID(attachData["audioId"])
		if url, ok := attachData["url"].(string); ok {
			attachment.AudioURL = url
		}
		if token, ok := attachData["token"].(string); ok {
			attachment.AudioToken = token
		}
		if duration, ok := attachData["duration"].(float64); ok {
			d := int(duration)
			attachment.AudioDuration = &d
		}
		if transcriptionStatus, ok := attachData["transcriptionStatus"].(string); ok {
			attachment.TranscriptionStatus = transcriptionStatus
		}
	}

	return &attachment
}

func parseForwardedMessage(linkData map[string]interface{}) *ForwardedMessage {
	messageData, _ := linkData["message"].(map[string]interface{})
	if messageData == nil {
		return nil
	}

	forwarded := &ForwardedMessage{}

	forwarded.ID = ParseID(messageData["id"])
	if text, ok := messageData["text"].(string); ok {
		forwarded.Text = text
	}

	var elements []map[string]interface{}
	if elems, ok := messageData["elements"].([]interface{}); ok {
		elements = make([]map[string]interface{}, len(elems))
		for i, elem := range elems {
			if m, ok := elem.(map[string]interface{}); ok {
				elements[i] = m
			}
		}
	}
	formatted := parseHTMLText(forwarded.Text, elements)
	forwarded.FormattedHTMLText = &formatted

	if timeVal, ok := messageData["time"].(float64); ok {
		t := int64(timeVal)
		forwarded.Time = &t
	}
	if msgType, ok := messageData["type"].(string); ok {
		forwarded.Type = &msgType
	}
	if timeVal, ok := messageData["time"].(float64); ok {
		t := int64(timeVal)
		forwarded.Time = &t
	}
	if msgType, ok := messageData["type"].(string); ok {
		forwarded.Type = &msgType
	}
	if status, ok := messageData["status"].(string); ok {
		switch status {
		case "EDITED":
			forwarded.Status = MessageStatusEDITED
		case "REMOVED":
			forwarded.Status = MessageStatusREMOVED
		default:
			forwarded.Status = MessageStatusNORMAL
		}
	} else {
		forwarded.Status = MessageStatusNORMAL
	}

	if senderVal := messageData["sender"]; senderVal != nil {
		sid := ParseID(senderVal)
		forwarded.SenderID = &sid
	}

	if channelData, ok := messageData["channel"].(map[string]interface{}); ok {
		channel := &Channel{}
		channel.ID = ParseID(channelData["id"])
		if name, ok := channelData["name"].(string); ok {
			channel.Name = name
		}
		if link, ok := channelData["link"].(string); ok {
			channel.Link = &link
		}
		if accessType, ok := channelData["accessType"].(string); ok {
			channel.AccessType = &accessType
		}
		if iconUrl, ok := channelData["iconUrl"].(string); ok {
			channel.IconURL = &iconUrl
		}
		forwarded.Channel = channel
	}

	if elements, ok := messageData["elements"].([]interface{}); ok {
		forwarded.Elements = make([]map[string]interface{}, len(elements))
		for i, elem := range elements {
			if m, ok := elem.(map[string]interface{}); ok {
				forwarded.Elements[i] = m
			}
		}
	}

	if attaches, ok := messageData["attaches"].([]interface{}); ok {
		for _, attach := range attaches {
			if attachMap, ok := attach.(map[string]interface{}); ok {
				if parsed := parseAttachment(attachMap); parsed != nil {
					forwarded.Attaches = append(forwarded.Attaches, *parsed)
				}
			}
		}
	}

	if link, ok := messageData["link"].(map[string]interface{}); ok {
		forwarded.Link = link
	}

	if cidVal := messageData["cid"]; cidVal != nil {
		c := ParseID(cidVal)
		forwarded.CID = &c
	}

	return forwarded
}

func parseMessage(messageData map[string]interface{}, chatID int) Message {
	msg := Message{
		ChatID: chatID,
	}

	msg.ID = ParseID(messageData["id"])
	msg.SenderID = ParseID(messageData["sender"])
	if text, ok := messageData["text"].(string); ok {
		msg.Text = text
	}
	if timeVal, ok := messageData["time"].(float64); ok {
		t := int64(timeVal)
		msg.Time = &t
	}
	if updateTime, ok := messageData["updateTime"].(float64); ok {
		ut := int64(updateTime)
		msg.UpdateTime = &ut
	}
	if msgType, ok := messageData["type"].(string); ok {
		msg.Type = &msgType
	}
	if status, ok := messageData["status"].(string); ok {
		switch status {
		case "EDITED":
			msg.Status = MessageStatusEDITED
		case "REMOVED":
			msg.Status = MessageStatusREMOVED
		default:
			msg.Status = MessageStatusNORMAL
		}
	} else {
		msg.Status = MessageStatusNORMAL
	}

	if elements, ok := messageData["elements"].([]interface{}); ok {
		msg.Elements = make([]map[string]interface{}, len(elements))
		for i, elem := range elements {
			if m, ok := elem.(map[string]interface{}); ok {
				msg.Elements[i] = m
			}
		}
	}

	if attaches, ok := messageData["attaches"].([]interface{}); ok {
		for _, attach := range attaches {
			if attachMap, ok := attach.(map[string]interface{}); ok {
				if parsed := parseAttachment(attachMap); parsed != nil {
					msg.Attaches = append(msg.Attaches, *parsed)
				}
			}
		}
	}

	if link, ok := messageData["link"].(map[string]interface{}); ok {
		msg.Link = link
		if linkType, ok := link["type"].(string); ok && linkType == "FORWARD" {
			msg.ForwardedMessage = parseForwardedMessage(link)
		}
	}

	if reactionInfo, ok := messageData["reactionInfo"].(map[string]interface{}); ok {
		msg.ReactionInfo = reactionInfo
	}

	if cidVal := messageData["cid"]; cidVal != nil {
		c := ParseID(cidVal)
		msg.CID = &c
	}

	formatted := parseHTMLText(msg.Text, msg.Elements)
	msg.FormattedHTMLText = &formatted

	return msg
}

func parseProfile(payload map[string]interface{}) Me {
	me := Me{}

	if profileData, ok := payload["profile"].(map[string]interface{}); ok {
		if contact, ok := profileData["contact"].(map[string]interface{}); ok {
			me.ID = ParseID(contact["id"])
			me.Phone = ParseID(contact["phone"])
			me.AccountStatus = ParseID(contact["accountStatus"])
			if updateTimeVal := contact["updateTime"]; updateTimeVal != nil {
				if ut, ok := parseInt64(updateTimeVal); ok {
					me.UpdateTime = &ut
				}
			}
			if options, ok := contact["options"].([]interface{}); ok {
				me.Options = make([]string, len(options))
				for i, opt := range options {
					if optStr, ok := opt.(string); ok {
						me.Options[i] = optStr
					}
				}
			}
			if names, ok := contact["names"].([]interface{}); ok && len(names) > 0 {
				if nameMap, ok := names[0].(map[string]interface{}); ok {
					if firstName, ok := nameMap["firstName"].(string); ok {
						me.FirstName = firstName
					}
					if lastName, ok := nameMap["lastName"].(string); ok {
						me.LastName = lastName
					}
					if name, ok := nameMap["name"].(string); ok {
						me.Name = name
					}
					if nameType, ok := nameMap["type"].(string); ok {
						me.NameType = nameType
					}
				}
			}
		}
		if profileOptions, ok := profileData["profileOptions"].([]interface{}); ok {
			me.ProfileOptions = make([]string, len(profileOptions))
			for i, opt := range profileOptions {
				if optStr, ok := opt.(string); ok {
					me.ProfileOptions[i] = optStr
				}
			}
		}
	}

	if videoChatHistory, ok := payload["videoChatHistory"].(bool); ok {
		me.VideoChatHistory = videoChatHistory
	}
	if chatMarker, ok := payload["chatMarker"].(float64); ok {
		me.ChatMarker = int(chatMarker)
	}
	if timeVal, ok := payload["time"].(float64); ok {
		t := int64(timeVal)
		me.Time = &t
	}
	if presence, ok := payload["presence"].(map[string]interface{}); ok {
		me.Presence = presence
	}
	if config, ok := payload["config"].(map[string]interface{}); ok {
		me.Config = config
	}

	return me
}

func parseChat(chatData map[string]interface{}) Chat {
	chat := Chat{}

	chat.ID = ParseID(chatData["id"])
	if title, ok := chatData["title"].(string); ok {
		chat.Title = title
	}
	if chatType, ok := chatData["type"].(string); ok {
		chat.Type = ChatType(chatType)
	}
	if status, ok := chatData["status"].(string); ok {
		chat.Status = status
	}
	if participants, ok := chatData["participants"].(map[string]interface{}); ok {
		chat.Participants = make(map[int]int)
		for k, v := range participants {
			key := ParseID(k)
			val := ParseID(v)
			chat.Participants[key] = val
		}
	}
	if created, ok := chatData["created"].(float64); ok {
		c := int64(created)
		chat.Created = &c
	}
	if modified, ok := chatData["modified"].(float64); ok {
		m := int64(modified)
		chat.Modified = &m
	}
	if joinTime, ok := chatData["joinTime"].(float64); ok {
		j := int64(joinTime)
		chat.JoinTime = &j
	}
	if ownerVal := chatData["owner"]; ownerVal != nil {
		o := ParseID(ownerVal)
		chat.Owner = &o
	}

	if lastMessageData, ok := chatData["lastMessage"].(map[string]interface{}); ok {
		lastMsg := parseMessage(lastMessageData, chat.ID)
		chat.LastMessage = &lastMsg
	}

	return chat
}

func parseChats(chatsData []map[string]interface{}) []Chat {
	chats := make([]Chat, len(chatsData))
	for i, chatData := range chatsData {
		chats[i] = parseChat(chatData)
	}
	return chats
}

func parseContact(contactData map[string]interface{}) Contact {
	contact := Contact{}

	contact.ID = ParseID(contactData["id"])
	contact.AccountStatus = ParseID(contactData["accountStatus"])
	if updateTimeVal := contactData["updateTime"]; updateTimeVal != nil {
		if ut, ok := parseInt64(updateTimeVal); ok {
			contact.UpdateTime = &ut
		}
	}
	if options, ok := contactData["options"].([]interface{}); ok {
		contact.Options = make([]string, len(options))
		for i, opt := range options {
			if optStr, ok := opt.(string); ok {
				contact.Options[i] = optStr
			}
		}
	}
	if baseUrl, ok := contactData["baseUrl"].(string); ok {
		contact.BaseURL = &baseUrl
	}
	if baseRawUrl, ok := contactData["baseRawUrl"].(string); ok {
		contact.BaseRawURL = &baseRawUrl
	}
	if photoIdVal := contactData["photoId"]; photoIdVal != nil {
		pid := ParseID(photoIdVal)
		if pid != 0 {
			contact.PhotoID = &pid
		}
	}
	if link, ok := contactData["link"].(string); ok {
		contact.Link = &link
	}
	if genderVal := contactData["gender"]; genderVal != nil {
		g := ParseID(genderVal)
		if g != 0 {
			contact.Gender = &g
		}
	}
	if description, ok := contactData["description"].(string); ok {
		contact.Description = &description
	}
	if webApp, ok := contactData["webApp"].(string); ok {
		contact.WebApp = &webApp
	}

	if names, ok := contactData["names"].([]interface{}); ok && len(names) > 0 {
		if nameMap, ok := names[0].(map[string]interface{}); ok {
			if firstName, ok := nameMap["firstName"].(string); ok {
				contact.FirstName = firstName
			}
			if lastName, ok := nameMap["lastName"].(string); ok {
				contact.LastName = lastName
			}
		}
	}

	return contact
}

func parseContacts(contactsData []map[string]interface{}) []Contact {
	contacts := make([]Contact, len(contactsData))
	for i, contactData := range contactsData {
		contacts[i] = parseContact(contactData)
	}
	return contacts
}

func parseFolder(folderData map[string]interface{}) Folder {
	folder := Folder{}

	folder.ID = ParseID(folderData["id"])
	if title, ok := folderData["title"].(string); ok {
		folder.Title = title
	}
	if include, ok := folderData["include"].([]interface{}); ok {
		folder.Include = make([]int, len(include))
		for i, v := range include {
			folder.Include[i] = ParseID(v)
		}
	}
	if filters, ok := folderData["filters"].([]interface{}); ok {
		folder.Filters = make([]int, len(filters))
		for i, v := range filters {
			folder.Filters[i] = ParseID(v)
		}
	}
	if options, ok := folderData["options"].([]interface{}); ok {
		folder.Options = make([]string, len(options))
		for i, opt := range options {
			if optStr, ok := opt.(string); ok {
				folder.Options[i] = optStr
			}
		}
	}
	if updateTimeVal := folderData["updateTime"]; updateTimeVal != nil {
		if ut, ok := parseInt64(updateTimeVal); ok {
			folder.UpdateTime = ut
		}
	}
	folder.SourceID = ParseID(folderData["sourceId"])

	return folder
}

func parseFolders(foldersData []map[string]interface{}) []Folder {
	folders := make([]Folder, len(foldersData))
	for i, folderData := range foldersData {
		folders[i] = parseFolder(folderData)
	}
	return folders
}

func FormatTime(timestamp int64) string {
	if timestamp == 0 {
		return ""
	}
	tm := time.Unix(timestamp/1000, 0)
	loc, _ := time.LoadLocation("Europe/Moscow")
	return tm.In(loc).Format("02.01.2006 15:04:05")
}
