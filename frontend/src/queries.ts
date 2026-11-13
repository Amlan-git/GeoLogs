import { queryOptions, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { ApiError, request, type Category, type Geofence, type Vehicle, type VehicleStatus } from "./api";

export const keys = {
  geofences: (category: Category | "" = "") => ["geofences", category] as const,
  vehicles: ["vehicles"] as const,
  vehicle: (id: string) => ["vehicle-status", id] as const,
  rules: ["alert-rules"] as const,
  violations: (filters: object) => ["violations", filters] as const,
};

export const geofencesQuery = (category: Category | "" = "") => queryOptions({
  queryKey: keys.geofences(category),
  queryFn: ({ signal }) => request<{ geofences: Geofence[] }>(`/geofences${category ? `?category=${category}` : ""}`, { signal }),
});

const vehiclesQuery = queryOptions({
  queryKey: keys.vehicles,
  queryFn: ({ signal }) => request<{ vehicles: Vehicle[] }>("/vehicles", { signal }),
});

const statusQuery = (id: string) => queryOptions({
  queryKey: keys.vehicle(id),
  queryFn: ({ signal }) => request<VehicleStatus>(`/vehicles/location/${encodeURIComponent(id)}`, { signal }),
});

export const queryDefaults = {
  queries: {
    retry: (count: number, error: Error) => !(error instanceof ApiError) && count < 1,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  },
  mutations: { retry: false },
};

export function useFleet() {
  const client = useQueryClient();
  const fleet = useQuery(vehiclesQuery);
  const vehicles = fleet.data?.vehicles ?? [];
  const statuses = useQueries({ queries: vehicles.map(({ id }) => statusQuery(id)) });

  useEffect(() => {
    let active = false;
    let disposed = false;
    async function refresh() {
      if (active || document.visibilityState !== "visible") return;
      active = true;
      try {
        const { vehicles } = await client.fetchQuery(vehiclesQuery);
        if (!disposed) await Promise.allSettled(vehicles.map(({ id }) => client.fetchQuery(statusQuery(id))));
      } catch {
        // Query state retains the last successful data and exposes the error.
      } finally {
        active = false;
      }
    }
    const timer = window.setInterval(() => void refresh(), 10_000);
    document.addEventListener("visibilitychange", refresh);
    return () => {
      disposed = true;
      window.clearInterval(timer);
      document.removeEventListener("visibilitychange", refresh);
    };
  }, [client]);

  return { fleet, vehicles, statuses };
}
