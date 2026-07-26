import {
  LngLatBounds,
  Map,
  Marker,
  NavigationControl,
  Popup,
  setWorkerUrl,
} from "maplibre-gl";
import { useEffect, useRef } from "react";
import "maplibre-gl/dist/maplibre-gl.css";
import workerUrl from "maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url";
import type { LocalSighting, SyncStatus } from "../types";
import { useOnline } from "@/hooks/useOnline";
import { StatusBanner } from "./StatusBanner";
import { formatObservedAt } from "@/lib/time";

setWorkerUrl(workerUrl);

const DEFAULT_CENTRE: [number, number] = [-2.5, 54.5];
const DEFAULT_ZOOM = 5;
const MAP_STATUS_COLORS: Record<SyncStatus, string> = {
  pending: "var(--color-muted)",
  synced: "var(--color-success)",
  failed: "var(--color-danger)",
};

type PlacedSighting = LocalSighting & { latitude: number; longitude: number };

export interface SightingMapProps {
  sightings: LocalSighting[];
  birdNameFor: (birdId: string | undefined) => string;
}

export function SightingMap({ sightings, birdNameFor }: SightingMapProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<Map | null>(null);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const map = new Map({
      container,
      style: "https://tiles.openfreemap.org/styles/liberty",
      center: DEFAULT_CENTRE,
      zoom: DEFAULT_ZOOM,
    });

    const control = new NavigationControl({ showCompass: false });
    map.addControl(control);
    mapRef.current = map;

    return () => {
      map.remove();
      mapRef.current = null;
    };
  }, []);

  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;

    const markers = sightings.filter(hasCoords).map((sighting) => {
      const container = document.createElement("div");
      const heading = document.createElement("h3");
      const birdContent = sighting.birdId
        ? birdNameFor(sighting.birdId)
        : "Unidentified bird";
      heading.textContent = birdContent;
      container.appendChild(heading);
      const locationText = document.createElement("p");
      locationText.textContent = formatObservedAt(
        sighting.observedAt,
        sighting.observedAtOffsetMinutes,
      );
      container.appendChild(locationText);

      const color = MAP_STATUS_COLORS[sighting.syncStatus];

      const marker = new Marker({ scale: 1.4, color: color })
        .setLngLat([sighting.longitude, sighting.latitude])
        .addTo(map)
        .setPopup(new Popup().setDOMContent(container).addTo(map));

      return marker;
    });

    return () => {
      for (const marker of markers) marker.remove();
    };
  }, [sightings, birdNameFor]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;

    const coords = sightings.filter(hasCoords);
    if (coords.length === 0) return;

    const bounds = new LngLatBounds(
      [coords[0].longitude, coords[0].latitude],
      [coords[0].longitude, coords[0].latitude],
    );

    for (const sighting of coords) {
      bounds.extend([sighting.longitude, sighting.latitude]);
    }

    map.fitBounds(bounds, { padding: 50, maxZoom: 15 });
  }, [sightings]);

  const online = useOnline();

  return (
    <div className="relative">
      {!online && (
        <div className="absolute inset-x-0 top-0 z-10 p-2">
          <StatusBanner tone="info">
            Map needs a connection; your sightings are safe in the list.
          </StatusBanner>
        </div>
      )}
      <div
        ref={containerRef}
        role="region"
        aria-label="Map of your sightings"
        className="h-[60vh] min-h-96 w-full overflow-hidden rounded-lg"
      ></div>
    </div>
  );
}

function hasCoords(sighting: LocalSighting): sighting is PlacedSighting {
  return sighting.latitude !== undefined && sighting.longitude !== undefined;
}
