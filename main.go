package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mochensky/max2tg/src"
)

func safeInt64(m map[string]interface{}, key string) int64 {
	if v, ok := m[key]; ok {
		if i, ok := v.(int64); ok {
			return i
		}
	}
	return 0
}

func BuildOutput(message src.Message, senderName string, userNames *src.SafeMap, deletionTime *int64, loc *time.Location) string {
	timeStr := src.FormatTime(src.GetMessageTime(message), loc)

	text := ""
	if message.FormattedHTMLText != nil {
		text = *message.FormattedHTMLText
	} else {
		text = message.Text
	}

	if strings.HasPrefix(text, "Добавил") || strings.HasPrefix(text, "Присоединился") || strings.HasPrefix(text, "Создал") {
		return "• " + timeStr + "\n\n" + text
	}

	output := "• " + senderName + "\n• " + timeStr

	if message.Status == src.MessageStatusEDITED && message.UpdateTime != nil {
		editTimeStr := src.FormatTime(*message.UpdateTime, loc)
		output += "\n• [Редактировано " + editTimeStr + "]"
	}

	if message.ForwardedMessage != nil {
		fwdSender := ""
		if message.ForwardedMessage.Channel != nil {
			fwdSender = message.ForwardedMessage.Channel.Name
		} else if message.ForwardedMessage.SenderID != nil {
			uid := strconv.Itoa(*message.ForwardedMessage.SenderID)
			fwdSender = userNames.GetOrDefault(uid, uid)
		}
		if fwdSender == "" {
			fwdSender = strconv.Itoa(message.ForwardedMessage.ID)
		}
		output += "\n• [Пересланное сообщение от " + fwdSender + "]"
		if deletionTime != nil {
			deletionTimeStr := src.FormatTime(*deletionTime, loc)
			output += "\n• [Удалено " + deletionTimeStr + "]"
		}
		fwdText := ""
		if message.ForwardedMessage.FormattedHTMLText != nil {
			fwdText = *message.ForwardedMessage.FormattedHTMLText
		} else {
			fwdText = message.ForwardedMessage.Text
		}
		output += "\n\n" + fwdText
	} else {
		if deletionTime != nil {
			deletionTimeStr := src.FormatTime(*deletionTime, loc)
			output += "\n• [Удалено " + deletionTimeStr + "]"
		}
		output += "\n\n" + text
	}

	return output
}

func HandleControlMessage(message src.Message, userNames *src.SafeMap, loc *time.Location) string {
	if len(message.Attaches) == 0 {
		return ""
	}

	var controlAttach *src.Attachment
	for i := range message.Attaches {
		attach := message.Attaches[i]
		if attach.Type == src.AttachmentTypeControl {
			controlAttach = &attach
			break
		}
	}
	if controlAttach == nil {
		return ""
	}

	event := controlAttach.Event
	if event == "" || (event != "add" && event != "joinByLink" && event != "remove" && event != "leave" && event != "new") {
		return ""
	}

	timeStr := src.FormatTime(src.GetMessageTime(message), loc)

	getName := func(uid int) string {
		return userNames.GetOrDefault(strconv.Itoa(uid), strconv.Itoa(uid))
	}

	switch event {
	case "joinByLink":
		userID := controlAttach.UserID
		if userID == nil {
			uid := message.SenderID
			userID = &uid
		}
		name := getName(*userID)
		return "• " + timeStr + "\n\n" + name + " присоединился(-ась) к чату"

	case "add":
		if len(controlAttach.UserIDs) == 0 {
			return ""
		}
		actorName := getName(message.SenderID)
		addedNames := make([]string, len(controlAttach.UserIDs))
		for i, uid := range controlAttach.UserIDs {
			addedNames[i] = getName(uid)
		}
		text := ""
		if len(addedNames) == 1 {
			text = actorName + " добавил(-а) " + addedNames[0] + " в чат"
		} else {
			text = actorName + " добавил(-а) " + strings.Join(addedNames[:len(addedNames)-1], ", ") + " и " + addedNames[len(addedNames)-1] + " в чат"
		}
		return "• " + timeStr + "\n\n" + text

	case "remove":
		var removedUserIDs []int
		if len(controlAttach.UserIDs) > 0 {
			removedUserIDs = controlAttach.UserIDs
		} else if controlAttach.UserID != nil {
			removedUserIDs = []int{*controlAttach.UserID}
		} else {
			return ""
		}
		actorName := getName(message.SenderID)
		removedNames := make([]string, len(removedUserIDs))
		for i, uid := range removedUserIDs {
			removedNames[i] = getName(uid)
		}
		text := ""
		if len(removedNames) == 1 {
			text = actorName + " удалил(-а) " + removedNames[0] + " из чата"
		} else {
			text = actorName + " удалил(-а) " + strings.Join(removedNames[:len(removedNames)-1], ", ") + " и " + removedNames[len(removedNames)-1] + " из чата"
		}
		return "• " + timeStr + "\n\n" + text

	case "leave":
		name := getName(message.SenderID)
		return "• " + timeStr + "\n\n" + name + " покинул(-а) чат"

	case "new":
		actorName := getName(message.SenderID)
		return "• " + timeStr + "\n\n" + actorName + " создал(-а) новый чат"
	}

	return ""
}

