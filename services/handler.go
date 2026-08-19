package services

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type HistoryInterval struct {
	ServiceName string    `json:"servicename"`
	From        time.Time `json:"from"`
	To          time.Time `json:"to"`
	Status      string    `json:"status"`
}

type HistoryResponse struct {
	Intervals  []HistoryInterval `json:"intervals"`
	NextCursor *string           `json:"next_cursor"`
	HasMore    bool              `json:"has_more"`
}

type cacheEntry struct {
	body      []byte
	expiresAt time.Time
}

var (
	responseCache      = make(map[string]cacheEntry)
	responseCacheMutex sync.RWMutex
	cacheTTL           = 30 * time.Second
)

func StatusHandler(w http.ResponseWriter, r *http.Request) {
	statuses := GetStatuses()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(statuses); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

func parseDBTime(s string) time.Time {
	if idx := strings.Index(s, " m=+"); idx != -1 {
		s = s[:idx]
	}
	t, _ := time.Parse("2006-01-02 15:04:05.999999999 -0700 UTC", s)
	return t
}

const (
	defaultLimit = 50
	maxLimit     = 200
)

func HistoryHandler(w http.ResponseWriter, r *http.Request) {
	d := getDB()
	if d == nil {
		http.Error(w, "database not initialized", http.StatusInternalServerError)
		return
	}

	limit := defaultLimit
	if l := r.URL.Query().Get("limit"); l != "" {
		parsed, err := strconv.Atoi(l)
		if err != nil || parsed < 1 {
			http.Error(w, "invalid 'limit' parameter", http.StatusBadRequest)
			return
		}
		if parsed > maxLimit {
			parsed = maxLimit
		}
		limit = parsed
	}

	var cursor *time.Time
	if c := r.URL.Query().Get("cursor"); c != "" {
		decoded, err := base64.StdEncoding.DecodeString(c)
		if err != nil {
			http.Error(w, "invalid 'cursor' parameter", http.StatusBadRequest)
			return
		}
		t, err := time.Parse(time.RFC3339, string(decoded))
		if err != nil {
			http.Error(w, "invalid 'cursor' parameter", http.StatusBadRequest)
			return
		}
		cursor = &t
	}

	cacheKey := buildCacheKey(r.URL.Query(), limit)

	responseCacheMutex.RLock()
	entry, ok := responseCache[cacheKey]
	responseCacheMutex.RUnlock()
	if ok && time.Now().Before(entry.expiresAt) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=30")
		w.Header().Set("X-Cache", "HIT")
		etag := fmt.Sprintf(`"%s"`, sha256hex(entry.body))
		w.Header().Set("ETag", etag)
		if match := r.Header.Get("If-None-Match"); match == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Write(entry.body)
		return
	}

	query := `
		SELECT servicename, MIN(time), MAX(time), status
		FROM (
			SELECT servicename, time, status,
				SUM(CASE WHEN status != prev_status OR prev_status IS NULL THEN 1 ELSE 0 END)
					OVER (PARTITION BY servicename ORDER BY time) AS grp
			FROM (
				SELECT servicename, time, status,
					LAG(status) OVER (PARTITION BY servicename ORDER BY time) AS prev_status
				FROM service_history
				WHERE 1=1`
	var args []any

	if service := r.URL.Query().Get("service"); service != "" {
		query += ` AND servicename = ?`
		args = append(args, service)
	}

	if since := r.URL.Query().Get("since"); since != "" {
		t, err := time.Parse(time.RFC3339, since)
		if err != nil {
			http.Error(w, "invalid 'since' parameter, use RFC3339 format", http.StatusBadRequest)
			return
		}
		query += ` AND time >= ?`
		args = append(args, t)
	}

	if until := r.URL.Query().Get("until"); until != "" {
		t, err := time.Parse(time.RFC3339, until)
		if err != nil {
			http.Error(w, "invalid 'until' parameter, use RFC3339 format", http.StatusBadRequest)
			return
		}
		query += ` AND time <= ?`
		args = append(args, t)
	}

	query += `
			)
		)
		GROUP BY servicename, grp, status`

	if cursor != nil {
		query += ` HAVING MIN(time) < ?`
		args = append(args, cursor.Format("2006-01-02 15:04:05.999999999 -0700 UTC"))
	}

	query += ` ORDER BY MIN(time) DESC LIMIT ?`
	args = append(args, limit+1)

	rows, err := d.Query(query, args...)
	if err != nil {
		http.Error(w, "failed to query history", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var intervals []HistoryInterval
	for rows.Next() {
		var interval HistoryInterval
		var fromStr, toStr string
		if err := rows.Scan(&interval.ServiceName, &fromStr, &toStr, &interval.Status); err != nil {
			http.Error(w, "failed to scan row", http.StatusInternalServerError)
			return
		}
		interval.From = parseDBTime(fromStr)
		interval.To = parseDBTime(toStr)
		intervals = append(intervals, interval)
	}

	if err := rows.Err(); err != nil {
		http.Error(w, "error iterating rows", http.StatusInternalServerError)
		return
	}

	hasMore := len(intervals) > limit
	if hasMore {
		intervals = intervals[:limit]
	}

	var nextCursor *string
	if hasMore && len(intervals) > 0 {
		c := base64.StdEncoding.EncodeToString([]byte(intervals[len(intervals)-1].From.Format(time.RFC3339)))
		nextCursor = &c
	}

	response := HistoryResponse{
		Intervals:  intervals,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=30")

	body, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	etag := fmt.Sprintf(`"%s"`, sha256hex(body))
	w.Header().Set("ETag", etag)
	w.Header().Set("X-Cache", "MISS")

	responseCacheMutex.Lock()
	responseCache[cacheKey] = cacheEntry{body: body, expiresAt: time.Now().Add(cacheTTL)}
	responseCacheMutex.Unlock()

	if match := r.Header.Get("If-None-Match"); match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Write(body)
}

func buildCacheKey(params map[string][]string, limit int) string {
	h := sha256.New()
	for k, vs := range params {
		h.Write([]byte(k))
		h.Write([]byte{0})
		for _, v := range vs {
			h.Write([]byte(v))
			h.Write([]byte{0})
		}
	}
	fmt.Fprintf(h, "%d", limit)
	return hex.EncodeToString(h.Sum(nil))
}

func sha256hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
