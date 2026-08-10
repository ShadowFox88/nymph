package services

import (
	"log"
	"net/http"
	"sync"
	"time"
)

var (
	cachedStatuses      map[string]Status
	cachedStatusesMutex sync.RWMutex
)

func StartMinutelyCheck(client *http.Client, services map[string]string) {
	go checkAndUpdate(client, services) // we want to check immediately on startup
	ticker := time.NewTicker(1 * time.Minute)

	for range ticker.C {
		go checkAndUpdate(client, services)
	}
}

func checkAndUpdate(client *http.Client, services map[string]string) {
	statuses := checkAll(client, services)

	cachedStatusesMutex.Lock()
	defer cachedStatusesMutex.Unlock()
	cachedStatuses = statuses

	log.Printf("statuses updated: %d services checked at %s\n", len(statuses), time.Now().Format(time.RFC3339))
}

func GetStatuses() map[string]Status {
	cachedStatusesMutex.RLock()
	defer cachedStatusesMutex.RUnlock()
	return cachedStatuses
}
