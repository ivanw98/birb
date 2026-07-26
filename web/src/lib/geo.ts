export interface Fix {
  latitude: number;
  longitude: number;
  accuracyM?: number;
}

export function getFix(timeoutMs = 10_000): Promise<Fix | null> {
  if (!("geolocation" in navigator)) {
    return Promise.resolve(null);
  }

  return new Promise((resolve) => {
    navigator.geolocation.getCurrentPosition(
      (pos) =>
        resolve({
          latitude: pos.coords.latitude,
          longitude: pos.coords.longitude,
          accuracyM: pos.coords.accuracy ?? undefined,
        }),
      () => resolve(null), // denied, unavailable, timeout
      { enableHighAccuracy: true, timeout: timeoutMs, maximumAge: 30_000 },
    );
  });
}
