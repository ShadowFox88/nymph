package services

import (
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Outage struct {
	Start      time.Time `json:"start"`
	End        time.Time `json:"end"`
	DurationMs int64     `json:"duration_ms"`
}

type DayOutage struct {
	Date       time.Time `json:"date"`
	Percentage *float64  `json:"percentage"`
	TopOutages []Outage  `json:"top_outages"`
}

type ServiceUptime struct {
	Service string      `json:"service"`
	Days    []DayOutage `json:"days"`
}

type dayStat struct {
	Online  int
	Total   int
	Outages []Outage
}

type dayKey struct {
	service string
	day     string
}

func StatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GetStatuses())
}

func HistoryHandler(w http.ResponseWriter, r *http.Request) {
	d := getDB()
	if d == nil {
		http.Error(w, "database not initialized", http.StatusInternalServerError)
		return
	}

	days := 90
	if ds := r.URL.Query().Get("days"); ds != "" {
		if n, err := strconv.Atoi(ds); err == nil && n > 0 {
			if n < 90 {
				days = n
			}
		}
	}
	service := r.URL.Query().Get("service")

	now := time.Now().UTC()
	start := now.Add(-time.Duration(days) * 24 * time.Hour)

	filter := ""
	if service != "" {
		filter = ` AND servicename = ?`
	}
	args := []any{start, now}
	if service != "" {
		args = append(args, service)
	}

	rows, err := d.Query(`
		SELECT servicename, substr(time, 1, 10) AS day,
			SUM(status = 'offline') AS offline, COUNT(*) AS total
		FROM service_history
		WHERE time >= ? AND time <= ?`+filter+`
		GROUP BY servicename, day`, args...)
	if err != nil {
		http.Error(w, "failed to query uptime", http.StatusInternalServerError)
		return
	}

	stats := make(map[dayKey]*dayStat)
	for rows.Next() {
		var name, day string
		var offline, total int
		if err := rows.Scan(&name, &day, &offline, &total); err != nil {
			rows.Close()
			http.Error(w, "failed to read uptime", http.StatusInternalServerError)
			return
		}
		stats[dayKey{name, day}] = &dayStat{Online: total - offline, Total: total}
	}
	rows.Close()

	rows, err = d.Query(`
		WITH
		numbered AS (
			SELECT servicename, time, status,
				ROW_NUMBER() OVER (PARTITION BY servicename ORDER BY time) AS rn
			FROM service_history
			WHERE time >= ? AND time <= ?`+filter+`
		),
		offline AS (
			SELECT servicename, time, rn,
				rn - ROW_NUMBER() OVER (PARTITION BY servicename ORDER BY time) AS run
			FROM numbered
			WHERE status = 'offline'
		),
		runs AS (
			SELECT servicename, substr(MIN(time), 1, 10) AS day,
				MIN(time) AS start, MAX(time) AS end, COUNT(*) AS minutes
			FROM offline
			GROUP BY servicename, run
		),
		ranked AS (
			SELECT *, ROW_NUMBER() OVER (PARTITION BY servicename, day ORDER BY minutes DESC) AS rn
			FROM runs
		)
		SELECT servicename, day, start, end, minutes
		FROM ranked
		WHERE rn <= 3`, args...)
	if err != nil {
		http.Error(w, "failed to query outages", http.StatusInternalServerError)
		return
	}

	for rows.Next() {
		var name, day, startStr, endStr string
		var minutes int
		if err := rows.Scan(&name, &day, &startStr, &endStr, &minutes); err != nil {
			rows.Close()
			http.Error(w, "failed to read outages", http.StatusInternalServerError)
			return
		}
		key := dayKey{name, day}
		if stats[key] == nil {
			stats[key] = &dayStat{}
		}
		stats[key].Outages = append(stats[key].Outages, Outage{
			Start:      parseDBTime(startStr),
			End:        parseDBTime(endStr),
			DurationMs: int64(minutes) * 60_000,
		})
	}
	rows.Close()

	names := map[string]bool{}
	for k := range stats {
		names[k.service] = true
	}
	sorted := make([]string, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}
	sort.Strings(sorted)

	response := make([]ServiceUptime, 0, len(sorted))
	for _, name := range sorted {
		response = append(response, buildServiceUptime(name, stats, start, now))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func buildServiceUptime(service string, stats map[dayKey]*dayStat, start, end time.Time) ServiceUptime {
	firstDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	numDays := int(end.Sub(start).Hours() / 24)

	days := make([]DayOutage, 0, numDays)
	for i := 0; i < numDays; i++ {
		day := firstDay.Add(time.Duration(i) * 24 * time.Hour)
		st := stats[dayKey{service, day.Format("2006-01-02")}]

		var percentage *float64
		var outages []Outage
		if st != nil && st.Total > 0 {
			pct := round1(float64(st.Online) / float64(st.Total) * 100)
			percentage = &pct
			outages = st.Outages
		}
		days = append(days, DayOutage{Date: day, Percentage: percentage, TopOutages: outages})
	}

	return ServiceUptime{Service: service, Days: days}
}

func round1(f float64) float64 {
	return math.Round(f*1000) / 10
}

func parseDBTime(s string) time.Time {
	if idx := strings.Index(s, " m=+"); idx != -1 {
		s = s[:idx]
	}
	t, _ := time.Parse("2006-01-02 15:04:05.999999999 -0700 UTC", s)
	return t
}
