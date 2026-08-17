package services

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"time"

	_ "modernc.org/sqlite"
)

type Status struct {
	Name      string
	Online    bool
	FetchedAt time.Time
}

const checkTimeout = 3 * time.Second

func ping(client *http.Client, url string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}

	response, err := client.Do(request)
	if err != nil {
		log.Printf("ping failed: %s: %v\n", url, err)
		return false
	}
	defer response.Body.Close()

	return response.StatusCode < 500
}

func checkAll(client *http.Client, services map[string]string) map[string]Status {
	statuses := make(map[string]Status, len(services))

	for name, url := range services {
		statuses[name] = Status{
			Name:      name,
			Online:    ping(client, url),
			FetchedAt: time.Now(),
		}
	}

	return statuses
}

func SaveStatuses(statuses map[string]Status) {
	db, err := sql.Open("sqlite", "/data/data.db")
	if err != nil {
		log.Printf("failed to open database: %v\n", err)
		return
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS service_history (
		servicename TEXT NOT NULL,
		time DATETIME NOT NULL,
		status TEXT NOT NULL CHECK(status IN ('online', 'offline'))
	)`)
	if err != nil {
		log.Printf("failed to create table: %v\n", err)
		return
	}

	tx, err := db.Begin()
	if err != nil {
		log.Printf("failed to begin transaction: %v\n", err)
		return
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO service_history (servicename, time, status) VALUES (?, ?, ?)`)
	if err != nil {
		log.Printf("failed to prepare statement: %v\n", err)
		return
	}
	defer stmt.Close()

	for _, s := range statuses {
		status := "offline"
		if s.Online {
			status = "online"
		}
		_, err := stmt.Exec(s.Name, s.FetchedAt, status)
		if err != nil {
			log.Printf("failed to insert status for %s: %v\n", s.Name, err)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("failed to commit transaction: %v\n", err)
	}
}
