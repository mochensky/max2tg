package src

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type TelegramSender struct {
	botToken       string
	routes         []ChatRoute
	maxRetries     int
	baseRetryDelay time.Duration
}

func NewTelegramSender(botToken string, routes []ChatRoute, cfg *Config) *TelegramSender {
	if cfg == nil {
		cfg = DefaultConfig
	}

	return &TelegramSender{
		botToken:       botToken,
		routes:         routes,
		maxRetries:     cfg.MaxRetries,
		baseRetryDelay: cfg.BaseRetryDelay,
	}
}

type RateLimitError struct {
	RetryAfter int
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited, retry after %d seconds", e.RetryAfter)
}

func isRateLimitError(body string) (*RateLimitError, bool) {
	var resp struct {
		OK          bool   `json:"ok"`
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
		Parameters  struct {
			RetryAfter int `json:"retry_after"`
		} `json:"parameters"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, false
	}
	if resp.ErrorCode == 429 {
		retryAfter := resp.Parameters.RetryAfter
		return &RateLimitError{RetryAfter: retryAfter + 1}, true
	}
	return nil, false
}

func (s *TelegramSender) FindRoute(maxChatID int) *ChatRoute {
	for i := range s.routes {
		if s.routes[i].MaxChatID == maxChatID {
			return &s.routes[i]
		}
	}
	return nil
}

func (s *TelegramSender) SendMessage(text string, maxChatID int, replyToMessageID *int) (int, error) {
	route := s.FindRoute(maxChatID)
	if route == nil {
		return 0, fmt.Errorf("no route found for MAX chat ID %d", maxChatID)
	}

	var lastErr error

	for attempt := 0; attempt < s.maxRetries; attempt++ {
		if attempt > 0 {
			retryDelay := s.baseRetryDelay * time.Duration(1<<uint(attempt-1))
			Logf("Retrying SendMessage (attempt %d/%d) after %v: %v", attempt+1, s.maxRetries, retryDelay, lastErr)
			time.Sleep(retryDelay)
		}

		startTime := time.Now()

		url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.botToken)
		payload := map[string]interface{}{
			"chat_id":    route.TelegramChatID,
			"text":       text,
			"parse_mode": "HTML",
		}
		if route.TelegramTopicID > 0 {
			payload["message_thread_id"] = route.TelegramTopicID
		}
		if replyToMessageID != nil {
			payload["reply_to_message_id"] = *replyToMessageID
		}

		jsonData, err := json.Marshal(payload)
		if err != nil {
			return 0, err
		}

		resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode == http.StatusTooManyRequests {
			if rateLimitErr, ok := isRateLimitError(string(body)); ok {
				Logf("Rate limited by Telegram, waiting %d seconds", rateLimitErr.RetryAfter)
				time.Sleep(time.Duration(rateLimitErr.RetryAfter) * time.Second)
				lastErr = rateLimitErr
				continue
			}
			lastErr = fmt.Errorf("telegram API error: %s", string(body))
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("telegram API error: %s", string(body))
			continue
		}

		var result struct {
			OK     bool `json:"ok"`
			Result struct {
				MessageID int `json:"message_id"`
			} `json:"result"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			lastErr = err
			continue
		}

		if !result.OK {
			lastErr = fmt.Errorf("telegram API returned not OK")
			continue
		}

		Logf("Message sent in %v", time.Since(startTime))
		return result.Result.MessageID, nil
	}

	return 0, fmt.Errorf("failed to send message after %d retries: %w", s.maxRetries, lastErr)
}

