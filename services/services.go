package services

import (
	"context"
	"log"
	"net/http"
	"time"
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
