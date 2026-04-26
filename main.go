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

func BuildOutput(message src.Message, senderName string, userNames map[string]string, deletionTime *int64) string {
	timeStr := src.FormatTime(src.GetMessageTime(message))

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
		editTimeStr := src.FormatTime(*message.UpdateTime)
		output += "\n• [Редактировано " + editTimeStr + "]"
	}

	if message.ForwardedMessage != nil {
		fwdSender := ""
		if message.ForwardedMessage.Channel != nil {
			fwdSender = message.ForwardedMessage.Channel.Name
		} else if message.ForwardedMessage.SenderID != nil {
			uid := strconv.Itoa(*message.ForwardedMessage.SenderID)
			if name, ok := userNames[uid]; ok {
				fwdSender = name
			} else {
				fwdSender = uid
			}
		}
		if fwdSender == "" {
			fwdSender = strconv.Itoa(message.ForwardedMessage.ID)
		}
		output += "\n• [Пересланное сообщение от " + fwdSender + "]"
		if deletionTime != nil {
			deletionTimeStr := src.FormatTime(*deletionTime)
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
			deletionTimeStr := src.FormatTime(*deletionTime)
			output += "\n• [Удалено " + deletionTimeStr + "]"
		}
		output += "\n\n" + text
	}

	return output
}