func (s *TelegramSender) SendMediaGroup(files []string, caption string, maxChatID int, replyToMessageID *int) ([]int, error) {
	route := s.FindRoute(maxChatID)
	if route == nil {
		return nil, fmt.Errorf("no route found for MAX chat ID %d", maxChatID)
	}

	var lastErr error

	for attempt := 0; attempt < s.maxRetries; attempt++ {
		if attempt > 0 {
			retryDelay := s.baseRetryDelay * time.Duration(1<<uint(attempt-1))
			Logf("Retrying SendMediaGroup (attempt %d/%d) after %v: %v", attempt+1, s.maxRetries, retryDelay, lastErr)
			time.Sleep(retryDelay)
		}

		startTime := time.Now()

		url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMediaGroup", s.botToken)
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		writer.WriteField("chat_id", fmt.Sprintf("%d", route.TelegramChatID))
		if route.TelegramTopicID > 0 {
			writer.WriteField("message_thread_id", fmt.Sprintf("%d", route.TelegramTopicID))
		}
		if caption != "" {
			writer.WriteField("caption", caption)
			writer.WriteField("parse_mode", "HTML")
		}
		if replyToMessageID != nil {
			writer.WriteField("reply_to_message_id", fmt.Sprintf("%d", *replyToMessageID))
		}

		media := []map[string]string{}
		for i, file := range files {
			media = append(media, map[string]string{
				"type":  s.getMediaType(file),
				"media": fmt.Sprintf("attach://file%d", i),
			})
		}

		if len(media) > 0 {
			media[0]["caption"] = caption
			media[0]["parse_mode"] = "HTML"
		}

		mediaJSON, _ := json.Marshal(media)
		writer.WriteField("media", string(mediaJSON))

		for i, file := range files {
			part, err := writer.CreateFormFile(fmt.Sprintf("file%d", i), filepath.Base(file))
			if err != nil {
				return nil, err
			}
			f, err := os.Open(file)
			if err != nil {
				return nil, err
			}
			defer f.Close()
			io.Copy(part, f)
		}

		writer.Close()

		resp, err := http.Post(url, writer.FormDataContentType(), &buf)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode == http.StatusTooManyRequests {
			if rateLimitErr, ok := isRateLimitError(string(body)); ok {
				Logf("Rate limited by Telegram, waiting %d seconds", rateLimitErr.RetryAfter)
				time.Sleep(time.Duration(rateLimitErr.RetryAfter) * time.Second)
				lastErr = rateLimitErr
				continue
			}
			lastErr = fmt.Errorf("telegram API error: %s", string(body))
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("telegram API error: %s", string(body))
			continue
		}

		var result struct {
			OK     bool `json:"ok"`
			Result []struct {
				MessageID int `json:"message_id"`
			} `json:"result"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			lastErr = err
			continue
		}

		if !result.OK {
			lastErr = fmt.Errorf("telegram API returned not OK")
			continue
		}

		messageIDs := make([]int, len(result.Result))
		for i, r := range result.Result {
			messageIDs[i] = r.MessageID
		}

		Logf("Media group sent in %v", time.Since(startTime))
		return messageIDs, nil
	}

	return nil, fmt.Errorf("failed to send media group after %d attempts: %w", s.maxRetries, lastErr)
}

func (s *TelegramSender) SendAudio(filePath string, caption string, maxChatID int, replyToMessageID *int) (int, error) {
	route := s.FindRoute(maxChatID)
	if route == nil {
		return 0, fmt.Errorf("no route found for MAX chat ID %d", maxChatID)
	}

	var lastErr error

	for attempt := 0; attempt < s.maxRetries; attempt++ {
		if attempt > 0 {
			retryDelay := s.baseRetryDelay * time.Duration(1<<uint(attempt-1))
			Logf("Retrying SendAudio (attempt %d/%d) after %v: %v", attempt+1, s.maxRetries, retryDelay, lastErr)
			time.Sleep(retryDelay)
		}

		startTime := time.Now()

		endpoint := "sendAudio"
		fieldName := "audio"

		url := fmt.Sprintf("https://api.telegram.org/bot%s/%s", s.botToken, endpoint)
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		writer.WriteField("chat_id", fmt.Sprintf("%d", route.TelegramChatID))
		if route.TelegramTopicID > 0 {
			writer.WriteField("message_thread_id", fmt.Sprintf("%d", route.TelegramTopicID))
		}
		if caption != "" {
			writer.WriteField("caption", caption)
			writer.WriteField("parse_mode", "HTML")
		}
		if replyToMessageID != nil {
			writer.WriteField("reply_to_message_id", fmt.Sprintf("%d", *replyToMessageID))
		}

		part, err := writer.CreateFormFile(fieldName, filepath.Base(filePath))
		if err != nil {
			return 0, err
		}
		f, err := os.Open(filePath)
		if err != nil {
			return 0, err
		}
		defer f.Close()
		io.Copy(part, f)
		writer.Close()

		resp, err := http.Post(url, writer.FormDataContentType(), &buf)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode == http.StatusTooManyRequests {
			if rateLimitErr, ok := isRateLimitError(string(body)); ok {
				Logf("Rate limited by Telegram, waiting %d seconds", rateLimitErr.RetryAfter)
				time.Sleep(time.Duration(rateLimitErr.RetryAfter) * time.Second)
				lastErr = rateLimitErr
				continue
			}
			lastErr = fmt.Errorf("telegram API error: %s", string(body))
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("telegram API error: %s", string(body))
			continue
		}

		var result struct {
			OK     bool `json:"ok"`
			Result struct {
				MessageID int `json:"message_id"`
			} `json:"result"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			lastErr = err
			continue
		}

		if !result.OK {
			lastErr = fmt.Errorf("telegram API returned not OK")
			continue
		}

		Logf("Audio sent in %v", time.Since(startTime))
		return result.Result.MessageID, nil
	}

	return 0, fmt.Errorf("failed to send audio after %d retries: %w", s.maxRetries, lastErr)
}

func (s *TelegramSender) getMediaType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return "photo"
	case ".mp4", ".avi", ".mov", ".mkv":
		return "video"
	default:
		return "document"
	}
}

