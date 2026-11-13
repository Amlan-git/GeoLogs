import { memo, useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { write, type Category, type Coordinates, type Geofence, type Vehicle, type VehicleStatus } from "./api";
import { parseCoordinateRing } from "./coordinates";
import { categories } from "./categories";
import { geofencesQuery, keys } from "./queries";
import styles from "./dashboard.module.css";

export type MapMode = "browse" | "draw" | "pick";

function Feedback({ error, message }: { error: Error | null; message?: string }) {
  return <>{error && <p role="alert" className={styles.error}>{error.message}</p>}{message && <p role="status">{message}</p>}</>;
}

export function ZoneForm({ ring, mode, onMode, setRing }: {
  ring: Coordinates[] | null;
  mode: MapMode;
  onMode: (mode: MapMode) => void;
  setRing: (ring: Coordinates[] | null) => void;
}) {
  const client = useQueryClient();
  const [source, setSource] = useState<"map" | "coordinates">("map");
  const [coordinates, setCoordinates] = useState("");
  const [coordinateError, setCoordinateError] = useState<Error | null>(null);
  const [category, setCategory] = useState<Category>("delivery_zone");
  const [defaultRule, setDefaultRule] = useState(true);
  const [warning, setWarning] = useState<{ id: string; message: string } | null>(null);
  const rule = useMutation({
    mutationFn: (id: string) => write("/alerts/configure", { geofence_id: id, event_type: "both" }),
    onSuccess: () => {
      setWarning(null);
      void client.invalidateQueries({ queryKey: keys.rules });
    },
    onError: (error, id) => setWarning({ id, message: error.message }),
  });
  const create = useMutation({
    mutationFn: (data: Omit<Geofence, "id">) => write<{ id: string }>("/geofences", data),
    onSuccess: () => {
      setRing(null);
      setCoordinates("");
      void client.invalidateQueries({ queryKey: ["geofences"] });
    },
  });

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!ring || create.isPending || rule.isPending) return;
    const form = event.currentTarget;
    const data = new FormData(form);
    const configure = category === "restricted_zone" && defaultRule;
    let saved: { id: string };
    try {
      saved = await create.mutateAsync({
        name: String(data.get("name")), description: String(data.get("description")),
        category, coordinates: ring,
      });
    } catch {
      return;
    }
    form.reset();
    setCategory("delivery_zone");
    setDefaultRule(true);
    if (configure) rule.mutate(saved.id);
  }

  return (
    <form className={styles.form} onSubmit={submit}>
      <h3>Create a geofence</h3>
      <label>Name<input name="name" required /></label>
      <label>Description<textarea name="description" rows={2} /></label>
      <label>Category
        <select value={category} onChange={(event) => { setCategory(event.target.value as Category); setDefaultRule(true); }}>
          {Object.entries(categories).map(([value, { label }]) => <option key={value} value={value}>{label}</option>)}
        </select>
      </label>
      {category === "restricted_zone" && <label className={styles.check}>
        <input type="checkbox" checked={defaultRule} onChange={(event) => setDefaultRule(event.target.checked)} />
        Alert on entry and exit for all vehicles
      </label>}
      <label>Boundary input
        <select value={source} disabled={create.isPending || rule.isPending} onChange={(event) => {
          setSource(event.target.value === "map" ? "map" : "coordinates");
          onMode("browse");
          setRing(null);
          setCoordinateError(null);
        }}>
          <option value="map">Draw on map</option>
          <option value="coordinates">Enter coordinates</option>
        </select>
      </label>
      {source === "map" ? <>
        <button className={styles.secondary} type="button" disabled={create.isPending || rule.isPending}
          onClick={() => onMode(mode === "draw" ? "browse" : "draw")}>
          {mode === "draw" ? "Cancel drawing" : ring ? "Redraw boundary" : "Draw boundary on map"}
        </button>
        {mode === "draw" && <p className={styles.muted}>Click to add vertices, then click the first point to close the polygon.</p>}
      </> : <label>Coordinates (latitude, longitude)
        <textarea rows={4} value={coordinates} placeholder={"37.77, -122.42\n37.78, -122.42\n37.78, -122.41"}
          disabled={create.isPending || rule.isPending}
          onChange={(event) => {
            const value = event.target.value;
            setCoordinates(value);
            setCoordinateError(null);
            setRing(null);
            if (!value.trim()) return;
            try {
              setRing(parseCoordinateRing(value));
            } catch (error) {
              if (error instanceof Error) setCoordinateError(error);
            }
          }} />
        <span className={styles.muted}>One pair per line. The ring closes automatically.</span>
      </label>}
      {coordinateError && <p className={styles.error} role="alert">{coordinateError.message}</p>}
      {ring && mode !== "draw" && <p className={styles.muted}>{ring.length - 1} vertices in preview</p>}
      <button className={styles.primary} disabled={!ring || mode === "draw" || create.isPending || rule.isPending}>
        {create.isPending ? "Saving geofence…" : "Save geofence"}
      </button>
      <Feedback error={create.error} message={rule.isPending ? "Creating alert rule…" : create.isSuccess ? "Geofence saved." : undefined} />
      {warning && <div className={styles.warning} role="alert">
        <p>Zone saved, but its alert rule could not be created. This zone will not send alerts yet.</p>
        <p>{warning.message}</p>
        <div className={styles.actions}>
          <button type="button" className={styles.secondary} onClick={() => rule.mutate(warning.id)} disabled={rule.isPending}>{rule.isPending ? "Retrying…" : "Retry"}</button>
          <button type="button" className={styles.secondary} onClick={() => setWarning(null)}>Dismiss</button>
        </div>
      </div>}
    </form>
  );
}

