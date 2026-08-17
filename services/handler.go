package services

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type HistoryInterval struct {
	ServiceName string    `json:"servicename"`
	From        time.Time `json:"from"`
	To          time.Time `json:"to"`
	Status      string    `json:"status"`
}

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

func HistoryHandler(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite", "/data/data.db")
	if err != nil {
		http.Error(w, "failed to open database", http.StatusInternalServerError)
		return
	}
	defer db.Close()

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
		GROUP BY servicename, grp, status
		ORDER BY MIN(time) DESC`

	rows, err := db.Query(query, args...)
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

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(intervals); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}
