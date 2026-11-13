package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestHealth(t *testing.T) {
	for _, tc := range []struct {
		name  string
		dbErr error
		code  int
	}{
		{"ready", nil, http.StatusOK},
		{"unavailable", errors.New("database unavailable"), http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewRouter(func(context.Context) error { return tc.dbErr }, "https://geolog.example", nil, nil)
			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			req.Header.Set("Origin", "https://geolog.example")
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			if res.Code != tc.code {
				t.Fatalf("status = %d, want %d", res.Code, tc.code)
			}
			if res.Header().Get("Access-Control-Allow-Origin") != "https://geolog.example" {
				t.Fatal("frontend origin missing")
			}
			var body map[string]string
			if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if _, err := strconv.ParseUint(body["time_ns"], 10, 64); err != nil {
				t.Fatalf("invalid time_ns: %v", body)
			}
			if tc.dbErr == nil && body["status"] != "ok" || tc.dbErr != nil && body["error"] == "" {
				t.Fatalf("unexpected body: %v", body)
			}
		})
	}
}
