# GeoLog handoff

Read [AGENTS.md](../AGENTS.md), the local [assignment contract](BRIEF.md), and [DECISIONS.md](../DECISIONS.md) before changing behavior. The current stop point is Stage 5, Step 2 review. The user will direct the next step.

## Completed work

| Stage | Work | Commits |
| --- | --- | --- |
| Planning | Local brief and approved decisions | `a275311` |
| 1 | Go readiness, embedded PostGIS migration, Docker, and Vercel walking skeleton | `e329864` |
| 2 | Schema, geofence and vehicle endpoints, validation and response normalization | `fffbc90`, `d7d5bf3` |
| 3 | Location updates, containment and crossings, filtered history, public ID parsing, five scope cuts | `9c59d65` |
| 4 | Alert rules, transactional alert records, and WebSocket delivery | `cd3d15d` |
| 5.1 | Fleet map, API client, query keys, and polling | `413f4e4` |
| 5.2 | Geofence drawing, vehicle registration, location forms, and lists | `4e8a485` |
| 5.2 review | Persistent map picking, temporary pin, marker picking, viewport controls, favicon, and timestamp submission | `168010b`, `aefe758` |

## Frontend

- React, TypeScript, Vite, CSS Modules, TanStack Query, Leaflet, and Leaflet-Geoman. `frontend/src/main.tsx` owns dashboard state and layout. `FleetMap.tsx` owns Leaflet layers and the draw and pick handlers. `FleetForms.tsx` contains forms, lists, and feedback. `categories.ts` defines category labels and colors.
- `api.ts` is the single HTTP entry point. Use its `request` and `write` helpers for readable server errors. `queries.ts` owns query keys, shared cache, and visible-tab polling. Keep forms and map interaction in React state. Invalidate the affected query after a write.
- Vehicle markers use bundled Leaflet image assets. Map selection persists across clicks. Marker clicks bubble to the map with the marker coordinates. The temporary pin is noninteractive. OpenStreetMap tiles are external and require attribution.
- An omitted location timestamp uses the browser's current time. An explicit local timestamp is converted to UTC before posting. A stale report is saved in the log while the current marker remains at its last accepted position. Drawn rings use the API's latitude-first order and are closed before submission.
- Restricted geofences offer a default-on entry-and-exit rule. Geofence creation and rule creation are separate requests. If the second fails, the form keeps the saved zone and offers a retry.

## Deployments and configuration

- Private GitHub repository [Amlan-git/GeoLogs](https://github.com/Amlan-git/GeoLogs), branch `main`.
- Frontend [Vercel](https://geolog-stage1.vercel.app/). Set `VITE_API_BASE_URL` at build time. The Vercel project is `geolog-stage1`, with the CLI linked from `frontend/` when local deployment metadata is present.
- Backend [Render](https://geologs-backend-stage1.onrender.com/healthz). The service name and URL still say Stage 1. It serves the Stage 4 image `amlanb26/geologs-backend:stage4` for `linux/amd64`. The live endpoint is `wss://geologs-backend-stage1.onrender.com/ws/alerts`.
- Backend settings are `DATABASE_URL`, `PORT`, and `ALLOWED_ORIGINS`. Use the internal Render Postgres URL in the backend. Never copy credential values into docs or frontend configuration. The `.env.example` files document local development settings.
- Render free instances can cold-start. The build may warn about TanStack Query's `use client` directives under Vite. The browser bundle builds successfully.

## Tests and builds

From the repository root:

```sh
npm --prefix frontend ci
npm --prefix frontend run build
npm --prefix frontend run dev
```

For local browser testing, set `VITE_API_BASE_URL` to the intended backend before starting Vite. With a fresh disposable PostgreSQL database on a PostGIS-enabled server, set `GEOLOG_TEST_DATABASE_URL` to its connection URL, then run:

```sh
cd backend
go test -race -v -count=1 ./...
go vet ./...
go build ./cmd/server
```

The HTTP integration tests skip when that test URL is absent. Use a separate fresh database for the HTTP smoke script. Start the Go server with `DATABASE_URL` set, then run `bash scripts/smoke.sh http://localhost:8080` from the root. The script covers rule setup and the complete outside → inside → inside → outside flow.

## Next work

1. **Stage 5 Step 3.** Add the rules form and list with enable/disable, a top-level WebSocket provider, connection status and reconnect handling, live feed, and toasts. Use red styling for restricted alerts. Keep the five agreed cuts. Use the committed alert payload and query invalidation behavior from DECISIONS.md.
2. **Stage 5 Step 4.** Add violation history with vehicle, geofence, and UTC date filters, 50-row offset pagination, and previous-page data during loading. Keep filter state in the UI.
3. Commit and push each step. Run the production build and verify the Vercel deployment against Render. Stage 5's final walkthrough should verify an alert in two tabs and corresponding filtered history. Compose, final setup documentation, reviewer access, and submission deliverables follow the dashboard stage.
