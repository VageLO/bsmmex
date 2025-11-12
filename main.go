package main

import (
	"log"
	"path/filepath"
	"strings"

	pdf "github.com/VageLO/pdf-parse"
	"github.com/fsnotify/fsnotify"
)

// Watch for CREATE event in specified folder
func WatchFolder(folder string) {

	// Create new watcher.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	defer watcher.Close()

	// Start listening for events.
	go watchLoop(watcher)

	log.Printf("Start watching folder %s", folder)

	// Add a path.
	err = watcher.Add(folder)
	if err != nil {
		log.Println("error:", err)
	}

	<-make(chan struct{})
}

func watchLoop(w *fsnotify.Watcher) {
	for {
		select {
		case event, ok := <-w.Events:
			if !ok {
				return
			}

			log.Println("EVENT:", event)

			if event.Has(fsnotify.Create) {
				ext := filepath.Ext(event.Name)
				if !strings.EqualFold(ext, ".pdf") {
					log.Println("WARNING:", "file extension is not .pdf")
					continue
				}

				log.Printf("Processing file %s", event.Name)
				// TODO: import pdf-parse repo function
				transactions := pdf.GetTransactions(event.Name)
				pdf.Csv(transactions)
			}
		case err, ok := <-w.Errors:
			if !ok {
				log.Println("ERROR:", err)
				return
			}
		}
	}
}

func main() {
	WatchFolder("./")
}
