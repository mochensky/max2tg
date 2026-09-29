package src

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const downloadCleanupGracePeriod = 10 * time.Minute

type downloadedFile struct {
	path    string
	size    int64
	modTime time.Time
}

func StartDownloadCleanup(cfg *Config) {
	if cfg.DownloadMaxAge <= 0 && cfg.DownloadMaxSizeMB <= 0 {
		return
	}

	Logf("Download cleanup enabled (max age: %v, max size: %d MB, interval: %v)", cfg.DownloadMaxAge, cfg.DownloadMaxSizeMB, cfg.DownloadCleanupInterval)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				Logf("PANIC in download cleanup: %v", r)
			}
		}()

		ticker := time.NewTicker(cfg.DownloadCleanupInterval)
		defer ticker.Stop()

		for {
			CleanupDownloads(cfg)
			<-ticker.C
		}
	}()
}

func CleanupDownloads(cfg *Config) {
	var files []downloadedFile
	var totalSize int64

	err := filepath.WalkDir(cfg.DownloadPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		files = append(files, downloadedFile{path: path, size: info.Size(), modTime: info.ModTime()})
		totalSize += info.Size()
		return nil
	})
	if err != nil {
		Logf("Failed to scan download directory: %v", err)
		return
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})

	maxSize := int64(cfg.DownloadMaxSizeMB) * 1024 * 1024
	now := time.Now()
	removedCount := 0
	var removedSize int64

	for _, file := range files {
		age := now.Sub(file.modTime)
		if age < downloadCleanupGracePeriod {
			break
		}

		expired := cfg.DownloadMaxAge > 0 && age > cfg.DownloadMaxAge
		overLimit := maxSize > 0 && totalSize > maxSize
		if !expired && !overLimit {
			break
		}

		if err := os.Remove(file.path); err != nil {
			Logf("Failed to remove downloaded file %s: %v", file.path, err)
			continue
		}

		totalSize -= file.size
		removedCount++
		removedSize += file.size
	}

	if removedCount > 0 {
		Logf("Download cleanup: removed %d files (%.2f MB), current size: %.2f MB", removedCount, float64(removedSize)/1024/1024, float64(totalSize)/1024/1024)
	}
}