func resolveContact(client *src.Client, userNames *src.SafeMap, userID int) string {
	key := strconv.Itoa(userID)
	if name, ok := userNames.Get(key); ok {
		return name
	}
	contacts, err := client.GetContacts([]int{userID})
	if err == nil && len(contacts) > 0 {
		contact := contacts[0]
		name := strings.TrimSpace(contact.FirstName + " " + contact.LastName)
		if name == "" {
			name = key
		}
		userNames.Set(key, name)
		return name
	}
	userNames.Set(key, key)
	return key
}

func ProcessMessage(client *src.Client, db *src.Database, sender *src.TelegramSender, message src.Message, userNames *src.SafeMap, channelNames *src.SafeIntMap, cfg *src.Config) {
	defer func() {
		if r := recover(); r != nil {
			src.Logf("PANIC in ProcessMessage (msgID=%d, chatID=%d): %v", message.ID, message.ChatID, r)
		}
	}()

	route := sender.FindRoute(message.ChatID)
	if route == nil {
		return
	}

	src.Logf("Processing message %d from chat %d (sender %d)", message.ID, message.ChatID, message.SenderID)

	existing, _ := db.GetMessageByMaxID(int64(message.ID))
	if existing != nil {
		return
	}

	src.Logf("Resolving sender contact for message %d", message.ID)
	resolveContact(client, userNames, message.SenderID)

	if message.ForwardedMessage != nil && message.ForwardedMessage.SenderID != nil {
		resolveContact(client, userNames, *message.ForwardedMessage.SenderID)
	}

	if message.ForwardedMessage != nil && message.ForwardedMessage.Channel != nil && message.ForwardedMessage.Channel.ID != 0 {
		channelID := message.ForwardedMessage.Channel.ID
		if _, ok := channelNames.Get(channelID); !ok {
			if message.ForwardedMessage.Channel.Name != "" {
				channelNames.Set(channelID, message.ForwardedMessage.Channel.Name)
			} else if channelID < 0 {
				if chats, err := client.GetChatInfo([]int{channelID}); err == nil && len(chats) > 0 {
					channelNames.Set(channelID, chats[0].Title)
				}
			}
		}
	}

	var controlUserIDs []int
	for _, attach := range message.Attaches {
		if attach.Type == src.AttachmentTypeControl {
			controlUserIDs = append(controlUserIDs, message.SenderID)
			switch attach.Event {
			case "add", "remove":
				controlUserIDs = append(controlUserIDs, attach.UserIDs...)
			case "joinByLink":
				if attach.UserID != nil {
					controlUserIDs = append(controlUserIDs, *attach.UserID)
				}
			}
			break
		}
	}

	if len(controlUserIDs) > 0 {
		uniqueUserIDs := make(map[int]struct{})
		for _, uid := range controlUserIDs {
			uniqueUserIDs[uid] = struct{}{}
		}
		idsToFetch := make([]int, 0, len(uniqueUserIDs))
		for uid := range uniqueUserIDs {
			if _, ok := userNames.Get(strconv.Itoa(uid)); !ok {
				idsToFetch = append(idsToFetch, uid)
			}
		}
		if len(idsToFetch) > 0 {
			contacts, err := client.GetContacts(idsToFetch)
			if err == nil {
				for _, contact := range contacts {
					key := strconv.Itoa(contact.ID)
					fullName := strings.TrimSpace(contact.FirstName + " " + contact.LastName)
					if fullName == "" {
						fullName = key
					}
					userNames.Set(key, fullName)
				}
			} else {
				src.Logf("Failed to fetch contacts for control message: %v", err)
			}
		}
	}

	loc := cfg.GetTimezone()

	if controlOutput := HandleControlMessage(message, userNames, loc); controlOutput != "" {
		tgMsgID, err := sender.SendMessage(controlOutput, message.ChatID, nil)
		if err != nil {
			src.Logf("Failed to send control message to Telegram: %v", err)
			return
		}
		ts := src.GetMessageTime(message)
		db.AddMessage(int64(message.ID), int64(tgMsgID), int64(message.SenderID), ts, 0)
		src.Logf("Control message %d saved with TG ID %d", message.ID, tgMsgID)
		return
	}

	audioPaths := []string{}
	audioDurations := []int{}
	audioFilePaths := []string{}
	filePaths := []string{}
	imagePaths := []string{}
	videoPaths := []string{}

	maxProxy := src.GetMaxProxy(cfg)

	for _, attach := range message.Attaches {
		switch attach.Type {
		case src.AttachmentTypeAudio:
			if attach.AudioURL != "" {
				path := src.DownloadAudio(attach.AudioURL, attach.AudioID, cfg.DownloadPath, cfg.AudioHeaders, cfg.UserAgent.UserAgent, maxProxy, cfg.MediaDownloadMaxRetries, cfg.MediaDownloadRetryDelay)
				if path != "" {
					audioPaths = append(audioPaths, path)
					dur := 0
					if attach.AudioDuration != nil {
						dur = *attach.AudioDuration
					}
					audioDurations = append(audioDurations, dur)
				}
			}
		case src.AttachmentTypeFile:
			url, err := client.GetFileLink(attach, message)
			if err == nil {
				path := src.DownloadFile(url, attach.FileID, attach.FileName, cfg.DownloadPath, cfg.UserAgent.UserAgent, maxProxy, cfg.MediaDownloadMaxRetries, cfg.MediaDownloadRetryDelay)
				if path != "" {
					if src.IsAudioFile(attach.FileName) {
						audioFilePaths = append(audioFilePaths, path)
					} else {
						filePaths = append(filePaths, path)
					}
				}
			}
		case src.AttachmentTypePhoto:
			if attach.BaseURL != "" && attach.PhotoToken != "" {
				path := src.DownloadPhoto(attach.BaseURL, attach.PhotoToken, attach.PhotoID, cfg.DownloadPath, cfg.UserAgent.UserAgent, maxProxy, cfg.MediaDownloadMaxRetries, cfg.MediaDownloadRetryDelay)
				if path != "" {
					imagePaths = append(imagePaths, path)
				}
			}
		case src.AttachmentTypeVideo:
			url, err := client.GetVideoLink(attach, message)
			if err == nil {
				path := src.DownloadVideo(url, attach.VideoID, cfg.DownloadPath, cfg.VideoHeaders, cfg.UserAgent.UserAgent, maxProxy, cfg.MediaDownloadMaxRetries, cfg.MediaDownloadRetryDelay)
				if path != "" {
					videoPaths = append(videoPaths, path)
				}
			}
		}
	}

	if message.ForwardedMessage != nil {
		for _, attach := range message.ForwardedMessage.Attaches {
			switch attach.Type {
			case src.AttachmentTypePhoto:
				if attach.BaseURL != "" && attach.PhotoToken != "" {
					path := src.DownloadPhoto(attach.BaseURL, attach.PhotoToken, attach.PhotoID, cfg.DownloadPath, cfg.UserAgent.UserAgent, maxProxy, cfg.MediaDownloadMaxRetries, cfg.MediaDownloadRetryDelay)
					if path != "" {
						imagePaths = append(imagePaths, path)
					}
				}
			case src.AttachmentTypeVideo:
				url, err := client.GetVideoLink(attach, message)
				if err == nil {
					path := src.DownloadVideo(url, attach.VideoID, cfg.DownloadPath, cfg.VideoHeaders, cfg.UserAgent.UserAgent, maxProxy, cfg.MediaDownloadMaxRetries, cfg.MediaDownloadRetryDelay)
					if path != "" {
						videoPaths = append(videoPaths, path)
					}
				}
			case src.AttachmentTypeFile:
				url, err := client.GetFileLink(attach, message)
				if err == nil {
					path := src.DownloadFile(url, attach.FileID, attach.FileName, cfg.DownloadPath, cfg.UserAgent.UserAgent, maxProxy, cfg.MediaDownloadMaxRetries, cfg.MediaDownloadRetryDelay)
					if path != "" {
						if src.IsAudioFile(attach.FileName) {
							audioFilePaths = append(audioFilePaths, path)
						} else {
							filePaths = append(filePaths, path)
						}
					}
				}
			case src.AttachmentTypeAudio:
				if attach.AudioURL != "" {
					path := src.DownloadAudio(attach.AudioURL, attach.AudioID, cfg.DownloadPath, cfg.AudioHeaders, cfg.UserAgent.UserAgent, maxProxy, cfg.MediaDownloadMaxRetries, cfg.MediaDownloadRetryDelay)
					if path != "" {
						audioPaths = append(audioPaths, path)
						dur := 0
						if attach.AudioDuration != nil {
							dur = *attach.AudioDuration
						}
						audioDurations = append(audioDurations, dur)
					}
				}
			}
		}
	}

	senderName := userNames.GetOrDefault(strconv.Itoa(message.SenderID), strconv.Itoa(message.SenderID))
	output := BuildOutput(message, senderName, userNames, nil, loc)

	var replyToMsgID *int
	if message.Link != nil {
		if linkType, ok := message.Link["type"].(string); ok && linkType == "REPLY" {
			if replyMsg, ok := message.Link["message"].(map[string]interface{}); ok {
				if replyIDVal := replyMsg["id"]; replyIDVal != nil {
					rid := src.ParseID(replyIDVal)
					if rid != 0 {
						existingReply, _ := db.GetMessageByMaxID(int64(rid))
						if existingReply != nil {
							tgID := int(safeInt64(existingReply, "tg_message_id"))
							if tgID != 0 {
								replyToMsgID = &tgID
							}
						}
					}
				}
			}
		}
	}

	var tgMsgID int

	hasMediaFiles := len(imagePaths) > 0 || len(videoPaths) > 0 || len(filePaths) > 0
	hasAudio := len(audioPaths) > 0
	hasAudioFiles := len(audioFilePaths) > 0

	src.Logf("Sending message %d to Telegram (hasMedia=%v, audioCount=%d, audioFileCount=%d)", message.ID, hasMediaFiles, len(audioPaths), len(audioFilePaths))

	if !hasMediaFiles && !hasAudio && !hasAudioFiles {
		var err error
		tgMsgID, err = sender.SendMessage(output, message.ChatID, replyToMsgID)
		if err != nil {
			src.Logf("Failed to send message to Telegram: %v", err)
			return
		}
	} else {
		if hasMediaFiles {
			allFiles := append(append([]string{}, imagePaths...), append(videoPaths, filePaths...)...)
			caption := output
			if hasAudio || hasAudioFiles {
				caption = ""
			}
			mediaGroupIDs, err := sender.SendMediaGroup(allFiles, caption, message.ChatID, replyToMsgID)
			if err != nil {
				src.Logf("Failed to send media group to Telegram: %v", err)
				return
			}
			if len(mediaGroupIDs) > 0 {
				tgMsgID = mediaGroupIDs[0]
			}
		}

		if hasAudioFiles {
			for _, audioFilePath := range audioFilePaths {
				caption := ""
				if !hasMediaFiles && !hasAudio {
					caption = output
				}
				audioMsgID, err := sender.SendAudio(audioFilePath, caption, message.ChatID, replyToMsgID)
				if err != nil {
					src.Logf("Failed to send audio file to Telegram: %v", err)
				} else if tgMsgID == 0 {
					tgMsgID = audioMsgID
				}
			}
		}

		if hasAudio {
			for i, audioPath := range audioPaths {
				dur := 0
				if i < len(audioDurations) {
					dur = audioDurations[i]
				}
				_, err := sender.SendVoice(audioPath, message.ChatID, replyToMsgID, dur)
				if err != nil {
					src.Logf("Failed to send voice to Telegram: %v", err)
				}
			}
		}

		if hasAudio || (hasAudioFiles && hasMediaFiles) {
			textMsgID, err := sender.SendMessage(output, message.ChatID, nil)
			if err != nil {
				src.Logf("Failed to send audio caption message to Telegram: %v", err)
			} else if tgMsgID == 0 {
				tgMsgID = textMsgID
			}
		}
	}

	ts := src.GetMessageTime(message)
	db.AddMessage(int64(message.ID), int64(tgMsgID), int64(message.SenderID), ts, 0)
	src.Logf("Message %d sent to Telegram with TG ID %d", message.ID, tgMsgID)
}

