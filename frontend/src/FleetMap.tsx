import { useEffect, useRef } from "react";
import L from "leaflet";
import "leaflet/dist/leaflet.css";
import "@geoman-io/leaflet-geoman-free";
import "@geoman-io/leaflet-geoman-free/dist/leaflet-geoman.css";
import marker from "leaflet/dist/images/marker-icon.png";
import markerRetina from "leaflet/dist/images/marker-icon-2x.png";
import shadow from "leaflet/dist/images/marker-shadow.png";
import type { MapMode } from "./FleetForms";
import type { Coordinates, Geofence, Position, Vehicle } from "./api";
import { categories } from "./categories";
import styles from "./dashboard.module.css";

const icon = L.icon({ iconUrl: marker, iconRetinaUrl: markerRetina, shadowUrl: shadow, iconSize: [25, 41], iconAnchor: [12, 41], shadowSize: [41, 41], popupAnchor: [1, -34] });
export type LocatedVehicle = { vehicle: Vehicle; position: Position };

export function FleetMap({ zones, located, mode, draft, onDraw, onPick, picked, focus }: {
  zones: Geofence[]; located: LocatedVehicle[]; mode: MapMode; draft: Coordinates[] | null;
  picked: Coordinates | null; focus: { id: string } | null;
  onDraw: (ring: Coordinates[]) => void; onPick: (point: Coordinates) => void;
}) {
  const element = useRef<HTMLDivElement>(null);
  const map = useRef<L.Map | null>(null);
  const zoneLayer = useRef<L.LayerGroup | null>(null);
  const vehicleLayer = useRef<L.LayerGroup | null>(null);
  const draftLayer = useRef<L.LayerGroup | null>(null);
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
    draftLayer.current = L.layerGroup().addTo(instance);
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
        markers.current.set(vehicle.id, L.marker(point, { icon, title: vehicle.vehicle_number, bubblingMouseEvents: true }).bindTooltip(label).addTo(layer));
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

  useEffect(() => {
    const instance = map.current;
    if (!instance) return;
    if (mode === "draw") {
      const created = (event: L.LeafletEvent) => {
        const layer = (event as L.LeafletEvent & { layer: L.Polygon }).layer;
        const vertices = (layer.getLatLngs() as L.LatLng[][])[0];
        const ring: Coordinates[] = vertices.map(({ lat, lng }) => [lat, lng]);
        layer.remove();
        const first = ring[0];
        const last = ring[ring.length - 1];
        if (first[0] !== last[0] || first[1] !== last[1]) ring.push(first);
        onDraw(ring);
      };
      instance.on("pm:create", created);
      instance.pm.enableDraw("Polygon", { allowSelfIntersection: false, snappable: false });
      return () => {
        instance.off("pm:create", created);
        instance.pm.disableDraw("Polygon");
      };
    }
    if (mode === "pick") {
      const pick = (event: L.LeafletMouseEvent) => onPick([event.latlng.lat, event.latlng.wrap().lng]);
      instance.on("click", pick);
      return () => { instance.off("click", pick); };
    }
  }, [mode, onDraw, onPick]);

  useEffect(() => {
    const layer = draftLayer.current;
    layer?.clearLayers();
    if (layer && draft && mode !== "draw") {
      L.polygon(draft, { color: "#526b82", dashArray: "6 4" }).addTo(layer);
    }
  }, [draft, mode]);

  useEffect(() => {
    if (!picked || !map.current) return;
    const pin = L.marker(picked, {
      icon: L.divIcon({ className: styles.pickedPin, iconSize: [24, 24], iconAnchor: [12, 24] }),
      interactive: false, keyboard: false, zIndexOffset: 1000,
    }).addTo(map.current);
    return () => { pin.remove(); };
  }, [picked]);

  useEffect(() => {
    if (!focus) return;
    const marker = markers.current.get(focus.id);
    if (marker) {
      map.current?.panTo(marker.getLatLng());
      marker.openTooltip();
    }
  }, [focus]);

  return <div ref={element} className={mode === "pick" ? `${styles.map} ${styles.picking}` : styles.map} aria-label="Fleet map" />;
}
