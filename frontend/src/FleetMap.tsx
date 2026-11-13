import { useEffect, useRef } from "react";
import L from "leaflet";
import "leaflet/dist/leaflet.css";
import marker from "leaflet/dist/images/marker-icon.png";
import markerRetina from "leaflet/dist/images/marker-icon-2x.png";
import shadow from "leaflet/dist/images/marker-shadow.png";
import type { Geofence, Position, Vehicle } from "./api";
import { categories } from "./categories";
import styles from "./dashboard.module.css";

const icon = L.icon({ iconUrl: marker, iconRetinaUrl: markerRetina, shadowUrl: shadow, iconSize: [25, 41], iconAnchor: [12, 41], shadowSize: [41, 41], popupAnchor: [1, -34] });
export type LocatedVehicle = { vehicle: Vehicle; position: Position };

export function FleetMap({ zones, located }: { zones: Geofence[]; located: LocatedVehicle[] }) {
  const element = useRef<HTMLDivElement>(null);
  const map = useRef<L.Map | null>(null);
  const zoneLayer = useRef<L.LayerGroup | null>(null);
  const vehicleLayer = useRef<L.LayerGroup | null>(null);
  const fitted = useRef(false);
  const markers = useRef(new Map<string, L.Marker>());

  useEffect(() => {
    const instance = L.map(element.current!).setView([37.7749, -122.4194], 12);
    map.current = instance;
    L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png", {
      maxZoom: 19, attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
    }).addTo(instance);
    zoneLayer.current = L.layerGroup().addTo(instance);
    vehicleLayer.current = L.layerGroup().addTo(instance);
    const resize = new ResizeObserver(() => instance.invalidateSize());
    resize.observe(element.current!);
    return () => {
      resize.disconnect();
      instance.remove();
      markers.current.clear();
      map.current = null;
    };
  }, []);

  useEffect(() => {
    const layer = zoneLayer.current;
    if (!layer) return;
    layer.clearLayers();
    for (const zone of zones) {
      const label = document.createElement("span");
      label.textContent = zone.name;
      L.polygon(zone.coordinates, { color: categories[zone.category].color, weight: 2, fillOpacity: 0.18 }).bindTooltip(label).addTo(layer);
    }
    if (zones.length && !fitted.current) {
      map.current?.fitBounds(L.latLngBounds(zones.flatMap((zone) => zone.coordinates)), { padding: [30, 30], maxZoom: 15 });
      fitted.current = true;
    }
  }, [zones]);

  useEffect(() => {
    const layer = vehicleLayer.current;
    if (!layer) return;
    const present = new Set<string>();
    for (const { vehicle, position } of located) {
      present.add(vehicle.id);
      const point: L.LatLngTuple = [position.latitude, position.longitude];
      const existing = markers.current.get(vehicle.id);
      if (existing) {
        if (!existing.getLatLng().equals(point, 0)) existing.setLatLng(point);
      } else {
        const label = document.createElement("span");
        label.textContent = vehicle.vehicle_number;
        markers.current.set(vehicle.id, L.marker(point, { icon, title: vehicle.vehicle_number }).bindTooltip(label).addTo(layer));
      }
    }
    for (const [id, item] of markers.current) {
      if (!present.has(id)) {
        layer.removeLayer(item);
        markers.current.delete(id);
      }
    }
    if (!fitted.current && located.length) {
      map.current?.fitBounds(L.latLngBounds(located.map(({ position }) => [position.latitude, position.longitude])), { padding: [30, 30], maxZoom: 15 });
      fitted.current = true;
    }
  }, [located]);

  return <div ref={element} className={styles.map} aria-label="Fleet map" />;
}
