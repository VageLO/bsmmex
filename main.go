package main

import (
	"log"
	"strings"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

type WatcherFunc func(err error) bool

func WatchFolderCreate(folder string, watcherFunc WatcherFunc) {
    // Create new watcher.
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        log.Fatal(err)
    }
    defer watcher.Close()

	done := make(chan bool)

    // Start listening for events.
    go func() {
        for {
            select {
            case event, ok := <-watcher.Events:
                if !ok {
                    return
                }
                log.Println("event:", event)
                if event.Has(fsnotify.Create) {
                    log.Println("created file:", event.Name)

					ext := filepath.Ext(event.Name)
					if !strings.EqualFold(ext, ".pdf") {
                		log.Println("error:", "file extension is not .pdf")
					}

					if !watcherFunc(nil) {
                        close(done)
                    }
                }
            case err, ok := <-watcher.Errors:
				if !ok {
                    return
                }
				if !watcherFunc(nil) {
                	close(done)
                }
                log.Println("error:", err)
            }
        }
    }()
	log.Printf("Start watching folder %s", folder)

    // Add a path.
    err = watcher.Add(folder)
    if err != nil {
		log.Println("error:", err)
    }

    <-done
}
