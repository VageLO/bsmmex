package main

import (
	"log"
	"log/syslog"
	"path/filepath"
	"strings"

	"github.com/VageLO/bsparse/parse"
	"github.com/fsnotify/fsnotify"
)

var syslogLogger *log.Logger

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

	syslogLogger.Printf("Start watching folder %s", folder)

	// Add a path.
	err = watcher.Add(folder)
	if err != nil {
		syslogLogger.Println("error:", err)
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

			syslogLogger.Println("EVENT:", event)

			if event.Has(fsnotify.Create) {
				ext := filepath.Ext(event.Name)
				if !strings.EqualFold(ext, ".pdf") {
					syslogLogger.Println("WARNING:", "file extension is not .pdf")
					continue
				}

				syslogLogger.Printf("Processing file %s", event.Name)
				transactions := bsparse.GetTransactions(event.Name)

				syslogLogger.Println("Saving csv file")
				bsparse.Csv(transactions)
			}
		case err, ok := <-w.Errors:
			if !ok {
				syslogLogger.Println("ERROR:", err)
				return
			}
		}
	}
}

func main() {
	syslogWriter, err := syslog.New(syslog.LOG_NOTICE|syslog.LOG_DAEMON, "bsmmex")
	if err != nil {
		log.Fatalf("Failed to connect to syslog: %v", err)
	}
	defer syslogWriter.Close()

	// Create a logger that outputs to syslog
	syslogLogger = log.New(syslogWriter, "SYSLOG: ", log.Ldate|log.Ltime)
	// TODO: refine
	WatchFolder("~/Workflow/statements")
}
