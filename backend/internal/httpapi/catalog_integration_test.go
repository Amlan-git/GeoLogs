package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"geolog/internal/httpapi"
	"geolog/internal/ids"
	"geolog/internal/service"
	"geolog/migrations"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCatalogWithPostGIS(t *testing.T) {
	url := os.Getenv("GEOLOG_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set GEOLOG_TEST_DATABASE_URL to a fresh disposable PostGIS database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := migrations.Up(*pool.Config().ConnConfig); err != nil {
		t.Fatal(err)
	}
	h := httpapi.NewRouter(pool.Ping, "", service.NewCatalog(pool, nil), nil)
	request := func(method, path, body string, status int) map[string]json.RawMessage {
		t.Helper()
		res := httptest.NewRecorder()
		h.ServeHTTP(res, httptest.NewRequest(method, path, strings.NewReader(body)))
		if res.Code != status {
			t.Fatalf("%s %s = %d, want %d: %s", method, path, res.Code, status, res.Body)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(res.Body.Bytes(), &fields); err != nil {
			t.Fatal(err)
		}
		var elapsed string
		if err := json.Unmarshal(fields["time_ns"], &elapsed); err != nil {
			t.Fatal(err)
		}
		if _, err := strconv.ParseUint(elapsed, 10, 64); err != nil {
			t.Fatal(err)
		}
		if status >= 400 && len(fields["error"]) == 0 {
			t.Fatalf("missing error field: %s", res.Body)
		}
		return fields
	}
	for _, name := range []string{"geofences", "vehicles"} {
		if got := request(http.MethodGet, "/"+name, "", 200)[name]; string(got) != "[]" {
			t.Fatalf("empty %s = %s", name, got)
		}
	}
	const ring = "[[37.7749,-122.4194],[37.7849,-122.4194],[37.7849,-122.4094],[37.7749,-122.4094],[37.7749,-122.4194]]"
	zone := func(coordinates string) string {
		return `{"name":"  Downtown Delivery Zone  ","category":"delivery_zone","coordinates":` + coordinates + "}"
	}
	for _, invalid := range []string{
		"[[0,0],[1,1],[0,1],[1,0],[0,0]]",
		"[[0,0],[1,1],[2,2],[0,0]]",
		"[[0,0],[1,0],[1,1],[0,1]]",
		"[[0,0],[1,0],[0,0]]",
		"[[91,0],[1,0],[1,1],[91,0]]",
		"[[null,0],[1,0],[1,1],[0,0]]",
		"[[0,0,1],[1,0],[1,1],[0,0]]",
		"[[0,0],[1,0],[1,0],[0,0]]",
	} {
		request(http.MethodPost, "/geofences", zone(invalid), 400)
	}
	request(http.MethodPost, "/geofences", zone(ring)+" {}", 400)
	created := request(http.MethodPost, "/geofences", zone(ring), 201)
	defer deleteCatalogFixture(t, pool, "geofence", created["id"])
	if !strings.HasPrefix(string(created["id"]), `"geo_`) || string(created["status"]) != `"active"` || string(created["name"]) != `"Downtown Delivery Zone"` || len(created) != 4 {
		t.Fatalf("invalid geofence response: %v", created)
	}
	list := request(http.MethodGet, "/geofences?category=delivery_zone", "", 200)
	var zones []struct {
		Name        string
		Coordinates json.RawMessage
		CreatedAt   string `json:"created_at"`
	}
	if err := json.Unmarshal(list["geofences"], &zones); err != nil || len(zones) != 1 || zones[0].Name != "Downtown Delivery Zone" || string(zones[0].Coordinates) != ring {
		t.Fatalf("geofence ring did not round-trip: %s (%v)", list["geofences"], err)
	}
	assertWholeSecondUTC(t, zones[0].CreatedAt)
	if got := request(http.MethodGet, "/geofences?category=toll_zone", "", 200)["geofences"]; string(got) != "[]" {
		t.Fatalf("unexpected filtered list: %s", got)
	}
	for _, point := range []struct {
		lat, lon float64
		inside   bool
	}{
		{37.7849, -122.4194, true},
		{37.7799, -122.4194, true},
		{37.7799, -122.4195, false},
		{37.7799, -122.4144, true},
	} {
		var inside bool
		err := pool.QueryRow(ctx, `SELECT ST_Covers(geometry, ST_SetSRID(ST_MakePoint($1, $2), 4326)) FROM geofences`, point.lon, point.lat).Scan(&inside)
		if err != nil || inside != point.inside {
			t.Fatalf("containment for %+v = %v (%v)", point, inside, err)
		}
	}
	vehicle := func(number string) string {
		return `{"vehicle_number":"` + number + `","driver_name":"  John Doe  ","vehicle_type":"  truck  ","phone":"  +1234567890  "}`
	}
	request(http.MethodPost, "/vehicles", `{"vehicle_number":"KA-01"}`, 400)
	request(http.MethodPost, "/vehicles", "{", 400)
	created = request(http.MethodPost, "/vehicles", vehicle("  KA-01  "), 201)
	defer deleteCatalogFixture(t, pool, "vehicle", created["id"])
	if !strings.HasPrefix(string(created["id"]), `"veh_`) || string(created["vehicle_number"]) != `"KA-01"` || len(created) != 4 {
		t.Fatalf("invalid vehicle response: %v", created)
	}
	request(http.MethodPost, "/vehicles", vehicle("ka-01"), 409)
	request(http.MethodPost, "/vehicles", vehicle(`\u00a0ka-01\t`), 409)
	request(http.MethodPost, "/vehicles", vehicle(`KA-02\u0000`), 400)
	list = request(http.MethodGet, "/vehicles", "", 200)
	var vehicles []map[string]json.RawMessage
	if err := json.Unmarshal(list["vehicles"], &vehicles); err != nil || len(vehicles) != 1 ||
		string(vehicles[0]["vehicle_number"]) != `"KA-01"` || string(vehicles[0]["status"]) != `"active"` {
		t.Fatalf("unexpected vehicle list: %s (%v)", list["vehicles"], err)
	}
	for field, want := range map[string]string{
		"driver_name": `"John Doe"`, "vehicle_type": `"truck"`, "phone": `"+1234567890"`,
	} {
		if got := string(vehicles[0][field]); got != want {
			t.Errorf("%s = %s; want %s", field, got, want)
		}
	}
	var createdAt string
	if err := json.Unmarshal(vehicles[0]["created_at"], &createdAt); err != nil {
		t.Fatal(err)
	}
	assertWholeSecondUTC(t, createdAt)
}

func assertWholeSecondUTC(t *testing.T, value string) {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil || parsed.UTC().Format(time.RFC3339) != value {
		t.Fatalf("expected UTC timestamp with whole seconds, got %q", value)
	}
}

func deleteCatalogFixture(t *testing.T, pool *pgxpool.Pool, kind string, raw json.RawMessage) {
	t.Helper()
	var publicID string
	if err := json.Unmarshal(raw, &publicID); err != nil {
		t.Error(err)
		return
	}
	query, parse := "DELETE FROM geofences WHERE id=$1", ids.ParseGeofence
	if kind == "vehicle" {
		query, parse = "DELETE FROM vehicles WHERE id=$1", ids.ParseVehicle
	}
	id, err := parse(publicID)
	if err != nil {
		t.Error(err)
		return
	}
	if _, err := pool.Exec(context.Background(), query, id); err != nil {
		t.Error(err)
	}
}