func (s *TelegramSender) EditMessageText(messageID int, text string, maxChatID int) error {
	route := s.FindRoute(maxChatID)
	if route == nil {
		return fmt.Errorf("no route found for MAX chat ID %d", maxChatID)
	}

	var lastErr error

	for attempt := 0; attempt < s.maxRetries; attempt++ {
		if attempt > 0 {
			retryDelay := s.baseRetryDelay * time.Duration(1<<uint(attempt-1))
			Logf("Retrying EditMessageText (attempt %d/%d) after %v: %v", attempt+1, s.maxRetries, retryDelay, lastErr)
			time.Sleep(retryDelay)
		}

		url := fmt.Sprintf("https://api.telegram.org/bot%s/editMessageText", s.botToken)

		payload := map[string]interface{}{
			"chat_id":    route.TelegramChatID,
			"message_id": messageID,
			"text":       text,
			"parse_mode": "HTML",
		}

		jsonData, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode == http.StatusTooManyRequests {
			if rateLimitErr, ok := isRateLimitError(string(body)); ok {
				Logf("Rate limited by Telegram, waiting %d seconds", rateLimitErr.RetryAfter)
				time.Sleep(time.Duration(rateLimitErr.RetryAfter) * time.Second)
				lastErr = rateLimitErr
				continue
			}
			lastErr = fmt.Errorf("telegram API error: %s", string(body))
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("telegram API error: %s", string(body))
			continue
		}

		var result struct {
			OK bool `json:"ok"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			lastErr = err
			continue
		}

		if !result.OK {
			lastErr = fmt.Errorf("telegram API returned not OK")
			continue
		}

		return nil
	}

	return fmt.Errorf("failed to edit message after %d attempts: %w", s.maxRetries, lastErr)
}

func (s *TelegramSender) EditMessageCaption(messageID int, caption string, maxChatID int) error {
	route := s.FindRoute(maxChatID)
	if route == nil {
		return fmt.Errorf("no route found for MAX chat ID %d", maxChatID)
	}

	var lastErr error

	for attempt := 0; attempt < s.maxRetries; attempt++ {
		if attempt > 0 {
			retryDelay := s.baseRetryDelay * time.Duration(1<<uint(attempt-1))
			Logf("Retrying EditMessageCaption (attempt %d/%d) after %v: %v", attempt+1, s.maxRetries, retryDelay, lastErr)
			time.Sleep(retryDelay)
		}

		url := fmt.Sprintf("https://api.telegram.org/bot%s/editMessageCaption", s.botToken)

		payload := map[string]interface{}{
			"chat_id":    route.TelegramChatID,
			"message_id": messageID,
			"caption":    caption,
			"parse_mode": "HTML",
		}

		jsonData, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode == http.StatusTooManyRequests {
			if rateLimitErr, ok := isRateLimitError(string(body)); ok {
				Logf("Rate limited by Telegram, waiting %d seconds", rateLimitErr.RetryAfter)
				time.Sleep(time.Duration(rateLimitErr.RetryAfter) * time.Second)
				lastErr = rateLimitErr
				continue
			}
			lastErr = fmt.Errorf("telegram API error: %s", string(body))
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("telegram API error: %s", string(body))
			continue
		}

		var result struct {
			OK bool `json:"ok"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			lastErr = err
			continue
		}

		if !result.OK {
			lastErr = fmt.Errorf("telegram API returned not OK")
			continue
		}

		return nil
	}

	return fmt.Errorf("failed to edit message caption after %d attempts: %w", s.maxRetries, lastErr)
}

func (s *TelegramSender) DeleteMessage(messageID int, maxChatID int) error {
	route := s.FindRoute(maxChatID)
	if route == nil {
		return fmt.Errorf("no route found for MAX chat ID %d", maxChatID)
	}

	var lastErr error

	for attempt := 0; attempt < s.maxRetries; attempt++ {
		if attempt > 0 {
			retryDelay := s.baseRetryDelay * time.Duration(1<<uint(attempt-1))
			Logf("Retrying DeleteMessage (attempt %d/%d) after %v: %v", attempt+1, s.maxRetries, retryDelay, lastErr)
			time.Sleep(retryDelay)
		}

		url := fmt.Sprintf("https://api.telegram.org/bot%s/deleteMessage", s.botToken)

		payload := map[string]interface{}{
			"chat_id":    route.TelegramChatID,
			"message_id": messageID,
		}

		jsonData, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode == http.StatusTooManyRequests {
			if rateLimitErr, ok := isRateLimitError(string(body)); ok {
				Logf("Rate limited by Telegram, waiting %d seconds", rateLimitErr.RetryAfter)
				time.Sleep(time.Duration(rateLimitErr.RetryAfter) * time.Second)
				lastErr = rateLimitErr
				continue
			}
			lastErr = fmt.Errorf("telegram API error: %s", string(body))
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("telegram API error: %s", string(body))
			continue
		}

		var result struct {
			OK bool `json:"ok"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			lastErr = err
			continue
		}

		if !result.OK {
			lastErr = fmt.Errorf("telegram API returned not OK")
			continue
		}

		return nil
	}

	return fmt.Errorf("failed to delete message after %d attempts: %w", s.maxRetries, lastErr)
}

func (s *TelegramSender) SendDebugMessage(text string, userID int64) error {
	if userID == 0 {
		return nil
	}
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.botToken)

	payload := map[string]interface{}{
		"chat_id": userID,
		"text":    text,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_ = resp
	return nil
}
