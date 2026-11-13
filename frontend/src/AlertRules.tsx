import { useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { write, type AlertRule, type Geofence, type RuleEventType, type Vehicle } from "./api";
import { keys, rulesQuery } from "./queries";
import styles from "./dashboard.module.css";

const eventLabels: Record<RuleEventType, string> = {
  entry: "Entry",
  exit: "Exit",
  both: "Entry and exit",
};

function RuleRow({ rule }: { rule: AlertRule }) {
  const client = useQueryClient();
  const toggle = useMutation({
    mutationFn: () => write(
      `/alerts/${encodeURIComponent(rule.alert_id)}`,
      { status: rule.status === "active" ? "inactive" : "active" },
      "PATCH",
    ),
    onSuccess: () => client.invalidateQueries({ queryKey: keys.rules }),
  });

  return <li className={rule.status === "inactive" ? styles.muted : undefined}>
    <div className={styles.rowTitle}>
      <strong>{rule.geofence_name}</strong>
      <label className={styles.ruleSwitch}>
        <input type="checkbox" role="switch"
          aria-label={`Alert rule for ${rule.geofence_name}, ${rule.vehicle_number ?? "all vehicles"}, ${eventLabels[rule.event_type]}`}
          checked={rule.status === "active"} disabled={toggle.isPending}
          onChange={() => toggle.mutate()} />
        {toggle.isPending ? "Saving…" : rule.status === "active" ? "Active" : "Inactive"}
      </label>
    </div>
    <p>{rule.vehicle_number ?? "All vehicles"} · {eventLabels[rule.event_type]}</p>
    {toggle.error && <p role="alert" className={styles.error}>{toggle.error.message}</p>}
  </li>;
}

export function AlertRules({ zones, vehicles }: { zones: Geofence[]; vehicles: Vehicle[] }) {
  const client = useQueryClient();
  const rules = useQuery(rulesQuery);
  const [saved, setSaved] = useState(false);
  const create = useMutation({
    mutationFn: (data: FormData) => write("/alerts/configure", {
      geofence_id: data.get("geofence_id"),
      vehicle_id: data.get("vehicle_id") || undefined,
      event_type: data.get("event_type"),
    }),
    onSuccess: () => client.invalidateQueries({ queryKey: keys.rules }),
  });

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (create.isPending) return;
    const form = event.currentTarget;
    setSaved(false);
    create.mutate(new FormData(form), { onSuccess: () => {
      form.reset();
      setSaved(true);
    } });
  }

  return <section className={styles.panel}>
    <h2>Alert rules</h2>
    <form className={styles.form} onSubmit={submit}>
      <label>Geofence<select name="geofence_id" required defaultValue="">
        <option value="">Select geofence</option>
        {zones.map((zone) => <option key={zone.id} value={zone.id}>{zone.name}</option>)}
      </select></label>
      <label>Vehicle<select name="vehicle_id">
        <option value="">All vehicles</option>
        {vehicles.map((vehicle) => <option key={vehicle.id} value={vehicle.id}>{vehicle.vehicle_number}</option>)}
      </select></label>
      <label>Event type<select name="event_type" defaultValue="both">
        {Object.entries(eventLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
      </select></label>
      <button className={styles.primary} disabled={create.isPending || !zones.length}>
        {create.isPending ? "Saving rule…" : "Create alert rule"}
      </button>
      {create.error && <p role="alert" className={styles.error}>{create.error.message}</p>}
      {saved && <p role="status">Alert rule created.</p>}
    </form>
    {rules.isFetching && <p role="status">Loading alert rules…</p>}
    {rules.error && <p role="alert" className={styles.error}>{rules.error.message}</p>}
    {rules.data?.alerts.length === 0 && <p className={styles.muted}>No alert rules yet.</p>}
    <ul className={styles.rows}>
      {rules.data?.alerts.map((rule) => <RuleRow key={rule.alert_id} rule={rule} />)}
    </ul>
  </section>;
}
