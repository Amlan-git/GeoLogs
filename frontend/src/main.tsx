import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider, useQuery } from "@tanstack/react-query";
import { FleetMap } from "./FleetMap";
import type { Geofence } from "./api";
import { categories } from "./categories";
import { geofencesQuery, queryDefaults, useFleet } from "./queries";
import styles from "./dashboard.module.css";
import "./global.css";

const client = new QueryClient({ defaultOptions: queryDefaults });
const noZones: Geofence[] = [];

function Dashboard() {
  const zones = useQuery(geofencesQuery());
  const { fleet, vehicles, statuses } = useFleet();
  const located = vehicles.flatMap((vehicle, index) => {
    const position = statuses[index].data?.current_location;
    return position ? [{ vehicle, position }] : [];
  });

  return (
    <>
      <header className={styles.header}>
        <div className={styles.brand}><span className={styles.brandMark} aria-hidden="true">◈</span>GeoLog</div>
        <span className={styles.headerLabel}>Fleet operations</span>
      </header>
      <main className={styles.main}>
        <h1>Fleet map</h1>
        <section className={styles.mapPanel} aria-label="Map and zone categories">
          <div className={styles.legend}>
            {Object.values(categories).map(({ label, color }) => (
              <span key={label}><i style={{ backgroundColor: color }} />{label}</span>
            ))}
          </div>
          <FleetMap zones={zones.data?.geofences ?? noZones} located={located} />
        </section>
        <div className={styles.feedback} aria-live="polite">
          {(zones.isFetching || fleet.isFetching || statuses.some((status) => status.isFetching)) && <p>Loading fleet data…</p>}
          {zones.error && <p role="alert">Geofences: {zones.error.message}</p>}
          {fleet.error && <p role="alert">Vehicles: {fleet.error.message}</p>}
          {statuses.map((status, index) => status.error && <p key={vehicles[index].id} role="alert">{vehicles[index].vehicle_number}: {status.error.message}</p>)}
        </div>
      </main>
    </>
  );
}

createRoot(document.getElementById("root")!).render(
  <QueryClientProvider client={client}><Dashboard /></QueryClientProvider>,
);
