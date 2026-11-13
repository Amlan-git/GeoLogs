package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"geolog/internal/domain"
	"geolog/internal/httpapi"
	"geolog/internal/ids"
	"geolog/internal/realtime"
	"geolog/internal/service"
	"geolog/migrations"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
)

type checkingPublisher struct {
	observe func(domain.Alert)
}

func (p checkingPublisher) Publish(alert domain.Alert) { p.observe(alert) }

func TestAlertsWithPostGIS(t *testing.T) {
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

	hub := realtime.NewHub("https://allowed.example")
	defer hub.Close()
	var published []domain.Alert
	publisher := checkingPublisher{observe: func(a domain.Alert) {
		checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		var visible bool
		err := pool.QueryRow(checkCtx, "SELECT EXISTS (SELECT 1 FROM alerts WHERE id=$1 AND violation_id=$2)", a.ID, a.Crossing.ID).Scan(&visible)
		if err != nil || !visible {
			t.Errorf("alert published before commit: visible=%v err=%v", visible, err)
		}
		published = append(published, a)
		hub.Publish(a)
	}}
	router := httpapi.NewRouter(pool.Ping, "https://allowed.example", service.NewCatalog(pool, publisher), hub)
	request := func(method, path, body string, status int) map[string]any {
		t.Helper()
		res := httptest.NewRecorder()
		router.ServeHTTP(res, httptest.NewRequest(method, path, strings.NewReader(body)))
		if res.Code != status {
			t.Fatalf("%s %s = %d, want %d: %s", method, path, res.Code, status, res.Body)
		}
		var data map[string]any
		if err := json.Unmarshal(res.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		elapsed, ok := data["time_ns"].(string)
		if _, err := strconv.ParseUint(elapsed, 10, 64); !ok || err != nil {
			t.Fatalf("invalid time_ns: %v", data)
		}
		return data
	}
	list := func(filter string) []any {
		t.Helper()
		return request("GET", "/alerts"+filter, "", 200)["alerts"].([]any)
	}
	if got := list(""); len(got) != 0 {
		t.Fatal(got)
	}

	const shape = `"coordinates":[[1,1],[1,2],[2,2],[2,1],[1,1]]`
	zone := func(name string) string {
		return request("POST", "/geofences", fmt.Sprintf(`{"name":%q,"category":"restricted_zone",%s}`, name, shape), 201)["id"].(string)
	}
	vehicle := func(number string) string {
		return request("POST", "/vehicles", fmt.Sprintf(`{"vehicle_number":%q,"driver_name":"John Doe","vehicle_type":"truck","phone":"+1000"}`, number), 201)["id"].(string)
	}
	geo := zone("Stage 4 Zone")
	truck, other := vehicle("STAGE-4-TRUCK"), vehicle("STAGE-4-OTHER")
	defer func() {
		key, _ := ids.ParseGeofence(geo)
		one, _ := ids.ParseVehicle(truck)
		two, _ := ids.ParseVehicle(other)
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Error(err)
			return
		}
		defer tx.Rollback(ctx)
		for _, query := range []string{
			"DELETE FROM alerts WHERE violation_id IN (SELECT id FROM violations WHERE geofence_id=$1)",
			"DELETE FROM violations WHERE geofence_id=$1",
			"DELETE FROM alert_rules WHERE geofence_id=$1",
			"DELETE FROM vehicle_geofence_memberships WHERE geofence_id=$1",
			"DELETE FROM geofences WHERE id=$1",
		} {
			if _, err := tx.Exec(ctx, query, key); err != nil {
				t.Error(err)
				return
			}
		}
		if _, err := tx.Exec(ctx, "DELETE FROM vehicle_locations WHERE vehicle_id IN ($1,$2)", one, two); err != nil {
			t.Error(err)
			return
		}
		if _, err := tx.Exec(ctx, "DELETE FROM vehicles WHERE id IN ($1,$2)", one, two); err != nil {
			t.Error(err)
			return
		}
		if err := tx.Commit(ctx); err != nil {
			t.Error(err)
		}
	}()
	zoneKey, _ := ids.ParseGeofence(geo)
	rule := func(z, v, event string, status int) map[string]any {
		body := map[string]string{"geofence_id": z, "event_type": event}
		if v != "" {
			body["vehicle_id"] = v
		}
		data, _ := json.Marshal(body)
		return request("POST", "/alerts/configure", string(data), status)
	}
	toggle := func(id, status string, code int) map[string]any {
		return request("PATCH", "/alerts/"+id, fmt.Sprintf(`{"status":%q}`, status), code)
	}
	rule("geo_99999999", "", "entry", 404)
	rule(geo, "veh_99999999", "entry", 404)
	rule(geo, "", "arrival", 400)
	toggle("alert_99999999", "active", 404)
	toggle("veh_1", "active", 404)

	const stamp = "2025-01-15T10:35:00Z"
	post := func(v string, inside bool, status int) {
		lat, lon := 0.0, 0.0
		if inside {
			lat, lon = 1.5, 1.5
		}
		request("POST", "/vehicles/location", fmt.Sprintf(`{"vehicle_id":%q,"latitude":%v,"longitude":%v,"timestamp":%q}`, v, lat, lon, stamp), status)
	}
	count := func() int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM alerts a JOIN violations v ON v.id=a.violation_id WHERE v.geofence_id=$1`, zoneKey).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	assertEvents := func(from int, expected ...string) {
		t.Helper()
		if count() != len(published) {
			t.Fatalf("stored and published counts differ: %d versus %d", count(), len(published))
		}
		var actual []string
		for _, a := range published[from:] {
			actual = append(actual, a.EventType)
		}
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("events %v, want %v", actual, expected)
		}
	}
	post(truck, true, 200)
	post(truck, false, 200)
	assertEvents(0)

	entry := rule(geo, "", "entry", 201)
	entryID := entry["alert_id"].(string)
	if len(entry) != 6 || entry["vehicle_id"] != nil || entry["geofence_id"] != geo || entry["event_type"] != "entry" || entry["status"] != "active" {
		t.Fatalf("unexpected rule response: %v", entry)
	}
	toggle(entryID, "disabled", 400)
	post(truck, true, 200)
	post(truck, false, 200)
	assertEvents(0, "entry")
	if got := toggle(entryID, "inactive", 200); got["status"] != "inactive" || len(got) != 6 {
		t.Fatal(got)
	}
	exitID := rule(geo, truck, "exit", 201)["alert_id"].(string)
	post(truck, true, 200)
	post(truck, false, 200)
	assertEvents(1, "exit")
	post(other, true, 200)
	post(other, false, 200)
	assertEvents(2)
	if got := list("?geofence_id=" + geo); len(got) != 2 {
		t.Fatal(got)
	} else {
		first, second := got[0].(map[string]any), got[1].(map[string]any)
		if len(first) != 8 || first["status"] != "inactive" || first["vehicle_number"] != nil || first["vehicle_id"] != nil || second["status"] != "active" {
			t.Fatal(got)
		}
	}
	if got := list("?vehicle_id=" + truck); len(got) != 1 || got[0].(map[string]any)["alert_id"] != exitID {
		t.Fatal(got)
	}
	if got := list("?vehicle_id=" + other); len(got) != 0 {
		t.Fatal(got)
	}
	if got := list("?geofence_id=geo_99999999"); len(got) != 0 {
		t.Fatal(got)
	}

	toggle(exitID, "inactive", 200)
	both := rule(geo, "", "both", 201)["alert_id"].(string)
	toggle(entryID, "active", 200)

	server := httptest.NewServer(router)
	defer server.Close()
	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/ws/alerts", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	post(truck, true, 200)
	post(truck, true, 200)
	post(truck, false, 200)
	assertEvents(2, "entry", "exit")
	var matched []int64
	if err := pool.QueryRow(ctx, "SELECT matched_rule_ids FROM alerts WHERE id=$1", published[2].ID).Scan(&matched); err != nil {
		t.Fatal(err)
	}
	entryKey, _ := ids.ParseAlertRule(entryID)
	bothKey, _ := ids.ParseAlertRule(both)
	if !reflect.DeepEqual(matched, []int64{entryKey, bothKey}) {
		t.Fatalf("matched rules: %v", matched)
	}

	if err := ws.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	for _, a := range published[2:] {
		var message map[string]any
		if err := ws.ReadJSON(&message); err != nil {
			t.Fatal(err)
		}
		expected := map[string]any{
			"event_id": ids.FormatEvent(a.ID), "event_type": a.EventType, "timestamp": stamp,
			"vehicle":  map[string]any{"vehicle_id": truck, "vehicle_number": "STAGE-4-TRUCK", "driver_name": "John Doe"},
			"geofence": map[string]any{"geofence_id": geo, "geofence_name": "Stage 4 Zone", "category": "restricted_zone"},
			"location": map[string]any{"latitude": a.Latitude, "longitude": a.Longitude},
		}
		if !reflect.DeepEqual(message, expected) {
			t.Fatalf("WebSocket message: %#v, want %#v", message, expected)
		}
	}

	// A deferred trigger rejects the transaction at COMMIT, after alert insertion.
	if _, err := pool.Exec(ctx, `
		CREATE FUNCTION reject_stage4_alert() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'forced Stage 4 commit failure'; END $$;
		CREATE CONSTRAINT TRIGGER reject_stage4_alert AFTER INSERT ON alerts
		DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_stage4_alert()
	`); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := pool.Exec(ctx, "DROP TRIGGER reject_stage4_alert ON alerts; DROP FUNCTION reject_stage4_alert()"); err != nil {
			t.Error(err)
		}
	}()
	var before int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM vehicle_locations").Scan(&before); err != nil {
		t.Fatal(err)
	}
	beforeHistory := request("GET", "/violations/history?vehicle_id="+truck, "", 200)["total_count"]
	post(truck, true, 500)
	if got := request("GET", "/violations/history?vehicle_id="+truck, "", 200)["total_count"]; got != beforeHistory {
		t.Fatalf("rollback crossing count %v -> %v", beforeHistory, got)
	}
	assertEvents(4)
	var after int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM vehicle_locations").Scan(&after); err != nil || after != before {
		t.Fatalf("rollback location count %d -> %d: %v", before, after, err)
	}
	state := request("GET", "/vehicles/location/"+truck, "", 200)
	if len(state["current_geofences"].([]any)) != 0 || state["current_location"].(map[string]any)["latitude"] != float64(0) {
		t.Fatal(state)
	}
}
