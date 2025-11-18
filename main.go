package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"log/syslog"
	"os"
	"path/filepath"
	"strings"
	"context"

	"github.com/VageLO/bsparse/parse"
	"github.com/fsnotify/fsnotify"
	_ "github.com/mattn/go-sqlite3"
	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()

var syslogLogger *log.Logger

var mmexFile string

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

			//log.Println("EVENT:", event)

			if event.Has(fsnotify.Create) {
				ext := filepath.Ext(event.Name)
				if !strings.EqualFold(ext, ".pdf") {
					//log.Println("WARNING:", "file extension is not .pdf")
					continue
				}

				syslogLogger.Printf("Processing file %s", event.Name)
				transactions := bsparse.GetTransactions(event.Name)

				uploadToMMEX(transactions)

				//syslogLogger.Println("Saving csv file")
				//bsparse.Csv(transactions)
			}
		case err, ok := <-w.Errors:
			if !ok {
				syslogLogger.Println("ERROR:", err)
				return
			}
		}
	}
}

func uploadToMMEX(transactions bsparse.TransactionSlice) {

	db, err := sql.Open("sqlite3", mmexFile)
	if err != nil {
		syslogLogger.Panicln(err)
	}
	defer db.Close()

	// name for category and payee
	name := "Unknown"

	selectAccountID := "SELECT ACCOUNTID FROM ACCOUNTLIST_V1 WHERE ACCOUNTNAME = ?"

	selectCategoryID := "SELECT CATEGID FROM CATEGORY_V1 WHERE CATEGNAME = ?"
	insertCategory := "INSERT INTO CATEGORY_V1 (CATEGNAME) VALUES (?)"

	selectPayeeID := "SELECT PAYEEID FROM PAYEE_V1 WHERE PAYEENAME = ?"
	insertPayee := "INSERT INTO PAYEE_V1 (PAYEENAME) VALUES (?)"

	accountId, err := getId(db, selectAccountID, "", "AlfaBank")
	if err != nil {
        syslogLogger.Println(err)
        return
    }

	categoryId, err := getId(db, selectCategoryID, insertCategory, name)
	if err != nil {
        syslogLogger.Println(err)
        return
    }

	payeeId, err := getId(db, selectPayeeID, insertPayee, name)
	if err != nil {
        syslogLogger.Println(err)
        return
    }

	insertTransaction := `
		INSERT INTO CHECKINGACCOUNT_V1 (
		ACCOUNTID, TOACCOUNTID, PAYEEID, TRANSCODE, TRANSAMOUNT,
		STATUS, TRANSACTIONNUMBER, NOTES, CATEGID, TRANSDATE
		) VALUES (?, -1, ?, ?, ?, '', '', ?, ?, ?)
	`

	// Create Redis client
	rdb := redis.NewClient(&redis.Options{
        Addr:     "localhost:6379",
        Password: "",
        DB:       0,
    })

	for index, transaction := range transactions {
		// Get key from redis
		mmexId, err := rdb.Get(ctx, transaction.Id).Result()
		if err == redis.Nil {
			syslogLogger.Printf("%s does not exist in redis", transaction.Id)
		} else if err != nil {
			syslogLogger.Panicln(err)
		} else {
			syslogLogger.Printf("Skipping %s transaction because it's already exist, ID=%s", transaction.Id, mmexId)
			continue
		}

		syslogLogger.Printf("Inserting %d transaction: %s", index+1, transaction.Id)

		typeOf := "Withdrawal"

		result, err := db.Exec(
			insertTransaction,
			accountId,
			payeeId,
			typeOf,
			transaction.Price,
			transaction.Description,
			categoryId,
			transaction.Date,
		)
		if err != nil {
			syslogLogger.Println(err)
			return
		}

		insertedId, err := result.LastInsertId()
		if err != nil {
			syslogLogger.Println(err)
			return
		}
		syslogLogger.Printf("Inserted ID - %d in mmex database", insertedId)

		err = rdb.Set(ctx, transaction.Id, insertedId, 0).Err()
		if err != nil {
			syslogLogger.Panicln(err)
		}
		syslogLogger.Printf("Key %s set with value %d", transaction.Id, insertedId)
	}
}

func getId(db *sql.DB, selectQuery string, insertQuery string, name string) (int64, error) {
	row := db.QueryRow(selectQuery, name);

	var id int64

	err := row.Scan(&id)
	if insertQuery == "" && errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("Account %s not found in MMEX database.", name)
	} else if errors.Is(err, sql.ErrNoRows) {
		return insert(db, insertQuery, name)
	} else if err != nil {
        return 0, err
    }
    return id, nil
}

// Insert category and payee into database
func insert(db *sql.DB, query string, name string) (int64, error) {

	result, err := db.Exec(query, name)
    if err != nil {
		syslogLogger.Println(err)
        return 0, err
    }

	return result.LastInsertId()
}

func main() {
	syslogWriter, err := syslog.New(syslog.LOG_NOTICE|syslog.LOG_DAEMON, "bsmmex")
	if err != nil {
		log.Fatalf("Failed to connect to syslog: %v", err)
	}
	defer syslogWriter.Close()

	// Create a logger that outputs to syslog
	syslogLogger = log.New(syslogWriter, "SYSLOG: ", log.Ldate|log.Ltime)

	bsfolderEnv := "BSFOLDER"
	mmexFileEnv := "MMEXFILE"

	bsfolder, exist := os.LookupEnv(bsfolderEnv)
	if !exist {
		syslogLogger.Fatalf("ENV variable %s is empty", bsfolderEnv)
	}

	mmexFile, exist = os.LookupEnv(mmexFileEnv)
	if !exist {
		syslogLogger.Fatalf("ENV variable %s is empty", mmexFileEnv)
	}

	WatchFolder(bsfolder)
}
