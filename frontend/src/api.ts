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

export async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const base = import.meta.env.VITE_API_BASE_URL?.trim().replace(/\/+$/, "");
  if (!base) throw new Error("GeoLog’s API URL is missing.");
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
