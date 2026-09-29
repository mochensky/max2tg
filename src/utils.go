package src

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// IsAudioFile returns true for file extensions that Telegram can play inline as audio.
func IsAudioFile(fileName string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	switch ext {
	case ".mp3", ".m4a", ".aac", ".flac", ".ogg", ".oga", ".opus", ".wav":
		return true
	}
	return false
}

func parseInt(v interface{}) (int, bool) {
	switch t := v.(type) {
	case float64:
		return int(t), true
	case int:
		return t, true
	default:
		return 0, false
	}
}

func GetMessageTime(message Message) int64 {
	if message.Time != nil {
		return *message.Time
	}
	return 0
}

func SanitizeFilename(name string) string {
	safe := ""
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-' || r == ' ' {
			safe += string(r)
		}
	}
	for len(safe) > 0 && safe[len(safe)-1] == '.' {
		safe = safe[:len(safe)-1]
	}
	return strings.TrimSpace(safe)
}

type MediaFile struct {
	Name     string
	Size     int64
	TooLarge bool
	path     string
	stream   func() (io.ReadCloser, error)
}

func (m *MediaFile) Open() (io.ReadCloser, error) {
	if m.path != "" {
		return os.Open(m.path)
	}
	return m.stream()
}

func mediaTimeout(sizeLimit int64) time.Duration {
	return 10*time.Minute + time.Duration(sizeLimit/(256*1024))*time.Second
}

func FormatFileSize(size int64) string {
	if size >= 1024*1024*1024 {
		return fmt.Sprintf("%.1fГБ", float64(size)/1024/1024/1024)
	}
	return fmt.Sprintf("%.1fМБ", float64(size)/1024/1024)
}

type mediaSource struct {
	kind      string
	id        int
	url       string
	headers   string
	filePath  string
	knownSize int64
	sizeLimit int64
	userAgent string
	proxyCfg  *ProxyConfig
}

func (s mediaSource) client() *http.Client {
	client, err := BuildHTTPClientWithProxy(s.proxyCfg, mediaTimeout(s.sizeLimit))
	if err != nil {
		Logf("Failed to configure proxy for %s download %d: %v", s.kind, s.id, err)
		client = &http.Client{Timeout: mediaTimeout(s.sizeLimit)}
	}
	return client
}