func HandleEditedMessage(client *src.Client, db *src.Database, sender *src.TelegramSender, message src.Message, userNames *src.SafeMap, channelNames *src.SafeIntMap, cfg *src.Config) {
	route := sender.FindRoute(message.ChatID)
	if route == nil {
		return
	}

	existing, err := db.GetMessageByMaxID(int64(message.ID))
	if err != nil || existing == nil {
		src.Logf("Edited message %d not found in database", message.ID)
		return
	}

	resolveContact(client, userNames, message.SenderID)

	if message.ForwardedMessage != nil && message.ForwardedMessage.SenderID != nil {
		resolveContact(client, userNames, *message.ForwardedMessage.SenderID)
	}

	loc := cfg.GetTimezone()
	senderName := userNames.GetOrDefault(strconv.Itoa(message.SenderID), strconv.Itoa(message.SenderID))
	output := BuildOutput(message, senderName, userNames, nil, loc)

	tgMsgID := int(safeInt64(existing, "tg_message_id"))
	if tgMsgID == 0 {
		src.Logf("Edited message %d has no valid tg_message_id in database, skipping", message.ID)
		return
	}

	hasAttachments := len(message.Attaches) > 0
	if message.ForwardedMessage != nil {
		hasAttachments = hasAttachments || len(message.ForwardedMessage.Attaches) > 0
	}

	if hasAttachments {
		err = sender.EditMessageCaption(tgMsgID, output, message.ChatID)
	} else {
		err = sender.EditMessageText(tgMsgID, output, message.ChatID)
	}
	if err != nil {
		src.Logf("Failed to edit message %d in Telegram: %v", message.ID, err)
	} else {
		src.Logf("Message %d edited in Telegram", message.ID)
		editTime := src.GetMessageTime(message)
		if message.UpdateTime != nil {
			editTime = *message.UpdateTime
		}
		if editTime > 0 {
			db.UpdateMessageEditedAt(int64(message.ID), editTime)
		}
	}
}

