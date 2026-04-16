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
	logFile   *os.File
	logWriter io.Writer
	logMutex  sync.Mutex
)

func SetupLogger(logPath string) error {
	if err := os.MkdirAll(logPath, 0755); err != nil {
		return err
	}

	timestamp := time.Now().Format("02.01.2006-15.04.05")
	logFilePath := filepath.Join(logPath, timestamp+".log")

	f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	logFile = f
	logWriter = io.MultiWriter(os.Stdout, f)

	return nil
}

func CloseLogger() {
	if logFile != nil {
		logFile.Close()
	}
}

func Logf(format string, v ...interface{}) {
	logMutex.Lock()
	defer logMutex.Unlock()
	if logWriter == nil {
		fmt.Printf("[???] "+format+"\n", v...)
		return
	}
	timestamp := time.Now().Format("[02.01.2006 15:04:05] ")
	msg := fmt.Sprintf(format, v...)
	fmt.Fprint(logWriter, timestamp+msg+"\n")
}
