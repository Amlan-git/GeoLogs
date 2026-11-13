import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { alertsSocketURL, type LiveAlert } from "./api";
import { categories } from "./categories";
import { keys } from "./queries";
import { formatUTC } from "./timestamps";
import styles from "./dashboard.module.css";

type ConnectionStatus = "Live" | "Reconnecting" | "Offline";
const AlertsContext = createContext<{ status: ConnectionStatus; alerts: LiveAlert[] }>({
  status: "Offline", alerts: [],
});

export function LiveAlertsProvider({ children }: { children: ReactNode }) {
  const client = useQueryClient();
  const [status, setStatus] = useState<ConnectionStatus>(navigator.onLine ? "Reconnecting" : "Offline");
  const [alerts, setAlerts] = useState<LiveAlert[]>([]);
  const [toasts, setToasts] = useState<LiveAlert[]>([]);

  useEffect(() => {
    let url: string;
    try {
      url = alertsSocketURL();
    } catch {
      setStatus("Offline");
      return;
    }

    let socket: WebSocket | undefined;
    let timer: number | undefined;
    let attempts = 0;
    let connectedBefore = false;
    let disposed = false;
    const recentEvents = new Set<string>();

    function close() {
      window.clearTimeout(timer);
      const previous = socket;
      socket = undefined;
      previous?.close();
    }

    function retry() {
      if (!navigator.onLine) {
        setStatus("Offline");
        return;
      }
      setStatus("Reconnecting");
      const delay = Math.min(30_000, 1000 * 2 ** Math.min(attempts++, 5));
      timer = window.setTimeout(connect, Math.min(30_000, delay * (0.9 + Math.random() * 0.2)));
    }

    function connect() {
      close();
      if (!navigator.onLine) {
        setStatus("Offline");
        return;
      }
      setStatus("Reconnecting");
      let connection: WebSocket;
      try {
        connection = new WebSocket(url);
      } catch {
        retry();
        return;
      }
      socket = connection;
      connection.onopen = () => {
        if (socket !== connection) return;
        attempts = 0;
        setStatus("Live");
        if (connectedBefore) {
          void client.invalidateQueries({ queryKey: keys.vehicleStatuses });
          void client.invalidateQueries({ queryKey: keys.violationPages });
        }
        connectedBefore = true;
      };
      connection.onclose = () => {
        if (disposed || socket !== connection) return;
        socket = undefined;
        retry();
      };
      connection.onmessage = ({ data }) => {
        if (socket !== connection || typeof data !== "string") return;
        let alert: LiveAlert;
        try {
          alert = JSON.parse(data) as LiveAlert;
        } catch {
          return;
        }
        if (!alert?.event_id || !alert.vehicle?.vehicle_id || !alert.location ||
            !Object.hasOwn(categories, alert.geofence?.category ?? "") ||
            (alert.event_type !== "entry" && alert.event_type !== "exit")) return;
        if (recentEvents.has(alert.event_id)) return;
        recentEvents.add(alert.event_id);
        if (recentEvents.size > 100) {
          const oldest = recentEvents.values().next().value;
          if (oldest !== undefined) recentEvents.delete(oldest);
        }
        setAlerts((previous) => [alert, ...previous].slice(0, 100));
        setToasts((previous) => [...previous, alert].slice(-3));
        void client.invalidateQueries({ queryKey: keys.vehicle(alert.vehicle.vehicle_id), exact: true });
        void client.invalidateQueries({ queryKey: keys.violationPages });
      };
    }

    function reconnectIfDisconnected() {
      if (!socket || socket.readyState > WebSocket.OPEN) connect();
    }

    function onVisibilityChange() {
      if (document.visibilityState === "visible") reconnectIfDisconnected();
    }

    function onOffline() {
      close();
      setStatus("Offline");
    }

    window.addEventListener("online", reconnectIfDisconnected);
    window.addEventListener("offline", onOffline);
    document.addEventListener("visibilitychange", onVisibilityChange);
    connect();
    return () => {
      disposed = true;
      close();
      window.removeEventListener("online", reconnectIfDisconnected);
      window.removeEventListener("offline", onOffline);
      document.removeEventListener("visibilitychange", onVisibilityChange);
    };
  }, [client]);

  const dismiss = useCallback((id: string) => {
    setToasts((previous) => previous.filter((alert) => alert.event_id !== id));
  }, []);

  return <AlertsContext.Provider value={{ status, alerts }}>
    {children}
    <div className={styles.toasts} aria-label="Alert notifications" aria-live="polite">
      {toasts.map((alert) => <Toast key={alert.event_id} alert={alert} dismiss={dismiss} />)}
    </div>
  </AlertsContext.Provider>;
}

function AlertDetails({ alert }: { alert: LiveAlert }) {
  return <>
    <strong>{alert.event_type === "entry" ? "Entry" : "Exit"} · {alert.geofence.geofence_name}</strong>
    <p>{alert.vehicle.vehicle_number} · {alert.vehicle.driver_name}</p>
    <p>{categories[alert.geofence.category].label}</p>
    <p><time dateTime={alert.timestamp}>{formatUTC(alert.timestamp)}</time></p>
    <p>Lat {alert.location.latitude}, lon {alert.location.longitude}</p>
  </>;
}

function Toast({ alert, dismiss }: { alert: LiveAlert; dismiss: (id: string) => void }) {
  useEffect(() => {
    const timer = window.setTimeout(() => dismiss(alert.event_id), 8000);
    return () => window.clearTimeout(timer);
  }, [alert.event_id, dismiss]);
  return <div className={styles.toast} style={{ borderLeftColor: categories[alert.geofence.category].color }}>
    <AlertDetails alert={alert} />
    <button type="button" className={styles.quiet} onClick={() => dismiss(alert.event_id)}>Dismiss</button>
  </div>;
}

export function LiveStatus() {
  const { status } = useContext(AlertsContext);
  return <span className={styles.connection} role="status" aria-label={`Alert connection: ${status}`}>Alerts · {status}</span>;
}

export function LiveFeed() {
  const { alerts } = useContext(AlertsContext);
  return <section className={styles.panel}>
    <h2>Live alerts</h2>
    {alerts.length === 0 && <p className={styles.muted}>No live alerts yet. Move a vehicle across a geofence with an active rule.</p>}
    <ul className={`${styles.rows} ${styles.alertFeed}`}>
      {alerts.map((alert) => <li key={alert.event_id}>
        <div className={styles.alertCard} style={{ borderLeftColor: categories[alert.geofence.category].color }}>
          <AlertDetails alert={alert} />
        </div>
      </li>)}
    </ul>
  </section>;
}