func HandleControlMessage(message src.Message, userNames map[string]string) string {
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

	timeStr := src.FormatTime(src.GetMessageTime(message))

	getName := func(uid int) string {
		if name, ok := userNames[strconv.Itoa(uid)]; ok {
			return name
		}
		return strconv.Itoa(uid)
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

func ProcessMessage(client *src.Client, db *src.Database, sender *src.TelegramSender, message src.Message, userNames map[string]string, channelNames map[int]string, cfg *src.Config) {
	route := sender.FindRoute(message.ChatID)
	if route == nil {
		return
	}

	existing, _ := db.GetMessageByMaxID(int64(message.ID))
	if existing != nil {
		return
	}

	senderIDStr := strconv.Itoa(message.SenderID)
	if _, ok := userNames[senderIDStr]; !ok {
		contacts, err := client.GetContacts([]int{message.SenderID})
		if err == nil && len(contacts) > 0 {
			contact := contacts[0]
			userNames[senderIDStr] = strings.TrimSpace(contact.FirstName + " " + contact.LastName)
		} else {
			userNames[senderIDStr] = senderIDStr
		}
	}

	if message.ForwardedMessage != nil && message.ForwardedMessage.SenderID != nil {
		fwdSenderIDStr := strconv.Itoa(*message.ForwardedMessage.SenderID)
		if _, ok := userNames[fwdSenderIDStr]; !ok {
			contacts, err := client.GetContacts([]int{*message.ForwardedMessage.SenderID})
			if err == nil && len(contacts) > 0 {
				contact := contacts[0]
				userNames[fwdSenderIDStr] = strings.TrimSpace(contact.FirstName + " " + contact.LastName)
			} else {
				userNames[fwdSenderIDStr] = fwdSenderIDStr
			}
		}
	}

	if message.ForwardedMessage != nil && message.ForwardedMessage.Channel != nil && message.ForwardedMessage.Channel.ID != 0 {
		channelID := message.ForwardedMessage.Channel.ID
		if _, ok := channelNames[channelID]; !ok {
			if message.ForwardedMessage.Channel.Name != "" {
				channelNames[channelID] = message.ForwardedMessage.Channel.Name
			} else if channelID < 0 {
				if chats, err := client.GetChatInfo([]int{channelID}); err == nil && len(chats) > 0 {
					channelNames[channelID] = chats[0].Title
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
			uidStr := strconv.Itoa(uid)
			if _, ok := userNames[uidStr]; !ok {
				idsToFetch = append(idsToFetch, uid)
			}
		}

		if len(idsToFetch) > 0 {
			contacts, err := client.GetContacts(idsToFetch)
			if err == nil {
				for _, contact := range contacts {
					uidStr := strconv.Itoa(contact.ID)
					fullName := strings.TrimSpace(contact.FirstName + " " + contact.LastName)
					if fullName == "" {
						fullName = uidStr
					}
					userNames[uidStr] = fullName
				}
			} else {
				src.Logf("Failed to fetch contacts for control message: %v", err)
			}
		}
	}

	if controlOutput := HandleControlMessage(message, userNames); controlOutput != "" {
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
				}
			}
		case src.AttachmentTypeFile:
			url, err := client.GetFileLink(attach, message)
			if err == nil {
				path := src.DownloadFile(url, attach.FileID, attach.FileName, cfg.DownloadPath, cfg.UserAgent.UserAgent, maxProxy, cfg.MediaDownloadMaxRetries, cfg.MediaDownloadRetryDelay)
				if path != "" {
					filePaths = append(filePaths, path)
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
						filePaths = append(filePaths, path)
					}
				}
			case src.AttachmentTypeAudio:
				if attach.AudioURL != "" {
					path := src.DownloadAudio(attach.AudioURL, attach.AudioID, cfg.DownloadPath, cfg.AudioHeaders, cfg.UserAgent.UserAgent, maxProxy, cfg.MediaDownloadMaxRetries, cfg.MediaDownloadRetryDelay)
					if path != "" {
						audioPaths = append(audioPaths, path)
					}
				}
			}
		}
	}

	senderName := userNames[senderIDStr]

	output := BuildOutput(message, senderName, userNames, nil)

	var replyToMsgID *int
	if message.Link != nil {
		if linkType, ok := message.Link["type"].(string); ok && linkType == "REPLY" {
			if replyMsg, ok := message.Link["message"].(map[string]interface{}); ok {
				if replyIDVal := replyMsg["id"]; replyIDVal != nil {
					rid := src.ParseID(replyIDVal)
					if rid != 0 {
						existingReply, _ := db.GetMessageByMaxID(int64(rid))
						if existingReply != nil {
							tgID := int(existingReply["tg_message_id"].(int64))
							replyToMsgID = &tgID
						}
					}
				}
			}
		}
	}

	var tgMsgID int

	hasMediaFiles := len(imagePaths) > 0 || len(videoPaths) > 0 || len(filePaths) > 0

	if !hasMediaFiles && len(audioPaths) == 0 {
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
			if len(audioPaths) > 0 {
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

		for i, audioPath := range audioPaths {
			caption := ""
			if i == 0 && !hasMediaFiles {
				caption = output
			}
			audioMsgID, err := sender.SendAudio(audioPath, caption, message.ChatID, replyToMsgID)
			if err != nil {
				src.Logf("Failed to send audio to Telegram: %v", err)
				continue
			}
			if tgMsgID == 0 {
				tgMsgID = audioMsgID
			}
		}

		if hasMediaFiles && len(audioPaths) > 0 {
			_, err := sender.SendMessage(output, message.ChatID, replyToMsgID)
			if err != nil {
				src.Logf("Failed to send text message to Telegram: %v", err)
			}
		}
	}

	ts := src.GetMessageTime(message)
	db.AddMessage(int64(message.ID), int64(tgMsgID), int64(message.SenderID), ts, 0)
	src.Logf("Message %d sent to Telegram with TG ID %d", message.ID, tgMsgID)
}

func HandleEditedMessage(client *src.Client, db *src.Database, sender *src.TelegramSender, message src.Message, userNames map[string]string, channelNames map[int]string, cfg *src.Config) {
	route := sender.FindRoute(message.ChatID)
	if route == nil {
		return
	}

	existing, err := db.GetMessageByMaxID(int64(message.ID))
	if err != nil || existing == nil {
		src.Logf("Edited message %d not found in database", message.ID)
		return
	}

	senderIDStr := strconv.Itoa(message.SenderID)
	if _, ok := userNames[senderIDStr]; !ok {
		contacts, err := client.GetContacts([]int{message.SenderID})
		if err == nil && len(contacts) > 0 {
			contact := contacts[0]
			userNames[senderIDStr] = strings.TrimSpace(contact.FirstName + " " + contact.LastName)
		} else {
			userNames[senderIDStr] = senderIDStr
		}
	}

	if message.ForwardedMessage != nil && message.ForwardedMessage.SenderID != nil {
		fwdSenderIDStr := strconv.Itoa(*message.ForwardedMessage.SenderID)
		if _, ok := userNames[fwdSenderIDStr]; !ok {
			contacts, err := client.GetContacts([]int{*message.ForwardedMessage.SenderID})
			if err == nil && len(contacts) > 0 {
				contact := contacts[0]
				userNames[fwdSenderIDStr] = strings.TrimSpace(contact.FirstName + " " + contact.LastName)
			} else {
				userNames[fwdSenderIDStr] = fwdSenderIDStr
			}
		}
	}

	name := userNames[senderIDStr]
	output := BuildOutput(message, name, userNames, nil)

	tgMsgID := int(existing["tg_message_id"].(int64))

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

func HandleDeletedMessage(client *src.Client, db *src.Database, sender *src.TelegramSender, message src.Message, userNames map[string]string, channelNames map[int]string, cfg *src.Config) {
	route := sender.FindRoute(message.ChatID)
	if route == nil {
		return
	}

	existing, err := db.GetMessageByMaxID(int64(message.ID))
	if err != nil || existing == nil {
		src.Logf("Deleted message %d not found in database", message.ID)
		return
	}

	tgMsgID := int(existing["tg_message_id"].(int64))

	if !cfg.SaveDeleted {
		err = sender.DeleteMessage(tgMsgID, message.ChatID)
		if err != nil {
			src.Logf("Failed to delete message %d in Telegram: %v", message.ID, err)
		} else {
			db.DeleteMessageByMaxID(int64(message.ID))
			src.Logf("Message %d deleted from Telegram and database", message.ID)
		}
	} else {
		senderIDStr := strconv.Itoa(message.SenderID)
		if _, ok := userNames[senderIDStr]; !ok {
			contacts, err := client.GetContacts([]int{message.SenderID})
			if err == nil && len(contacts) > 0 {
				contact := contacts[0]
				userNames[senderIDStr] = strings.TrimSpace(contact.FirstName + " " + contact.LastName)
			} else {
				userNames[senderIDStr] = senderIDStr
			}
		}

		if message.ForwardedMessage != nil && message.ForwardedMessage.SenderID != nil {
			fwdSenderIDStr := strconv.Itoa(*message.ForwardedMessage.SenderID)
			if _, ok := userNames[fwdSenderIDStr]; !ok {
				contacts, err := client.GetContacts([]int{*message.ForwardedMessage.SenderID})
				if err == nil && len(contacts) > 0 {
					contact := contacts[0]
					userNames[fwdSenderIDStr] = strings.TrimSpace(contact.FirstName + " " + contact.LastName)
				} else {
					userNames[fwdSenderIDStr] = fwdSenderIDStr
				}
			}
		}

		senderName := userNames[senderIDStr]

		var deletionTimestamp int64
		if message.UpdateTime != nil {
			deletionTimestamp = *message.UpdateTime
		} else {
			deletionTimestamp = time.Now().Unix()
		}

		output := BuildOutput(message, senderName, userNames, &deletionTimestamp)

		hasAttachments := len(message.Attaches) > 0
		if message.ForwardedMessage != nil {
			hasAttachments = hasAttachments || len(message.ForwardedMessage.Attaches) > 0
		}

		err = nil
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
}

func SyncChatHistory(client *src.Client, db *src.Database, sender *src.TelegramSender, userNames map[string]string, channelNames map[int]string, cfg *src.Config, chatID int) {
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
				storedEditTime := existing["edited_at"].(int64)
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

	if err := src.SetupLogger(cfg.LogPath, cfg.LogTimezone); err != nil {
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

	userNames := make(map[string]string)
	channelNames := make(map[int]string)

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
			contacts, err := client.GetContacts([]int{userID})
			if err == nil && len(contacts) > 0 {
				contact := contacts[0]
				userNames[strconv.Itoa(userID)] = strings.TrimSpace(contact.FirstName + " " + contact.LastName)
			} else {
				userNames[strconv.Itoa(userID)] = strconv.Itoa(userID)
			}
		}

		SyncChatHistory(client, db, telegramSender, userNames, channelNames, cfg, route.MaxChatID)
	}

	select {}
}
