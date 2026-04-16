package src

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

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

func DownloadPhoto(baseURL, photoToken string, photoID int, downloadPath string, userAgent string) string {
	urlStr := fmt.Sprintf("%s&sig=%s", baseURL, photoToken)
	filePath := filepath.Join(downloadPath, "images", fmt.Sprintf("%d.webp", photoID))

	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		Logf("Failed to create request for photo %d: %v", photoID, err)
		return ""
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		Logf("Failed to download photo %d: %v", photoID, err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		Logf("Failed to download image: HTTP %d", resp.StatusCode)
		return ""
	}

	file, err := os.Create(filePath)
	if err != nil {
		Logf("Failed to create file for photo %d: %v", photoID, err)
		return ""
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		Logf("Failed to save photo %d: %v", photoID, err)
		return ""
	}

	Logf("Image downloaded: %s", filePath)
	return filePath
}

func DownloadVideo(urlStr string, videoID int, downloadPath string, videoHeaders string, userAgent string) string {
	filePath := filepath.Join(downloadPath, "videos", fmt.Sprintf("%d.mp4", videoID))

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		Logf("Failed to parse video URL %d: %v", videoID, err)
		return ""
	}

	client := &http.Client{Timeout: 120 * time.Second}
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		Logf("Failed to create request for video %d: %v", videoID, err)
		return ""
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Host", parsedURL.Host)
	headers := videoHeaders
	for _, line := range strings.Split(headers, "\n") {
		if parts := strings.SplitN(line, ":", 2); len(parts) == 2 {
			req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		Logf("Failed to download video %d: %v", videoID, err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		Logf("Failed to download video: HTTP %d", resp.StatusCode)
		return ""
	}

	file, err := os.Create(filePath)
	if err != nil {
		Logf("Failed to create file for video %d: %v", videoID, err)
		return ""
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		Logf("Failed to save video %d: %v", videoID, err)
		return ""
	}

	Logf("Video downloaded: %s", filePath)
	return filePath
}

func DownloadFile(urlStr string, fileID int, fileName string, downloadPath string, userAgent string) string {
	safeName := SanitizeFilename(fileName)
	if safeName == "" {
		safeName = fmt.Sprintf("file-%d", fileID)
	}
	filePath := filepath.Join(downloadPath, "files", fmt.Sprintf("%d-%s", fileID, safeName))

	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		Logf("Failed to create request for file %d: %v", fileID, err)
		return ""
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		Logf("Failed to download file %d: %v", fileID, err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		Logf("Failed to download file: HTTP %d", resp.StatusCode)
		return ""
	}

	file, err := os.Create(filePath)
	if err != nil {
		Logf("Failed to create file for file %d: %v", fileID, err)
		return ""
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		Logf("Failed to save file %d: %v", fileID, err)
		return ""
	}

	Logf("File downloaded: %s", filePath)
	return filePath
}

func DownloadAudio(urlStr string, audioID int, downloadPath string, audioHeaders string, userAgent string) string {
	filePath := filepath.Join(downloadPath, "audio", fmt.Sprintf("%d.mp3", audioID))

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		Logf("Failed to parse audio URL %d: %v", audioID, err)
		return ""
	}

	client := &http.Client{Timeout: 120 * time.Second}
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		Logf("Failed to create request for audio %d: %v", audioID, err)
		return ""
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Host", parsedURL.Host)
	headers := audioHeaders
	for _, line := range strings.Split(headers, "\n") {
		if parts := strings.SplitN(line, ":", 2); len(parts) == 2 {
			req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		Logf("Failed to download audio %d: %v", audioID, err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		Logf("Failed to download audio: HTTP %d", resp.StatusCode)
		return ""
	}

	file, err := os.Create(filePath)
	if err != nil {
		Logf("Failed to create file for audio %d: %v", audioID, err)
		return ""
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		Logf("Failed to save audio %d: %v", audioID, err)
		return ""
	}

	Logf("Audio downloaded: %s", filePath)
	return filePath
}