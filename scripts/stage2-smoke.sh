#!/usr/bin/env bash
# Run against a locally started GeoLog server with a fresh disposable database.
set -euo pipefail
base_url="${1:-http://localhost:8080}"

check() {
    local expected=$1 method=$2 path=$3
    shift 3
    local response status body
    printf '\n%s %s\n' "$method" "$path"
    response=$(curl --silent --show-error --request "$method" --write-out '\n%{http_code}' "$@" "$base_url$path")
    status=${response##*$'\n'}
    body=${response%$'\n'*}
    printf 'HTTP %s\n%s\n' "$status" "$body"
    last_body=$body
    python3 - "$expected" "$status" "$body" "$method" "$path" <<'PY'
import json
import sys

expected, status, body, method, path = sys.argv[1:]
assert status == expected, (status, expected)
data = json.loads(body)
elapsed = data["time_ns"]
assert isinstance(elapsed, str) and elapsed.isdecimal(), data
if method == "GET" and path in ("/geofences", "/vehicles"):
    assert data[path[1:]] == [], data
if path == "/geofences?category=delivery_zone":
    assert len(data["geofences"]) == 1, data
    assert data["geofences"][0]["coordinates"] == [
        [37.7749, -122.4194],
        [37.7849, -122.4194],
        [37.7849, -122.4094],
        [37.7749, -122.4094],
        [37.7749, -122.4194],
    ], data
PY
}

json=(-H 'Content-Type: application/json' --data-binary)
check 200 GET /geofences
check 200 GET /vehicles
check 201 POST /geofences "${json[@]}" @- <<'JSON'
{
  "name": "Downtown Delivery Zone",
  "description": "Main delivery area for downtown customers",
  "coordinates": [
    [37.7749, -122.4194],
    [37.7849, -122.4194],
    [37.7849, -122.4094],
    [37.7749, -122.4094],
    [37.7749, -122.4194]
  ],
  "category": "delivery_zone"
}
JSON
created_geofence=$last_body
check 400 POST /geofences "${json[@]}" '{"name":"Bow-tie","coordinates":[[0,0],[1,1],[0,1],[1,0],[0,0]],"category":"delivery_zone"}'
check 400 POST /geofences "${json[@]}" '{"name":"Unclosed","coordinates":[[0,0],[1,0],[1,1],[0,1]],"category":"delivery_zone"}'
check 200 GET '/geofences?category=delivery_zone'
check 201 POST /vehicles "${json[@]}" @- <<'JSON'
{
  "vehicle_number": "KA-01-AB-1234",
  "driver_name": "John Doe",
  "vehicle_type": "truck",
  "phone": "+1234567890"
}
JSON
created_vehicle=$last_body
check 409 POST /vehicles "${json[@]}" '{"vehicle_number":"ka-01-ab-1234","driver_name":"John Doe","vehicle_type":"truck","phone":"+1234567890"}'
check 400 POST /vehicles "${json[@]}" '{"vehicle_number":"KA-02-AB-1234","driver_name":"John Doe","vehicle_type":"truck"}'
vehicle_id=$(python3 -c 'import json,sys; print(json.loads(sys.argv[1])["id"])' "$created_vehicle")
geofence_id=$(python3 -c 'import json,sys; print(json.loads(sys.argv[1])["id"])' "$created_geofence")
check 201 POST /alerts/configure "${json[@]}" \
    "{\"geofence_id\":\"$geofence_id\",\"event_type\":\"both\"}"
check 200 GET "/vehicles/location/$vehicle_id"
python3 -c 'import json,sys; d=json.loads(sys.argv[1]); assert d["current_location"] is None and d["current_geofences"] == []' "$last_body"

location() {
    check 200 POST /vehicles/location "${json[@]}" \
        "{\"vehicle_id\":\"$vehicle_id\",\"latitude\":$1,\"longitude\":$2,\"timestamp\":\"2025-01-15T10:35:0$3Z\"}"
}
location 0 0 0
location 37.7849 -122.4194 1
location 37.7799 -122.4144 2
location 0 0 3
check 200 GET "/vehicles/location/$vehicle_id"
python3 -c 'import json,sys; d=json.loads(sys.argv[1]); assert d["current_location"]["latitude"] == 0 and d["current_geofences"] == []' "$last_body"
check 200 GET "/violations/history?vehicle_id=$vehicle_id"
python3 -c 'import json,sys; d=json.loads(sys.argv[1]); assert d["total_count"] == 2 and [v["event_type"] for v in d["violations"]] == ["exit", "entry"]' "$last_body"
printf '\nAll responses have a decimal string time_ns.\n'
