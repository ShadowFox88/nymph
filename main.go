package main

import (
	"log"
	"net/http"
	"time"

	"nymph/github"
	"nymph/middleware"
	"nymph/services"
)

func main() {
	client := &http.Client{Timeout: 10 * time.Second}

	err := loadConfig()

	if err != nil {
		panic(err)
	}

	go github.StartHourlyFetch(client, config.GithubToken, "ShadowFox88")
	go services.StartMinutelyCheck(client, config.Services)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/github", github.ReposHandler)
	mux.HandleFunc("GET /api/services", services.StatusHandler)
	mux.HandleFunc("GET /api/services/history", services.HistoryHandler)

	handler := middleware.RateLimit(mux)
	handler = middleware.CORS(handler)

	log.Println("listening on :9813")
	log.Fatal(http.ListenAndServe(":9813", handler))
}