func HandleDeletedMessage(client *src.Client, db *src.Database, sender *src.TelegramSender, message src.Message, userNames *src.SafeMap, channelNames *src.SafeIntMap, cfg *src.Config) {
	route := sender.FindRoute(message.ChatID)
	if route == nil {
		return
	}

	existing, err := db.GetMessageByMaxID(int64(message.ID))
	if err != nil || existing == nil {
		src.Logf("Deleted message %d not found in database", message.ID)
		return
	}

	tgMsgID := int(safeInt64(existing, "tg_message_id"))
	if tgMsgID == 0 {
		src.Logf("Deleted message %d has no valid tg_message_id in database, skipping", message.ID)
		return
	}

	if !cfg.SaveDeleted {
		err = sender.DeleteMessage(tgMsgID, message.ChatID)
		if err != nil {
			src.Logf("Failed to delete message %d in Telegram: %v", message.ID, err)
		} else {
			db.DeleteMessageByMaxID(int64(message.ID))
			src.Logf("Message %d deleted from Telegram and database", message.ID)
		}
		return
	}

	resolveContact(client, userNames, message.SenderID)

	if message.ForwardedMessage != nil && message.ForwardedMessage.SenderID != nil {
		resolveContact(client, userNames, *message.ForwardedMessage.SenderID)
	}

	senderName := userNames.GetOrDefault(strconv.Itoa(message.SenderID), strconv.Itoa(message.SenderID))

	var deletionTimestamp int64
	if message.UpdateTime != nil {
		deletionTimestamp = *message.UpdateTime
	} else {
		deletionTimestamp = time.Now().Unix()
	}

	loc := cfg.GetTimezone()
	output := BuildOutput(message, senderName, userNames, &deletionTimestamp, loc)

	hasAttachments := len(message.Attaches) > 0
	if message.ForwardedMessage != nil {
		hasAttachments = hasAttachments || len(message.ForwardedMessage.Attaches) > 0
	}

	if hasAttachments {
		err = sender.EditMessageCaption(tgMsgID, output, message.ChatID)
	} else {
		err = sender.EditMessageText(tgMsgID, output, message.ChatID)
	}

	if err != nil {
		src.Logf("Failed to edit deleted message %d in Telegram: %v", message.ID, err)
	} else {
		src.Logf("Message %d marked as deleted (edited with marker)", message.ID)
	}
}

