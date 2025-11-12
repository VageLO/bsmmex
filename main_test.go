package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWatchFolderCreate(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fsnotify-test")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	go func() {
		filePath := filepath.Join(tempDir, "test.pdf")
		file, err := os.Create(filePath)
		t.Logf("Created file: %s", file.Name())
		if err != nil {
			t.Errorf("Error creating file: %v\n", err)
		}

		defer file.Close()
	}()

	WatchFolder(tempDir)
}
