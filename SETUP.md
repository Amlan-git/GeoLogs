# GeoLog setup

## Prerequisites

- Docker Engine or Docker Desktop with Docker Compose v2 for the container setup. The PostGIS image uses `linux/amd64`, so enable amd64 emulation if your Docker host is ARM.
- Go 1.27.4, Node.js 24.12.0 with its bundled npm 11.6.2, and PostgreSQL 17 with PostGIS 3.5 available for the setup without Docker.
- Bash, curl, and Python 3 for the smoke test. Node.js and npm are also needed to run the `wscat` example.

## Run with Docker Compose

From the repository root:

```sh
docker compose up --build
```

Open [the dashboard](http://localhost:5173). The API is at [localhost:8080](http://localhost:8080/healthz). Compose waits for PostgreSQL to accept TCP connections before starting Go. The Go server runs migrations and checks PostGIS before listening. The first build downloads the base images and dependencies.

From another terminal, run `bash scripts/smoke.sh http://localhost:8080` once against a fresh database. The script creates synthetic fixtures and checks that an outside → inside → still inside → outside sequence produces one entry and one exit. Run `docker compose down` to stop the stack and keep its database. Run `docker compose down -v` in a disposable test checkout to delete that checkout's Compose database.

## Run locally without Docker

On a local PostgreSQL server with the PostGIS package installed, use an administrative role to create the local database and enable the extension:

```sql
CREATE ROLE geolog LOGIN PASSWORD 'geolog';
CREATE DATABASE geolog OWNER geolog;
```

Connect to the `geolog` database as an administrator and run `CREATE EXTENSION IF NOT EXISTS postgis;`. The server applies the remaining migrations when it starts. In one terminal, from the repository root:

```sh
cd backend
export DATABASE_URL='postgres://geolog:geolog@localhost:5432/geolog?sslmode=disable'
export PORT=8080
export ALLOWED_ORIGINS='http://localhost:5173,http://127.0.0.1:5173'
go run ./cmd/server
```

In another terminal, from the repository root:

```sh
cd frontend
cp .env.example .env.local
npm ci
npm run dev -- --port 5173 --strictPort
```

Open [localhost:5173](http://localhost:5173). A successful request to [the readiness endpoint](http://localhost:8080/healthz) returns `status: ok` and a string `time_ns`.

## API testing

Run the following Bash examples against a disposable database. The examples capture the generated IDs with Python 3. Run the smoke script before these examples if using the same database, since the script expects empty geofence and vehicle lists.

```bash
api=http://localhost:8080
json_id() { python3 -c 'import json,sys; print(json.load(sys.stdin)[sys.argv[1]])' "$1"; }

# POST /geofences
zone=$(curl -fsS "$api/geofences" -H 'Content-Type: application/json' -d '{
  "name":"Demo restricted zone","description":"Synthetic demo","category":"restricted_zone",
  "coordinates":[[37.77,-122.42],[37.78,-122.42],[37.78,-122.41],[37.77,-122.41],[37.77,-122.42]]
}' | json_id id)

# GET /geofences
curl -fsS "$api/geofences?category=restricted_zone"

# POST /vehicles
vehicle=$(curl -fsS "$api/vehicles" -H 'Content-Type: application/json' -d "{
  \"vehicle_number\":\"DEMO-$(date +%s)\",\"driver_name\":\"Demo Driver\",
  \"vehicle_type\":\"truck\",\"phone\":\"+15550100100\"
}" | json_id id)

# GET /vehicles
curl -fsS "$api/vehicles"

# POST /alerts/configure
rule=$(curl -fsS "$api/alerts/configure" -H 'Content-Type: application/json' \
  -d "{\"geofence_id\":\"$zone\",\"vehicle_id\":\"$vehicle\",\"event_type\":\"both\"}" | json_id alert_id)

# GET /alerts
curl -fsS "$api/alerts?geofence_id=$zone"
```

In a separate terminal, connect before posting the locations:

```sh
npx --yes wscat -c ws://localhost:8080/ws/alerts
```

Back in the original terminal:

```bash
# POST /vehicles/location
post_location() {
  curl -fsS "$api/vehicles/location" -H 'Content-Type: application/json' \
    -d "{\"vehicle_id\":\"$vehicle\",\"latitude\":$1,\"longitude\":$2,\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}"
}
post_location 37.76 -122.42
post_location 37.775 -122.415
post_location 37.776 -122.414
post_location 37.76 -122.42

# GET /vehicles/location/{vehicle_id}
curl -fsS "$api/vehicles/location/$vehicle"

# GET /violations/history
curl -fsS -G "$api/violations/history" \
  --data-urlencode "vehicle_id=$vehicle" --data-urlencode "geofence_id=$zone" \
  --data-urlencode 'limit=50' --data-urlencode 'offset=0'

# PATCH /alerts/{alert_id}
curl -fsS -X PATCH "$api/alerts/$rule" -H 'Content-Type: application/json' \
  -d '{"status":"inactive"}'
```

The socket receives entry and exit messages for the matching rule. History includes both crossings. To narrow history by time, add `start_date` or `end_date` as RFC 3339 query parameters. Use `{"status":"active"}` to enable the rule again.

## Dashboard usage

1. Draw a boundary by clicking vertices on the map and clicking the first point to close it. Name the zone, choose its category, and save. Restricted zones offer a default entry-and-exit rule for all vehicles.
2. Register a vehicle. Pick coordinates on the map or enter them manually. Leave the timestamp blank to use the current time. Submit an outside point, an inside point, another inside point, and an outside point.
3. Use Alert rules to create or toggle rules. Open a second tab and wait for **Live** in both tabs before sending a crossing. The feed and toast show matching events.
4. Filter Violation history by vehicle, geofence, or UTC date and time. Use **Previous** and **Next** to page through results.

## Architecture

```mermaid
flowchart LR
  Browser["React dashboard"]
  API["Go HTTP API"]
  Service["Location service"]
  DB["PostgreSQL + PostGIS"]
  Hub["WebSocket hub"]
  Browser -->|REST| API
  API --> Service
  Service -->|Location transaction| DB
  Service -->|Committed alerts| Hub
  Hub -->|WebSocket events| Browser
```

### POST /vehicles/location request flow

1. The handler starts measuring `time_ns` before decoding the payload. The service validates the sample, locks the vehicle row, and appends the GPS sample.
2. An older sample is committed to the log without changing the current location. For a current sample, PostGIS finds covering polygons, and the service diffs them against stored memberships to record entries and exits.
3. The service matches active rules and stores at most one alert per crossing in the same transaction. After commit, it queues the alert for the WebSocket hub.
4. The API returns the current memberships and elapsed time. Connected dashboards receive the event, update the feed, and refresh the vehicle status and history.
