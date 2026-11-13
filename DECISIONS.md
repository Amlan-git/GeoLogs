# GeoLog architecture decisions

GeoLog is a fleet operations dashboard for drawing zones, registering vehicles, submitting GPS points, configuring live alerts, and reviewing crossings. The [checked-in MapUp brief](docs/BRIEF.md) defines the external contract. The prep guide informs the build order, verification, and walkthrough.

## API and identifiers

- Preserve the nine specified REST methods, paths, query defaults, and response shapes. The two additive features are `PATCH /alerts/{alert_id}` and optional `offset` on history. Use `GET /healthz` for readiness. Return `200` with `{"status":"ok"}` and string `time_ns` when the database responds. Return `503` with the standard error body when it does not.
- Return `201` for `POST /geofences`, `POST /vehicles`, and `POST /alerts/configure`. Return `200` for location posts, reads, and successful rule toggles. Malformed JSON and invalid inputs return `400`. Duplicate vehicle numbers return `409`. Unexpected failures return `500` with a generic client message and full server logging.
- Start timing on the first line of every handler. One response helper writes elapsed nanoseconds as a string and maps service errors to `{"error":"<plain message>","time_ns":"<string>"}`. All empty arrays serialize as `[]`, including `geofences`, `vehicles`, `alerts`, `violations`, and `current_geofences`.
- Use generated `BIGINT` primary keys and numeric foreign keys. Derive API IDs at the boundary using `geo_`, `veh_`, `alert_`, `viol_`, and `evt_`. Use one helper package with format and parse functions for each entity. Require the correct prefix, digits only, a positive number, and no overflow. Treat malformed and unknown IDs as `404` when identifying a resource and as empty results when used as list filters. An omitted optional rule vehicle ID means all vehicles.
- A registered vehicle without a GPS report returns `200` with `vehicle_id`, `vehicle_number`, `current_location: null`, and `current_geofences: []`. An unknown vehicle returns `404` on either location route, and a location post saves nothing in that case. Keep `GET /vehicles` as specified.

- Geofences and vehicles have fixed status `active` wherever the documented response includes a status field.
- Return timestamps in UTC as RFC 3339 with whole seconds and a `Z` suffix. Use the GPS sample timestamp for the crossing and its WebSocket alert.

## Schema and spatial behavior

- Tables are `geofences`, `vehicles`, `vehicle_locations`, `vehicle_geofence_memberships`, `violations`, `alert_rules`, and `alerts`. Use foreign keys for entity relationships, `TIMESTAMPTZ` for timestamps, and a unique vehicle–geofence pair in memberships.
- Trim surrounding whitespace from geofence names, vehicle numbers, driver names, vehicle types, and phone numbers before storage and return the trimmed values. Preserve vehicle-number casing. Require a nonempty phone and vehicle type. Enforce uniqueness in PostgreSQL on the trimmed, case-insensitive vehicle number.
- Validate geofences in order. Check required fields and one of `delivery_zone`, `restricted_zone`, `toll_zone`, or `customer_area`. Check that each coordinate contains exactly two numbers. Check latitude and longitude ranges, at least four points with the first equal to the last, and at least three distinct vertices. Then check `ST_IsValid` in PostGIS.
- Reject invalid geometry with `400` and a plain-language explanation appropriate to the failure. Log the raw PostGIS reason on the server. Accept either winding order and preserve the shape the dispatcher drew.
- Store the submitted `[lat, lon]` ring in JSONB and return it from `GET /geofences` in the same pair order and ring sequence. Store a separate GiST-indexed `geometry(Polygon, 4326)` for containment. Use `ST_Covers` so edges and vertices count as inside, including the brief's sample vertex. Construct the query point with `ST_SetSRID(ST_MakePoint(longitude, latitude), 4326)`.
- Keep the current position on the vehicle row as nullable `last_latitude`, `last_longitude`, and `last_location_at`, with an all-or-none check. Use `DOUBLE PRECISION` for coordinates and build point geometry for containment as needed. Read position and memberships from one database snapshot.
- The location log is append-only and indexed on `(vehicle_id, recorded_at DESC)` for future replay. The current state has fixed size per vehicle and stays consistent with the log through the location transaction.

## Location transaction and alert rules

