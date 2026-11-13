package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"geolog/internal/httpapi"
	"geolog/internal/ids"
	"geolog/internal/service"
	"geolog/migrations"

	"github.com/jackc/pgx/v5/pgxpool"
)

type locationReply struct {
	Updated  bool            `json:"location_updated"`
	Position json.RawMessage `json:"current_location"`
	Zones    []struct {
		ID string `json:"geofence_id"`
	} `json:"current_geofences"`
	Violations []struct {
		ID        string `json:"id"`
		Type      string `json:"event_type"`
		Timestamp string `json:"timestamp"`
	} `json:"violations"`
	Total int    `json:"total_count"`
	ID    string `json:"id"`
	Time  string `json:"time_ns"`
}

func TestLocationsAndHistoryWithPostGIS(t *testing.T) {
	connection := os.Getenv("GEOLOG_TEST_DATABASE_URL")
	if connection == "" {
		t.Skip("set GEOLOG_TEST_DATABASE_URL to a fresh disposable PostGIS database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connection)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := migrations.Up(*pool.Config().ConnConfig); err != nil {
		t.Fatal(err)
	}

	h := httpapi.NewRouter(pool.Ping, "", service.NewCatalog(pool))
	request := func(t *testing.T, method, path, body string, status int) locationReply {
		t.Helper()
		res := httptest.NewRecorder()
		h.ServeHTTP(res, httptest.NewRequest(method, path, strings.NewReader(body)))
		if res.Code != status {
			t.Errorf("%s %s: got %d want %d: %s", method, path, res.Code, status, res.Body)
		}
		var result locationReply
		if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil || result.Time == "" {
			t.Errorf("invalid response %s: %v", res.Body, err)
		}
		return result
	}
	const ring = "[[37.7749,-122.4194],[37.7849,-122.4194],[37.7849,-122.4094],[37.7749,-122.4094],[37.7749,-122.4194]]"
	zone := func(name string) string {
		return request(t, "POST", "/geofences", fmt.Sprintf(`{"name":%q,"category":"restricted_zone","coordinates":%s}`, name, ring), 201).ID
	}
	vehicle := func(number string) string {
		return request(t, "POST", "/vehicles", fmt.Sprintf(`{"vehicle_number":%q,"driver_name":"Test Driver","vehicle_type":"truck","phone":"+1000"}`, number), 201).ID
	}
	update := func(t *testing.T, id string, lat, lon float64, at string, status int) locationReply {
		t.Helper()
		return request(t, "POST", "/vehicles/location", fmt.Sprintf(`{"vehicle_id":%q,"latitude":%v,"longitude":%v,"timestamp":%q}`, id, lat, lon, at), status)
	}
	current := func(id string) locationReply { return request(t, "GET", "/vehicles/location/"+id, "", 200) }
	history := func(query string) locationReply { return request(t, "GET", "/violations/history"+query, "", 200) }
	logCount := func() int {
		t.Helper()
		var count int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM vehicle_locations").Scan(&count); err != nil {
			t.Fatal(err)
		}
		return count
	}
	const first = "2025-01-15T10:35:00Z"
	const next = "2025-01-15T10:36:00Z"
	const later = "2025-01-15T10:37:00Z"

	mainZone := zone("Stage 3 zone")
	v := vehicle("STAGE-3-MAIN")
	waiting := current(v)
	if string(waiting.Position) != "null" || waiting.Zones == nil || len(waiting.Zones) != 0 {
		t.Fatalf("awaiting vehicle: %+v", waiting)
	}
	if got := history(""); got.Total != 0 || got.Violations == nil {
		t.Fatalf("empty history: %+v", got)
	}
	update(t, v, 0, 0, first, 200)
	vertex := update(t, v, 37.7849, -122.4194, next, 200)
	if !vertex.Updated || len(vertex.Zones) != 1 || vertex.Zones[0].ID != mainZone {
		t.Fatalf("vertex: %+v", vertex)
	}
	update(t, v, 37.7799, -122.4144, next, 200)
	stale := update(t, v, 0, 0, first, 200)
	if stale.Updated || !reflect.DeepEqual(stale.Zones, vertex.Zones) || logCount() != 4 {
		t.Fatalf("stale response: %+v", stale)
	}
	var point struct {
		Latitude  float64
		Timestamp string
	}
	state := current(v)
	if err := json.Unmarshal(state.Position, &point); err != nil || point.Latitude != 37.7799 || point.Timestamp != next || !reflect.DeepEqual(state.Zones, vertex.Zones) {
		t.Fatalf("stale update changed current state: %+v, %s", state, state.Position)
	}
	if got := history("?vehicle_id=" + v); got.Total != 1 {
		t.Fatalf("expected one entry before exit: %+v", got)
	}
	exit := update(t, v, 0, 0, next, 200)
	if !exit.Updated || exit.Zones == nil || len(exit.Zones) != 0 {
		t.Fatalf("equal timestamp exit: %+v", exit)
	}
	sequence := history("?vehicle_id=" + v)
	if sequence.Total != 2 || len(sequence.Violations) != 2 || sequence.Violations[0].Type != "exit" || sequence.Violations[1].Type != "entry" || sequence.Violations[0].Timestamp != next {
		t.Fatalf("crossing sequence: %+v", sequence)
	}

	secondZone := zone("Stage 3 overlap")
	overlap := vehicle("STAGE-3-OVERLAP")
	if got := update(t, overlap, 37.7799, -122.4144, later, 200); len(got.Zones) != 2 {
		t.Fatalf("overlap: %+v", got)
	}
	if got := history("?vehicle_id=" + overlap); got.Total != 2 || len(got.Violations) != 2 || got.Violations[0].Type != "entry" || got.Violations[1].Type != "entry" {
		t.Fatalf("first point inside overlapping zones: %+v", got)
	}

	before := logCount()
	update(t, v, 0, 0, time.Now().Add(6*time.Minute).UTC().Format(time.RFC3339), 400)
	request(t, "POST", "/vehicles/location", fmt.Sprintf(`{"vehicle_id":%q,"longitude":0,"timestamp":%q}`, v, first), 400)
	for _, invalid := range []string{"veh_99999999", "geo_1", "veh_bad"} {
		update(t, invalid, 0, 0, first, 404)
		request(t, "GET", "/vehicles/location/"+invalid, "", 404)
	}
	if logCount() != before {
		t.Fatal("invalid updates wrote to the location log")
	}

	concurrent := vehicle("STAGE-3-CONCURRENT")
	var group sync.WaitGroup
	start := make(chan struct{})
	for range 2 {
		group.Go(func() {
			<-start
			update(t, concurrent, 37.7799, -122.4144, later, 200)
		})
	}
	close(start)
	group.Wait()
	if got := current(concurrent); len(got.Zones) != 2 {
		t.Fatalf("concurrent memberships: %+v", got)
	}
	if got := history("?vehicle_id=" + concurrent); got.Total != 2 {
		t.Fatalf("concurrent crossings: %+v", got)
	}
	if logCount() != before+2 {
		t.Fatal("expected both concurrent samples in the log")
	}

	for _, tc := range []struct {
		query       string
		total, size int
	}{
		{"?vehicle_id=" + v, 2, 2},
		{"?geofence_id=" + mainZone, 4, 4},
		{"?geofence_id=" + secondZone, 2, 2},
		{"?start_date=" + next + "&end_date=" + next, 2, 2},
		{"?vehicle_id=" + overlap + "&geofence_id=" + secondZone + "&start_date=" + later + "&end_date=" + later, 1, 1},
		{"?vehicle_id=veh_99999999", 0, 0},
		{"?geofence_id=bad", 0, 0},
		{"?vehicle_id=" + v + "&limit=1&offset=1", 2, 1},
		{"?vehicle_id=" + v + "&offset=2", 2, 0},
	} {
		got := history(tc.query)
		if got.Total != tc.total || got.Violations == nil || len(got.Violations) != tc.size {
			t.Errorf("%s: %+v", tc.query, got)
		}
	}
	page := history("?vehicle_id=" + v + "&limit=1&offset=1")
	if len(page.Violations) != 1 || page.Violations[0].Type != "entry" {
		t.Fatalf("unstable offset ordering: %+v", page)
	}
	for _, query := range []string{"limit=0", "limit=-1", "limit=bad", "offset=-1", "offset=bad", "start_date=bad", "end_date=bad", "start_date=" + later + "&end_date=" + first} {
		request(t, "GET", "/violations/history?"+query, "", 400)
	}

	vehicleKey, _ := ids.ParseVehicle(v)
	zoneKey, _ := ids.ParseGeofence(mainZone)
	if _, err := pool.Exec(ctx, `INSERT INTO violations (vehicle_id,geofence_id,event_type,latitude,longitude,timestamp)
		SELECT $1,$2,'entry',0,0,$3 FROM generate_series(1,501)`, vehicleKey, zoneKey, time.Date(2025, 1, 16, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		query string
		size  int
	}{{"", 50}, {"&limit=1000", 500}, {"&limit=500&offset=500", 3}} {
		got := history("?vehicle_id=" + url.QueryEscape(v) + tc.query)
		if got.Total != 503 || len(got.Violations) != tc.size {
			t.Fatalf("pagination %s: total=%d size=%d", tc.query, got.Total, len(got.Violations))
		}
	}
}
