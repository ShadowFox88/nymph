package github

import (
	"encoding/json"
	"net/http"
)

func ReposHandler(w http.ResponseWriter, r *http.Request) {
	resp := GetCachedResponse()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}