1. Validate the request. Distinguish missing latitude, longitude, and timestamp from zero values. Latitude and longitude zero are valid. Parse the timestamp and reject values more than five minutes ahead of server time with `400`.
2. Lock the vehicle row with `SELECT … FOR UPDATE` and read its latest timestamp. Append the sample to the log. If strictly older than the latest timestamp, read the actual current memberships, commit, and return `200` with `location_updated: false`. This sample has no effect on current state, crossings, or alerts. Equal timestamps are processed in lock-acquisition order.
3. For a current sample, update the three current-position columns. Query containing zones once, diff the result against stored membership, and update membership. A vehicle starts with an empty membership set. Its first point inside a zone records entry as first observed inside. A zone drawn around a stationary vehicle is evaluated at that vehicle's next current update.
4. Store every entry and exit in `violations`. In one rule-matching query for the sample's crossings, find active rules by geofence, the specific vehicle or all vehicles, and the crossing type or `both`. Use a partial index on `alert_rules (geofence_id)` for active rules.
5. If one or more rules match a crossing, insert exactly one row into `alerts` with its generated key, `violation_id`, `matched_rule_ids BIGINT[]`, and `created_at`. Enforce `UNIQUE (violation_id)`. Store all matched rule keys for audit. Build the WebSocket payload from the stored crossing and related vehicle and geofence data.
6. Commit the transaction, then hand off each alert to the publisher. Return the documented location response with `location_updated: true` and current memberships. Keep all writes for the sample atomic under the vehicle lock. Publish only committed alerts.

- `POST /alerts/configure` returns `404` for an unknown `geofence_id` or supplied `vehicle_id`, and `400` for an invalid `event_type`. For all-vehicle rules, return `vehicle_id: null` in the create response and both `vehicle_id: null` and `vehicle_number: null` in `GET /alerts`.
- `GET /alerts?vehicle_id=X` returns rules scoped to that specific vehicle only.
- `GET /alerts` lists active and inactive rules in the brief's response shape. `PATCH /alerts/{alert_id}` accepts `{"status":"active"}` or `{"status":"inactive"}` and returns the create response shape with the updated status and `time_ns`. Invalid status is `400`, unknown ID is `404`. The rules list has a switch on each row and visually mutes inactive rules. Keep rule records so stored alerts remain traceable.
- All categories use the same matching logic. For `restricted_zone`, the create form offers a default-on checkbox to configure a `both` rule for all vehicles. It uses a separate `POST /alerts/configure` after saving the zone. If the rule request fails, keep the zone visible and show a persistent, dismissible warning and a one-click Retry for that rule request. A lost success response can lead to duplicate rules on retry. The crossing still yields at most one alert. Use stronger red styling for restricted-zone alerts in the feed and toast.

## History

- Filter by vehicle, geofence, and inclusive crossing timestamps. Parse dates as RFC 3339. Invalid dates and a start after the end return `400`. Unknown or malformed filter IDs yield an empty list with `total_count: 0`.
- Sort by timestamp descending and numeric key descending. Default `limit` to 50 and clamp it at 500. Default `offset` to 0. Reject nonnumeric values, limits below 1, and negative offsets with `400`. An offset past the end returns `[]` and the correct filtered count.
- Run a separate count with the same filters and without pagination, in the same read snapshot as the page. Index history by timestamp and numeric key descending, with additional indexes prefixed by vehicle ID and by geofence ID.
- Show 50 rows per page with Previous and Next and page numbers derived from `total_count`. Reset to page 1 when filters change. Retain the previous page while the next loads.

## Live delivery

- Use one backend instance and one hub goroutine that owns the client registry. Register, unregister, and broadcast through channels. The location service depends on a small publisher interface.
- After commit, enqueue the alert with a non-blocking send to a 256-message queue. Log and drop the live notification if full. Encode the payload once for all clients. Give each client a 32-message buffer and one writer with a write deadline. Disconnect a client whose buffer fills. The database remains the durable record.
- Ping roughly every 30 seconds and drop clients that have gone about 60 seconds without answering. On SIGTERM, stop accepting requests, then send WebSocket close frames. The hub sends only the brief's alert format on `/ws/alerts`.
- Keep one socket per tab. Close the old socket before replacement and on unmount. Reconnect with 1, 2, 4, 8 seconds and increasing delays capped at 30 seconds, with jitter. Reset after connecting. Reconnect immediately when the browser comes online or the tab becomes visible if disconnected. Show Live, Reconnecting, or Offline.
- On reconnect, refresh vehicle statuses and refresh history if open. The live feed resumes with new alerts, deduplicated by `event_id`, and retains the latest 100 alerts. An incoming alert refreshes that vehicle's status and invalidates violations queries.

## Application structure and frontend data