func SyncChatHistory(client *src.Client, db *src.Database, sender *src.TelegramSender, userNames *src.SafeMap, channelNames *src.SafeIntMap, cfg *src.Config, chatID int) {
	src.Logf("Starting chat history synchronization for chat %d...", chatID)

	messages, err := client.GetMessages(chatID, cfg.SyncHistoryDepth, 0, nil)
	if err != nil {
		src.Logf("Failed to get chat history: %v", err)
		return
	}

	for _, msg := range messages {
		if msg.Status == src.MessageStatusREMOVED {
			continue
		}

		existing, _ := db.GetMessageByMaxID(int64(msg.ID))
		if existing == nil {
			src.Logf("Found historical message %d, sending to Telegram", msg.ID)
			ProcessMessage(client, db, sender, msg, userNames, channelNames, cfg)
		} else {
			if msg.Status == src.MessageStatusEDITED {
				updateTime := src.GetMessageTime(msg)
				if msg.UpdateTime != nil {
					updateTime = *msg.UpdateTime
				}
				storedEditTime := safeInt64(existing, "edited_at")
				if updateTime > storedEditTime {
					src.Logf("Message %d was edited, updating in Telegram", msg.ID)
					HandleEditedMessage(client, db, sender, msg, userNames, channelNames, cfg)
				}
			}
		}
	}

	src.Logf("Chat history synchronization completed.")
}

