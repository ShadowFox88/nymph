package github

import (
	"log"
	"net/http"
	"sync"
	"time"
)

var (
	cachedResponse      Response
	cachedResponseMutex sync.RWMutex
)

func StartHourlyFetch(client *http.Client, token string, username string) {
	go fetchAndUpdate(client, token, username) // we want to fetch immediately on startup
	ticker := time.NewTicker(1 * time.Hour)

	for range ticker.C {
		go fetchAndUpdate(client, token, username)
	}
}

func fetchAndUpdate(client *http.Client, token string, username string) {
	response, err := FetchUserRepositories(client, token, username, nil)
	if err != nil {
		log.Println("fetch failed:", err)
		return
	}

	cachedResponseMutex.Lock()
	defer cachedResponseMutex.Unlock()
	cachedResponse = response

	log.Printf("cache updated: %d repositories fetched at %s\n", len(response.Repositories), time.Now().Format(time.RFC3339))
}

func GetCachedResponse() Response {
	cachedResponseMutex.RLock()
	defer cachedResponseMutex.RUnlock()
	return cachedResponse
}