- Handlers decode JSON, convert IDs, call one service method, and encode HTTP responses. Services own use cases and transactions. Store methods execute queries on the supplied pool or transaction. Pure structural validation, membership diffing, and rule predicates live in domain code. Keep CRUD thin, pass `context.Context` through each layer, and add interfaces where substitution is needed.
- Use one `fetch` module with the build-time API URL. Throw on non-2xx responses with the server's error message. Centralize TanStack Query keys for geofences by category, vehicles, status by vehicle ID, rules, and violations by filters and page. Keep map markers and vehicle rows on the same status cache entry.
- Creating a zone refreshes geofences. Creating a vehicle refreshes vehicles. Posting a location refreshes that vehicle's status. Creating or toggling rules refreshes rules. Read requests can retry network failures a small number of times. Writes and `4xx` responses skip automatic retries.
- Keep form drafts, drawing state, vehicle selection, map view, and live feed in React state. One top-level provider owns the socket, status, and feed. Show loading and error states for API operations.
- Poll every 10 seconds while visible. Each cycle reloads vehicles and fetches their statuses in parallel. Refresh immediately on becoming visible, skip ticks during an in-flight cycle, and clean up on unmount. Preserve last good markers on failure, and skip poll error toasts. Preserve unchanged cache references and memoize rows and markers where useful.
- Use a single dashboard with map, vehicles, geofences, rules, and history views. Display **Awaiting first location** with no marker or zone chips before the first report. Fix Leaflet marker asset paths for Vite. Prevent self-intersection in Geoman, close the ring before submission, and allow map clicks or manual coordinates for location updates. Start with an empty database and use synthetic demo data.

## Infrastructure and build order

- Use a private GitHub repository with `backend/`, `frontend/`, and root Compose and documentation files. Compose starts React, Go, and PostgreSQL/PostGIS. The browser uses the published local API port for REST and WebSockets.
- Production uses Vercel for React and a single Render web service pulling a tagged `linux/amd64` Docker Hub image, regardless of the development machine's architecture. Use Render Postgres on the private network in the same region. Choose the Render tier for the review window and budget.
- Listen on `0.0.0.0` using the `PORT` environment variable, with a local default. Limit HTTP request bodies to 1 MB.
- Answer CORS `OPTIONS` preflight requests. Allow `GET`, `POST`, `PATCH`, and `OPTIONS` and the `Content-Type` request header.
- Configure the backend origin allowlist via environment variable. Support explicit local and deployed frontend origins or `*`. Accept WebSocket upgrades with no Origin header for command-line and desktop clients. Keep the database URL and credentials on the backend. Set the frontend API base URL at build time and use HTTPS and WSS in production.
- Apply embedded, versioned SQL migrations using golang-migrate before accepting requests. The first migration enables PostGIS. Deploy and verify the walking skeleton before feature work, including the Vercel browser calling the Render readiness endpoint and successful PostGIS creation on the actual database. Check the selected tier early. Render documents PostGIS for PostgreSQL 13 and later and requires `linux/amd64` images. [Render Postgres extensions](https://render.com/docs/postgresql-extensions) · [Render image requirements](https://render.com/docs/deploying-an-image#image-requirements)
- Then follow the prep guide order: schema and geofence/vehicle endpoints, location and history, rules and live delivery, frontend, Compose/deployment and setup documentation, then video. Commit in small, descriptive steps per feature.

## Verification and walkthrough

- Test IDs with a round trip for each prefix and rejection of wrong prefixes, letters, negative or zero keys, and overflow. Assert empty arrays for each list endpoint and empty membership for a vehicle awaiting location.
- Test polygon rejection for bow-ties, collinear points, unclosed rings, fewer than four points, and out-of-range latitude. Accept the brief's sample polygon and test containment against real PostGIS for its sample vertex, an edge midpoint, a point just outside, and an interior point.
- Use real PostGIS integration tests for store and location transactions. Cover older samples, equal timestamps, initial membership, matching multiple rules, and publication after commit and never on rollback. Run hub tests with `go test -race` for two receivers, a slow client being dropped while another receives, and a disconnect during broadcast.
- Verify the deployed app and a fresh Compose run. In the video, show map-based zone creation, vehicle registration, the outside → inside → still inside → outside sequence, live alerts in two tabs, history filters, and Docker running locally. Walk through the actual code from handler through transaction, commit, and hub, then discuss gaps and scaling. Include the recording link in the README and complete the submission checklist in AGENTS.md.
- Scope gaps for the video include first-observed timestamps, late samples and potential history reconstruction, planar geometry for city-sized zones, and GPS jitter that could later be handled with hysteresis. Discuss offset page shifts, expensive deep offsets and counts, and a possible keyset migration.
- Also discuss request-per-vehicle polling and a future scoped position channel, best-effort notification loss on restart or queue saturation, outage history with exact replay deferred, and a shared event channel for multiple backend instances. Guaranteed delivery would pair an outbox with client cursors. Rule analytics could move the stored rule ID array to a link table.
- Authentication, tenant scoping, rate limiting, route replay, third-party telematics, hard deletion, and an actor audit trail are outside this submission. Sequential public IDs are guessable. A future multi-tenant version would scope every query and could add opaque IDs. Duplicate rules after a lost response are a known retry limitation.

## Known gaps from the agreed scope cuts

1. The live feed omits the outage gap notice and its link to filtered history.
2. History keeps filter and pagination state in the UI instead of the URL.
3. The geofence list omits the **No alerts configured** badge.
4. Vehicle polling omits the **Last updated** indicator.
5. Shutdown closes WebSockets without draining the pending live-alert queue. Stored records remain available.
