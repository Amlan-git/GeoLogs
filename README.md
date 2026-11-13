# GeoLog

GeoLog is a fleet geofencing dashboard. Draw zones, register vehicles, submit GPS samples, configure entry and exit alerts, and review crossing history. Go and PostGIS track zone memberships, and WebSockets deliver live alerts to the React dashboard.

- [Dashboard on Vercel](https://geolog-stage1.vercel.app/)
- [Render API readiness](https://geologs-backend-stage1.onrender.com/healthz)
- [Docker Hub backend image `amlanb26/geologs-backend:v1.0.0`](https://hub.docker.com/r/amlanb26/geologs-backend/tags?name=v1.0.0)

The free Render instance can take about a minute to wake.

Video walkthrough: <LINK>

## Run locally

Run `docker compose up --build` from the root and open [localhost:5173](http://localhost:5173). See [SETUP.md](SETUP.md) for prerequisites, setup without Docker, API examples, and dashboard usage.

## Testing

From `backend/`:

```sh
go test -race -count=1 ./...
go vet ./...
```

The race-detector run includes the WebSocket hub tests. Set `GEOLOG_TEST_DATABASE_URL` to a fresh disposable database on a PostgreSQL server with PostGIS available to include the HTTP integration tests. The test connection role must be able to enable PostGIS, or an administrator must enable it beforehand.

From the repository root, with the API running against a separate fresh database:

```sh
bash scripts/smoke.sh http://localhost:8080
npm --prefix frontend ci
npm --prefix frontend run build
```

Set `VITE_API_BASE_URL` at build time when serving the resulting frontend bundle.

## Key design decisions

PostGIS uses `ST_Covers` so polygon edges count as inside. A transaction keeps the current position, zone memberships, crossings, and matching alerts consistent. The WebSocket hub publishes after commit. See [DECISIONS.md](DECISIONS.md) for the full design and tradeoffs.

## Known gaps

- The live feed has no outage-gap notice or replay. Crossing history remains available.
- History filters and pagination live in the dashboard state.
- The geofence list omits the **No alerts configured** badge, and vehicle polling omits a **Last updated** indicator.
- Shutdown closes WebSockets without draining pending live alerts. Committed records remain available.
- The dashboard assumes a single backend instance. Authentication, tenant isolation, and rate limiting are outside the assessment scope.

## Bonus items

- **Implemented:** Unit tests and HTTP integration tests against real PostgreSQL/PostGIS. Both run with Go's race detector using the command above.
- **Deferred:** Authentication and tenant isolation are a separate production concern. The assessment uses synthetic data and a single shared workspace.
- **Deferred:** Rate limiting needs a policy for how frequently real vehicles report and how clients retry. The demo uses manually submitted samples.
- **Deferred:** Pagination on every list endpoint would add complexity for the demo-sized vehicle, geofence, and rule lists. Violation history has filtering and pagination because it grows with each crossing.
