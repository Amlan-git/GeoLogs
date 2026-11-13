export type Category = "delivery_zone" | "restricted_zone" | "toll_zone" | "customer_area";
export type Coordinates = [number, number];

export interface Geofence {
  id: string;
  name: string;
  description: string;
  category: Category;
  coordinates: Coordinates[];
}

export interface Vehicle {
  id: string;
  vehicle_number: string;
  driver_name: string;
  vehicle_type: string;
  phone: string;
}

export interface Position {
  latitude: number;
  longitude: number;
  timestamp: string;
}

export interface VehicleStatus {
  current_location: Position | null;
  current_geofences: { geofence_id: string; geofence_name: string; category: Category }[];
}

export class ApiError extends Error {
  constructor(message: string, readonly status: number) {
    super(message);
  }
}

function apiBaseURL() {
  const base = import.meta.env.VITE_API_BASE_URL?.trim().replace(/\/+$/, "");
  if (!base) throw new Error("GeoLog’s API URL is missing.");
  return base;
}

export async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const base = apiBaseURL();
  let response: Response;
  try {
    response = await fetch(base + path, options);
  } catch (error) {
    if (options?.signal?.aborted) throw error;
    throw new Error("Couldn’t reach GeoLog’s API. Check your connection and try again.");
  }
  const body = await response.json().catch(() => null);
  if (!response.ok) throw new ApiError(body?.error || `The API returned HTTP ${response.status}.`, response.status);
  if (!body) throw new ApiError("The API returned an unreadable response.", response.status);
  return body as T;
}

export function write<T>(path: string, body: unknown, method = "POST") {
  return request<T>(path, {
    method,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export type CrossingType = "entry" | "exit";
export type RuleEventType = CrossingType | "both";
export type RuleStatus = "active" | "inactive";

export interface AlertRule {
  alert_id: string;
  geofence_id: string;
  geofence_name: string;
  vehicle_id: string | null;
  vehicle_number: string | null;
  event_type: RuleEventType;
  status: RuleStatus;
  created_at: string;
}

export interface LiveAlert {
  event_id: string;
  event_type: CrossingType;
  timestamp: string;
  vehicle: { vehicle_id: string; vehicle_number: string; driver_name: string };
  geofence: { geofence_id: string; geofence_name: string; category: Category };
  location: { latitude: number; longitude: number };
}

export function alertsSocketURL() {
  const url = new URL("/ws/alerts", apiBaseURL());
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  return url.href;
}

export interface Violation extends Position {
  id: string;
  vehicle_id: string;
  vehicle_number: string;
  geofence_id: string;
  geofence_name: string;
  event_type: CrossingType;
}

export interface HistoryFilters {
  vehicle_id?: string;
  geofence_id?: string;
  start_date?: string;
  end_date?: string;
}
