# GeoLog contributor instructions

Use the checked-in [MapUp assessment README](docs/BRIEF.md) as the contract for routes, field names, payloads, and submission requirements. Read this local copy each session and skip re-fetching it from GitHub. Follow [DECISIONS.md](DECISIONS.md) for the approved implementation details and scope. Implementation starts only after the user gives the go-ahead.

## Delivery order

1. Deploy the walking skeleton before feature work. Build and push the Go backend image for `linux/amd64` to Docker Hub and deploy that image as one Render web service. Connect it privately to Render Postgres in the same region. The first startup migration runs `CREATE EXTENSION IF NOT EXISTS postgis`. In the first half hour of implementation, check PostGIS support on the selected Render tier and verify extension creation on the actual database. Deploy a placeholder React page to Vercel and verify from the browser that it can call the Go readiness endpoint. The endpoint checks database connectivity.
2. Build the schema and geofence and vehicle endpoints. Then implement location updates and crossing history, followed by rule matching and live delivery, then the React dashboard. Finish the full Compose setup, feature deployment, `SETUP.md`, and video. Verify each stage.
3. Use small, descriptive commits by feature. Keep the submission repo private, with separate `backend/` and `frontend/` directories and the Compose file and documentation at the root.

## Priorities if time runs short

Cut in this order and record each actual cut in DECISIONS.md as a known gap.

1. Outage gap notice in the feed.
2. History filters in the URL.
3. **No alerts configured** badge.
4. **Last updated** indicator.
5. Graceful queue drain on shutdown.

Always ship the nine endpoints and their documented shapes, `time_ns`, live alerts in two tabs, a map with polygons and markers, history filters and pagination, fresh-clone Compose setup, and the deployed app.

## Implementation conventions

- Use Go `net/http`, pgx v5, Gorilla WebSocket, and versioned SQL embedded with golang-migrate. Use React, TypeScript, Vite, CSS Modules, TanStack Query, Leaflet, Leaflet-Geoman, and attributed OpenStreetMap tiles. Verify current library versions and APIs in official documentation before writing against them.
- Use `cmd/server/main.go` for wiring and shutdown. Under `internal/`, keep HTTP in `httpapi`, use cases and transaction ownership in `service`, SQL in `store`, pure logic in `domain`, ID conversion in `ids`, and the hub and socket pumps in `realtime`. Put SQL migrations in `migrations/`. Use raw SQL and keep interfaces small and tied to concrete substitution needs.
- Preserve every documented REST route and JSON shape. Measure `time_ns` from the start of each handler and encode it as a string in every JSON response, including errors. Use the response helper for consistent output.
- Keep alert persistence in the location transaction. Publish after successful commit through the bounded best-effort hub. Keep the WebSocket payload in the brief's format.
- Use one frontend API module and one set of query key definitions. Keep server data in TanStack Query and transient interaction and socket state in React.
- Commit `backend/.env.example` and `frontend/.env.example` with example values. Never commit real credentials. Keep secrets on the backend. Set the frontend API base URL at build time. Read the HTTP and WebSocket origin policy from the environment, accepting explicit origins or `*`. Accept WebSocket upgrades without an Origin header.

## Verification and submission

- At the end of each delivery stage, run the tests and give the user a brief report listing the files added, the request flow through them, and any deviations from DECISIONS.md. These reports support the code walkthrough.

- Unit-test structural validation, membership diffs, rule predicates, and public ID helpers. Test SQL, containment, location transactions, and endpoint contracts with real PostGIS. Run hub tests with `go test -race`. See DECISIONS.md for required cases.
- Verify a fresh `docker compose up` and the deployed outside → inside → still inside → outside flow in two tabs. Check marker and membership state, one entry and one exit alert, filtered history, and reviewer access.
- Provide `SETUP.md` with prerequisites, local and Compose setup, curl examples, and frontend usage. Push the backend image, deploy both apps, invite all nine collaborators listed in the brief, add the 30–45 minute video link to the README, and submit the emailed Google Form. Use original work and synthetic demo data.