export function RegisterVehicle() {
  const client = useQueryClient();
  const mutation = useMutation({
    mutationFn: (data: FormData) => write("/vehicles", Object.fromEntries(data)),
    onSuccess: () => { void client.invalidateQueries({ queryKey: keys.vehicles }); },
  });
  return (
    <form className={styles.form} onSubmit={(event) => {
      event.preventDefault();
      if (mutation.isPending) return;
      const form = event.currentTarget;
      mutation.mutate(new FormData(form), { onSuccess: () => form.reset() });
    }}>
      <h3>Register vehicle</h3>
      <label>Vehicle number<input name="vehicle_number" required /></label>
      <label>Driver name<input name="driver_name" required /></label>
      <label>Vehicle type<input name="vehicle_type" placeholder="e.g. Truck" required /></label>
      <label>Phone<input name="phone" type="tel" required /></label>
      <button className={styles.primary} disabled={mutation.isPending}>{mutation.isPending ? "Registering…" : "Register vehicle"}</button>
      <Feedback error={mutation.error} message={mutation.isSuccess ? "Vehicle registered." : undefined} />
    </form>
  );
}

export function LocationForm({ vehicles, vehicleID, setVehicleID, point, setPoint, mode, onMode, onSubmitted }: {
  vehicles: Vehicle[];
  vehicleID: string;
  setVehicleID: (id: string) => void;
  point: [string, string];
  setPoint: (point: [string, string]) => void;
  mode: MapMode;
  onMode: (mode: MapMode) => void;
  onSubmitted: () => void;
}) {
  const client = useQueryClient();
  const [timestamp, setTimestamp] = useState("");
  const [dateError, setDateError] = useState<Error | null>(null);
  const mutation = useMutation({
    mutationFn: (sample: { vehicle_id: string; latitude: number; longitude: number; timestamp: string }) =>
      write<{ location_updated: boolean }>("/vehicles/location", sample),
    onSuccess: (result, sample) => {
      setTimestamp("");
      onSubmitted();
      void client.invalidateQueries({ queryKey: keys.vehicle(sample.vehicle_id) });
      if (result.location_updated) void client.invalidateQueries({ queryKey: keys.violationPages });
    },
  });
  return (
    <form className={styles.form} onSubmit={(event) => {
      event.preventDefault();
      if (mutation.isPending) return;
      const enteredTimestamp = String(new FormData(event.currentTarget).get("timestamp") ?? "");
      const date = enteredTimestamp ? new Date(enteredTimestamp) : new Date();
      if (!Number.isFinite(date.getTime())) {
        setDateError(new Error("Enter a valid timestamp."));
        return;
      }
      setDateError(null);
      mutation.mutate({ vehicle_id: vehicleID, latitude: Number(point[0]), longitude: Number(point[1]), timestamp: date.toISOString() });
    }}>
      <h3>Update location</h3>
      <label>Vehicle<select required value={vehicleID} onChange={(event) => setVehicleID(event.target.value)}>
        <option value="">Select vehicle</option>
        {vehicles.map((vehicle) => <option key={vehicle.id} value={vehicle.id}>{vehicle.vehicle_number}</option>)}
      </select></label>
      <div className={styles.fields}>
        <label>Latitude<input required type="number" step="any" min={-90} max={90} value={point[0]} onChange={(event) => setPoint([event.target.value, point[1]])} /></label>
        <label>Longitude<input required type="number" step="any" min={-180} max={180} value={point[1]} onChange={(event) => setPoint([point[0], event.target.value])} /></label>
      </div>
      <button className={styles.primary} disabled={mutation.isPending}>{mutation.isPending ? "Updating…" : "Update location"}</button>
      <button className={styles.secondary} type="button" onClick={() => onMode(mode === "pick" ? "browse" : "pick")}>
        {mode === "pick" ? "Cancel map selection" : "Pick coordinates on map"}
      </button>
      <label>Timestamp (local time, optional)<input type="datetime-local" name="timestamp" step="1" value={timestamp} onChange={(event) => setTimestamp(event.target.value)} /></label>

      <Feedback error={dateError ?? mutation.error} message={mutation.isSuccess
        ? mutation.data.location_updated ? "Location updated." : "This report is older than the vehicle's latest position, so the map was not changed."
        : undefined} />
    </form>
  );
}

