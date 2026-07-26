import { useState, useEffect } from "react";

export type GeoPermission = "granted" | "prompt" | "denied" | "unknown";

// Subscribe to the geolocation permission state, unhook the listener when the component unmount
export function useGeoPermission(): GeoPermission {
  const [state, setState] = useState<GeoPermission>("unknown");

  useEffect(() => {
    let cancelled = false;
    let status: PermissionStatus | undefined;

    navigator.permissions
      ?.query({ name: "geolocation" })
      .then((s) => {
        if (cancelled) return;
        status = s;
        setState(s.state);
        s.onchange = () => setState(s.state);
      })
      .catch(() => setState("unknown"));
    return () => {
      cancelled = true;
      if (status) status.onchange = null;
    };
  }, []);

  return state;
}