func main() {
	configPath := "data/config.yml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := src.LoadConfig(configPath)
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if err := src.SetupLogger(cfg.LogPath, cfg.Timezone); err != nil {
		fmt.Printf("Failed to setup logger: %v\n", err)
		os.Exit(1)
	}
	defer src.CloseLogger()

	src.Logf("Starting %s...", src.GetVersionInfo())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	latestVersion, updateAvailable, err := src.CheckForUpdates(ctx)
	cancel()

	if err != nil {
		src.Logf("Failed to check for updates: %v", err)
	} else if updateAvailable {
		src.Logf("New version available: %s (current: %s). Download: https://github.com/mochensky/max2tg/releases/latest", latestVersion, src.AppVersion)
	} else {
		src.Logf("Application is up to date (%s)", src.AppVersion)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stop
		src.Logf("Stopped!")
		os.Exit(0)
	}()

	db, err := src.NewDatabase(cfg.DBPath)
	if err != nil {
		src.Logf("Failed to initialize database: %v", err)
		return
	}
	defer db.Close()

	telegramSender := src.NewTelegramSender(cfg.TGToken, cfg.ChatRoutes, cfg)

	userNames := src.NewSafeMap()
	channelNames := src.NewSafeIntMap()

	client := src.NewClient(cfg)

	client.OnMessage(func(message src.Message) {
		ProcessMessage(client, db, telegramSender, message, userNames, channelNames, cfg)
	})

	client.OnEdited(func(message src.Message) {
		HandleEditedMessage(client, db, telegramSender, message, userNames, channelNames, cfg)
	})

	client.OnDeleted(func(message src.Message) {
		HandleDeletedMessage(client, db, telegramSender, message, userNames, channelNames, cfg)
	})

	client.OnDisconnected(func(reason string) {
		src.Logf("Disconnected: %s", reason)
		if cfg.TGDebugUserID != 0 {
			telegramSender.SendDebugMessage("Disconnected: "+reason, cfg.TGDebugUserID)
		}
	})

	client.OnAfterReconnect(func() {
		src.Logf("Reconnected!")
		if cfg.TGDebugUserID != 0 {
			telegramSender.SendDebugMessage("Reconnected!", cfg.TGDebugUserID)
		}
		for _, route := range cfg.ChatRoutes {
			SyncChatHistory(client, db, telegramSender, userNames, channelNames, cfg, route.MaxChatID)
		}
	})

	if err := client.Start(); err != nil {
		src.Logf("Failed to start client: %v", err)
		return
	}

	me := client.GetMe()
	if me == nil {
		src.Logf("Failed to get client info")
		return
	}
	src.Logf("Connected as %s (ID: %d)", me.FirstName, me.ID)

	for _, route := range cfg.ChatRoutes {
		targetChat := client.GetChat(route.MaxChatID)
		if targetChat == nil {
			src.Logf("Chat %d not found", route.MaxChatID)
			continue
		}

		for userID := range targetChat.Participants {
			resolveContact(client, userNames, userID)
		}

		SyncChatHistory(client, db, telegramSender, userNames, channelNames, cfg, route.MaxChatID)
	}

	select {}
}
