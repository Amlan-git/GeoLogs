package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func NewRouter(ping func(context.Context) error, allowedOrigins string) http.Handler {
	router := http.NewServeMux()
	router.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		if err := ping(ctx); err != nil {
			writeJSON(w, started, http.StatusServiceUnavailable, map[string]string{"error": "Database unavailable"})
			return
		}
		writeJSON(w, started, http.StatusOK, map[string]string{"status": "ok"})
	})

	origins := strings.Split(allowedOrigins, ",")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")
		origin := r.Header.Get("Origin")
		for _, entry := range origins {
			allowed := strings.TrimSpace(entry)
			if origin != "" && (allowed == "*" || allowed == origin) {
				w.Header().Set("Access-Control-Allow-Origin", allowed)
				break
			}
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		router.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, started time.Time, status int, body map[string]string) {
	body["time_ns"] = strconv.FormatInt(time.Since(started).Nanoseconds(), 10)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
