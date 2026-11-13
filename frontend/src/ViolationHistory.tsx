import { useState } from "react";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import type { Geofence, HistoryFilters, Vehicle } from "./api";
import { historyPageSize, historyQuery } from "./queries";
import styles from "./dashboard.module.css";

function HistoryResults({ filters }: { filters: HistoryFilters }) {
  const [page, setPage] = useState(1);
  const history = useQuery({ ...historyQuery(filters, page), placeholderData: keepPreviousData });
  const count = history.data?.total_count ?? 0;
  const lastPage = Math.max(1, Math.ceil(count / historyPageSize));

  return <>
    {history.isFetching && <p role="status">
      {history.isPlaceholderData ? `Loading page ${page}…` : "Loading crossing history…"}
    </p>}
    {history.error && <div role="alert" className={styles.error}>
      <p>{history.error.message}</p>
      <button type="button" className={styles.secondary} onClick={() => void history.refetch()}>Retry</button>
    </div>}
    {history.data && count === 0 && <p className={styles.muted}>No crossings match these filters.</p>}
    {history.data && history.data.violations.length > 0 && <div className={styles.historyTable}>
      <table aria-label="Geofence crossings">
        <thead><tr><th scope="col">Time (UTC)</th><th scope="col">Vehicle</th><th scope="col">Geofence</th>
          <th scope="col">Event</th><th scope="col">Coordinates</th></tr></thead>
        <tbody>{history.data.violations.map((event) => <tr key={event.id}>
          <td><time dateTime={event.timestamp}>{event.timestamp.replace("T", " ").replace(/Z$/, "")}</time></td>
          <td>{event.vehicle_number}</td>
          <td>{event.geofence_name}</td>
          <td>{event.event_type === "entry" ? "Entry" : "Exit"}</td>
          <td>{event.latitude}, {event.longitude}</td>
        </tr>)}</tbody>
      </table>
    </div>}
    {(count > 0 || page > 1) && <nav className={styles.historyPager} aria-label="History pagination">
      <button type="button" className={styles.secondary} disabled={page <= 1 || history.isPlaceholderData}
        onClick={() => setPage(page - 1)}>Previous</button>
      {history.data && <span>
        {history.isPlaceholderData ? `Loading page ${page} of ${lastPage}` : `Page ${page} of ${lastPage} · ${count} crossings`}
      </span>}
      <button type="button" className={styles.secondary} disabled={page >= lastPage || history.isPlaceholderData}
        onClick={() => setPage(page + 1)}>Next</button>
    </nav>}
  </>;
}

export function ViolationHistory({ zones, vehicles }: { zones: Geofence[]; vehicles: Vehicle[] }) {
  const [vehicle, setVehicle] = useState("");
  const [zone, setZone] = useState("");
  const [start, setStart] = useState("");
  const [end, setEnd] = useState("");
  const startTime = start ? Date.parse(`${start}Z`) : undefined;
  const endTime = end ? Date.parse(`${end}Z`) : undefined;
  const invalidDate = Number.isNaN(startTime) || Number.isNaN(endTime);
  const reversed = startTime !== undefined && endTime !== undefined && startTime > endTime;
  const filters: HistoryFilters = {
    vehicle_id: vehicle || undefined,
    geofence_id: zone || undefined,
    start_date: startTime !== undefined && !invalidDate ? new Date(startTime).toISOString() : undefined,
    end_date: endTime !== undefined && !invalidDate ? new Date(endTime).toISOString() : undefined,
  };

  return <section className={`${styles.panel} ${styles.history}`}>
    <h2>Violation history</h2>
    <div className={`${styles.form} ${styles.historyFilters}`}>
      <label>Vehicle<select value={vehicle} onChange={(event) => setVehicle(event.target.value)}>
        <option value="">All vehicles</option>
        {vehicles.map((item) => <option key={item.id} value={item.id}>{item.vehicle_number}</option>)}
      </select></label>
      <label>Geofence<select value={zone} onChange={(event) => setZone(event.target.value)}>
        <option value="">All geofences</option>
        {zones.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
      </select></label>
      <label>From (UTC)<input type="datetime-local" step="1" value={start} onChange={(event) => setStart(event.target.value)} /></label>
      <label>To (UTC)<input type="datetime-local" step="1" value={end} onChange={(event) => setEnd(event.target.value)} /></label>
    </div>
    {invalidDate || reversed
      ? <p role="alert" className={styles.error}>{invalidDate ? "Enter valid dates." : "The end must be at or after the start."}</p>
      : <HistoryResults key={JSON.stringify(filters)} filters={filters} />}
  </section>;
}