func (s mediaSource) request(client *http.Client) (*http.Response, error) {
	req, err := http.NewRequest("GET", s.url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept-Encoding", "identity")
	for _, line := range strings.Split(s.headers, "\n") {
		if parts := strings.SplitN(line, ":", 2); len(parts) == 2 {
			req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return resp, nil
}

func (s mediaSource) open() (io.ReadCloser, error) {
	resp, err := s.request(s.client())
	if err != nil {
		return nil, fmt.Errorf("failed to open %s %d: %w", s.kind, s.id, err)
	}
	return resp.Body, nil
}

func responseSize(resp *http.Response) int64 {
	if contentRange := resp.Header.Get("Content-Range"); contentRange != "" {
		if i := strings.LastIndex(contentRange, "/"); i != -1 {
			if total, err := strconv.ParseInt(contentRange[i+1:], 10, 64); err == nil {
				return total
			}
		}
	}
	return resp.ContentLength
}

func fetchMedia(source mediaSource, maxRetries int, retryDelay time.Duration, saveMedia bool) *MediaFile {
	name := filepath.Base(source.filePath)
	client := source.client()

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			delay := retryDelay * time.Duration(1<<uint(attempt-1))
			Logf("Retrying %s download %d (attempt %d/%d) after %v: %v", source.kind, source.id, attempt+1, maxRetries, delay, lastErr)
			time.Sleep(delay)
		}

		resp, err := source.request(client)
		if err != nil {
			lastErr = err
			continue
		}

		size := responseSize(resp)
		if size < 0 && source.knownSize > 0 {
			size = source.knownSize
		}

		if size > source.sizeLimit {
			resp.Body.Close()
			Logf("The %s %d is too large for Telegram (%d bytes), skipping", source.kind, source.id, size)
			return &MediaFile{Name: name, Size: size, TooLarge: true}
		}

		if !saveMedia {
			resp.Body.Close()
			Logf("The %s %d will be sent to Telegram without saving to disk (%d bytes)", source.kind, source.id, size)
			return &MediaFile{Name: name, Size: size, stream: source.open}
		}

		file, err := os.Create(source.filePath)
		if err != nil {
			resp.Body.Close()
			lastErr = err
			continue
		}

		written, err := io.Copy(file, io.LimitReader(resp.Body, source.sizeLimit+1))
		file.Close()
		if err != nil {
			resp.Body.Close()
			lastErr = err
			os.Remove(source.filePath)
			continue
		}

		if written > source.sizeLimit {
			os.Remove(source.filePath)
			rest, _ := io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			size = written + rest
			Logf("The %s %d is too large for Telegram (%d bytes), skipping", source.kind, source.id, size)
			return &MediaFile{Name: name, Size: size, TooLarge: true}
		}
		resp.Body.Close()

		Logf("The %s %d downloaded: %s", source.kind, source.id, source.filePath)
		return &MediaFile{Name: name, Size: written, path: source.filePath}
	}

	Logf("Failed to download %s %d after %d attempts: %v", source.kind, source.id, maxRetries, lastErr)
	return nil
}

func DownloadPhoto(baseURL, photoToken string, photoID int, downloadPath string, userAgent string, proxyCfg *ProxyConfig, maxRetries int, retryDelay time.Duration, saveMedia bool, sizeLimit int64) *MediaFile {
	return fetchMedia(mediaSource{
		kind:      "photo",
		id:        photoID,
		url:       fmt.Sprintf("%s&sig=%s", baseURL, photoToken),
		filePath:  filepath.Join(downloadPath, "images", fmt.Sprintf("%d.webp", photoID)),
		sizeLimit: sizeLimit,
		userAgent: userAgent,
		proxyCfg:  proxyCfg,
	}, maxRetries, retryDelay, saveMedia)
}

func DownloadVideo(urlStr string, videoID int, downloadPath string, videoHeaders string, userAgent string, proxyCfg *ProxyConfig, maxRetries int, retryDelay time.Duration, saveMedia bool, sizeLimit int64) *MediaFile {
	return fetchMedia(mediaSource{
		kind:      "video",
		id:        videoID,
		url:       urlStr,
		headers:   videoHeaders,
		filePath:  filepath.Join(downloadPath, "videos", fmt.Sprintf("%d.mp4", videoID)),
		sizeLimit: sizeLimit,
		userAgent: userAgent,
		proxyCfg:  proxyCfg,
	}, maxRetries, retryDelay, saveMedia)
}

func DownloadFile(urlStr string, fileID int, fileName string, fileSize int, downloadPath string, userAgent string, proxyCfg *ProxyConfig, maxRetries int, retryDelay time.Duration, saveMedia bool, sizeLimit int64) *MediaFile {
	safeName := SanitizeFilename(fileName)
	if safeName == "" {
		safeName = fmt.Sprintf("file-%d", fileID)
	}

	return fetchMedia(mediaSource{
		kind:      "file",
		id:        fileID,
		url:       urlStr,
		filePath:  filepath.Join(downloadPath, "files", fmt.Sprintf("%d-%s", fileID, safeName)),
		knownSize: int64(fileSize),
		sizeLimit: sizeLimit,
		userAgent: userAgent,
		proxyCfg:  proxyCfg,
	}, maxRetries, retryDelay, saveMedia)
}

func DownloadAudio(urlStr string, audioID int, downloadPath string, audioHeaders string, userAgent string, proxyCfg *ProxyConfig, maxRetries int, retryDelay time.Duration, saveMedia bool, sizeLimit int64) *MediaFile {
	return fetchMedia(mediaSource{
		kind:      "audio",
		id:        audioID,
		url:       urlStr,
		headers:   audioHeaders,
		filePath:  filepath.Join(downloadPath, "audio", fmt.Sprintf("%d.ogg", audioID)),
		sizeLimit: sizeLimit,
		userAgent: userAgent,
		proxyCfg:  proxyCfg,
	}, maxRetries, retryDelay, saveMedia)
}

func CountVisibleCharacters(text string) int {
	count := 0
	inTag := false
	for _, r := range text {
		if r == '<' {
			inTag = true
		} else if r == '>' {
			inTag = false
		} else if !inTag {
			count++
		}
	}
	return count
}

func TruncateMessage(text string, isCaption bool) (string, bool) {
	maxLen := 4096
	if isCaption {
		maxLen = 1024
	}

	visibleLen := CountVisibleCharacters(text)
	if visibleLen <= maxLen {
		return text, false
	}

	targetLen := maxLen - 3
	result := ""
	count := 0
	inTag := false
	var tagBuffer strings.Builder

	for _, r := range text {
		if r == '<' {
			inTag = true
			tagBuffer.Reset()
			tagBuffer.WriteRune(r)
		} else if r == '>' {
			inTag = false
			tagBuffer.WriteRune(r)
			result += tagBuffer.String()
		} else if inTag {
			tagBuffer.WriteRune(r)
		} else {
			if count >= targetLen {
				result += closeOpenTags(result) + "..."
				return result, true
			}
			result += string(r)
			count++
		}
	}

	return result, false
}

func closeOpenTags(text string) string {
	openTags := []string{}
	i := 0
	for i < len(text) {
		if i < len(text)-1 && text[i:i+2] == "</" {
			endIdx := strings.Index(text[i:], ">")
			if endIdx != -1 {
				tagName := strings.TrimSpace(text[i+2 : i+endIdx])
				if len(openTags) > 0 && openTags[len(openTags)-1] == tagName {
					openTags = openTags[:len(openTags)-1]
				}
				i += endIdx + 1
			} else {
				i++
			}
		} else if text[i] == '<' {
			endIdx := strings.Index(text[i:], ">")
			if endIdx != -1 {
				tagContent := text[i+1 : i+endIdx]
				tagName := strings.Fields(tagContent)[0]
				if !strings.HasSuffix(tagContent, "/") && !strings.Contains(tagContent, "/") {
					openTags = append(openTags, tagName)
				}
				i += endIdx + 1
			} else {
				i++
			}
		} else {
			i++
		}
	}

	result := ""
	for i := len(openTags) - 1; i >= 0; i-- {
		result += "</" + openTags[i] + ">"
	}
	return result
}

func CheckAndHandleMessageLength(text string, isCaption bool, truncate bool) (string, bool) {
	if truncate {
		newText, wasTruncated := TruncateMessage(text, isCaption)
		if wasTruncated {
			Logf("Message was truncated to fit Telegram limits")
		}
		return newText, true
	}

	maxLen := 4096
	if isCaption {
		maxLen = 1024
	}

	visibleLen := CountVisibleCharacters(text)
	if visibleLen > maxLen {
		Logf("Message is too long (%d chars, max %d). Skipping send.", visibleLen, maxLen)
		return text, false
	}

	return text, true
}
