import { useCallback, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider, useQuery } from "@tanstack/react-query";
import { AlertRules } from "./AlertRules";
import { LiveAlertsProvider, LiveFeed, LiveStatus } from "./LiveAlerts";
import { ViolationHistory } from "./ViolationHistory";
import { FleetMap } from "./FleetMap";
import type { Coordinates, Geofence } from "./api";
import { GeofenceList, LocationForm, RegisterVehicle, VehicleRow, ZoneForm, type MapMode } from "./FleetForms";
import { categories } from "./categories";
import { geofencesQuery, queryDefaults, useFleet } from "./queries";
import styles from "./dashboard.module.css";
import "./global.css";

const client = new QueryClient({ defaultOptions: queryDefaults });
const noZones: Geofence[] = [];

function Dashboard() {
  const mapPanel = useRef<HTMLElement>(null);
  const locationPanel = useRef<HTMLElement>(null);
  const [mode, setMode] = useState<MapMode>("browse");
  const [ring, setRing] = useState<Coordinates[] | null>(null);
  const [point, setPoint] = useState<[string, string]>(["", ""]);
  const [picked, setPicked] = useState<Coordinates | null>(null);
  const [mapFocus, setMapFocus] = useState<{ id: string } | null>(null);
  const [vehicleID, setVehicleID] = useState("");
  const changeMode = useCallback((next: MapMode) => {
    setMode(next);
    setPicked(null);
    if (next !== "browse") mapPanel.current?.scrollIntoView({ block: "nearest" });
  }, []);
  const onDraw = useCallback((coordinates: Coordinates[]) => {
    setRing(coordinates);
    setMode("browse");
  }, []);
  const onPick = useCallback(([lat, lon]: Coordinates) => {
    setPoint([String(lat), String(lon)]);
    setPicked([lat, lon]);
  }, []);
  const showOnMap = useCallback((id: string) => {
    setMapFocus({ id });
    mapPanel.current?.scrollIntoView({ block: "nearest" });
  }, []);
  const select = useCallback((id: string) => {
    setVehicleID(id);
    locationPanel.current?.scrollIntoView({ block: "nearest" });
  }, []);
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
        <LiveStatus />
      </header>
      <main className={styles.main}>
        <h1>Fleet map</h1>
        <div className={styles.workspace}>
          <section ref={mapPanel} className={styles.mapPanel} aria-label="Map and zone categories">
            <div className={styles.legend}>
              {Object.values(categories).map(({ label, color }) => (
                <span key={label}><i style={{ backgroundColor: color }} />{label}</span>
              ))}
            </div>
            {mode !== "browse" && <div className={styles.mapPrompt} role="status">
              {mode === "draw" ? "Draw a boundary. Click the first point to finish." : "Click the map to pick coordinates."}
              <button type="button" className={styles.secondary} onClick={() => changeMode("browse")}>Cancel</button>
            </div>}
            <FleetMap zones={zones.data?.geofences ?? noZones} located={located} mode={mode} draft={ring} onDraw={onDraw} onPick={onPick} picked={picked} focus={mapFocus} />
          </section>
        </div>
        <div className={styles.feedback} aria-live="polite">
          {(zones.isFetching || fleet.isFetching || statuses.some((status) => status.isFetching)) && <p>Loading fleet data…</p>}
          {zones.error && <p role="alert">Geofences: {zones.error.message}</p>}
          {fleet.error && <p role="alert">Vehicles: {fleet.error.message}</p>}

        </div>
        <div className={styles.workspace}>
          <GeofenceList />
          <section className={styles.panel}>
            <ZoneForm ring={ring} mode={mode} onMode={changeMode} setRing={setRing} />
          </section>
        </div>
        <div className={styles.workspace}>
          <section className={styles.panel}>
            <h2>Vehicles</h2>
            {fleet.data?.vehicles.length === 0 && <p className={styles.muted}>No vehicles registered yet. Register a vehicle to track its location.</p>}
            <ul className={styles.rows}>
              {vehicles.map((vehicle, index) => <VehicleRow key={vehicle.id} vehicle={vehicle} status={statuses[index].data}
                fetching={statuses[index].isFetching} error={statuses[index].error} select={select} showOnMap={showOnMap} />)}
            </ul>
          </section>
          <section className={styles.panel}><RegisterVehicle /></section>
        </div>
          <section ref={locationPanel} className={`${styles.panel} ${styles.locationSection}`}>
            <LocationForm vehicles={vehicles} vehicleID={vehicleID} setVehicleID={setVehicleID} point={point} setPoint={setPoint} mode={mode} onMode={changeMode} onSubmitted={() => changeMode("browse")} />
          </section>
        <div className={styles.workspace}>
          <AlertRules zones={zones.data?.geofences ?? noZones} vehicles={vehicles} />
          <LiveFeed />
        </div>
        <ViolationHistory zones={zones.data?.geofences ?? noZones} vehicles={vehicles} />
      </main>
    </>
  );
}

createRoot(document.getElementById("root")!).render(
  <QueryClientProvider client={client}>
    <LiveAlertsProvider><Dashboard /></LiveAlertsProvider>
  </QueryClientProvider>,
);