export function GeofenceList() {
  const [category, setCategory] = useState<Category | "">("");
  const query = useQuery(geofencesQuery(category));
  return (
    <section className={styles.panel}>
      <h2>Geofences</h2>
      <label className={styles.filter}>Category filter<select value={category} onChange={(event) => setCategory(event.target.value as Category | "")}>
        <option value="">All categories</option>
        {Object.entries(categories).map(([value, { label }]) => <option key={value} value={value}>{label}</option>)}
      </select></label>
      <Feedback error={query.error} message={query.isFetching ? "Loading geofences…" : undefined} />
      {query.data?.geofences.length === 0 && <p className={styles.muted}>No geofences in this view.</p>}
      <ul className={styles.rows}>
        {query.data?.geofences.map((zone) => <li key={zone.id}>
          <strong>{zone.name}</strong>
          <span className={styles.chip} style={{ color: categories[zone.category].color }}>{categories[zone.category].label}</span>
          {zone.description && <p>{zone.description}</p>}
        </li>)}
      </ul>
    </section>
  );
}

export const VehicleRow = memo(function VehicleRow({ vehicle, status, fetching, error, select, showOnMap }: {
  vehicle: Vehicle; status?: VehicleStatus; fetching: boolean; error: Error | null; select: (id: string) => void; showOnMap: (id: string) => void;
}) {
  return <li>
    <div className={styles.rowTitle}><strong>{vehicle.vehicle_number}</strong>
      <div className={styles.actions}>
      <button className={styles.secondary} type="button" onClick={() => select(vehicle.id)}>Set location</button>
        <button className={styles.secondary} type="button" disabled={!status?.current_location} onClick={() => showOnMap(vehicle.id)}>Show on map</button>
      </div>
    </div>
    <p>{vehicle.driver_name} · {vehicle.vehicle_type}</p>
    <Feedback error={error} message={fetching ? "Loading location…" : undefined} />
    {status && (status.current_location ? <>
      <p className={styles.muted}>{status.current_location.latitude.toFixed(5)}, {status.current_location.longitude.toFixed(5)}</p>
      <p className={styles.muted}>{new Date(status.current_location.timestamp).toLocaleString()}</p>
      {status.current_geofences.map((zone) => <span className={styles.chip} key={zone.geofence_id} style={{ color: categories[zone.category].color }}>{zone.geofence_name}</span>)}
    </> : <p className={styles.muted}>Awaiting first location</p>)}
  </li>;
});
