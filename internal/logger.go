package internal

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

func SetupLogger() error {
	appDir := GetAppDir()
	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		if err := os.MkdirAll(appDir, 0700); err != nil {
			return fmt.Errorf("failed to create app directory: %w", err)
		}
	}

	logPath := filepath.Join(appDir, "logs.txt")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Create a multi-writer to write to both stdout and the log file
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	log.Println("--- Logger Initialized ---")
	return nil
}
