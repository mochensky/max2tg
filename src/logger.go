package src

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	logFile     *os.File
	logWriter   io.Writer
	logMutex    sync.Mutex
	logLocation *time.Location
)

func SetupLogger(logPath string, timezone string) error {
	if err := os.MkdirAll(logPath, 0755); err != nil {
		return err
	}

	logLocation = resolveTimezone(timezone)

	now := time.Now().In(logLocation)
	timestamp := now.Format("02.01.2006-15.04.05")
	logFilePath := filepath.Join(logPath, timestamp+".log")

	f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	logFile = f
	logWriter = io.MultiWriter(os.Stdout, f)

	return nil
}

func resolveTimezone(timezone string) *time.Location {
	if timezone == "" {
		return time.Local
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		fmt.Printf("Warning: invalid timezone %q: %v, falling back to UTC\n", timezone, err)
		return time.UTC
	}
	return loc
}

func CloseLogger() {
	if logFile != nil {
		logFile.Close()
	}
}

func Logf(format string, v ...interface{}) {
	logMutex.Lock()
	defer logMutex.Unlock()

	loc := logLocation
	if loc == nil {
		loc = time.Local
	}

	if logWriter == nil {
		fmt.Printf("[???] "+format+"\n", v...)
		return
	}
	timestamp := time.Now().In(loc).Format("[02.01.2006 15:04:05] ")
	msg := fmt.Sprintf(format, v...)
	fmt.Fprint(logWriter, timestamp+msg+"\n")
}
