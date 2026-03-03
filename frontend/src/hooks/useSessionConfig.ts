import { useEffect, useState } from 'react';
import { API_BASE_URL, LOCATION_KEY, LOCATION_PERMISSION_KEY, NEW_DELHI_COORDS } from '../constants/app';
import { clamp } from '../lib/chart';
import type { LocationPermission, SessionConfig } from '../types/app';

interface Params {
  sessionId: string;
}

export function useSessionConfig({ sessionId }: Params) {
  const [sessionConfig, setSessionConfig] = useState<SessionConfig>({
    origin_lat: NEW_DELHI_COORDS.lat,
    origin_lon: NEW_DELHI_COORDS.lon,
    spawn_radius_km: 5,
  });
  const [sessionConfigReady, setSessionConfigReady] = useState(false);
  const [isUpdatingRadius, setIsUpdatingRadius] = useState(false);
  const [locationPermission, setLocationPermission] = useState<LocationPermission>(() => {
    const stored = localStorage.getItem(LOCATION_PERMISSION_KEY);
    if (stored === 'granted' || stored === 'denied') return stored;
    return 'unknown';
  });

  useEffect(() => {
    const headers = {
      'Content-Type': 'application/json',
      'X-Session-ID': sessionId,
    };

    const readCurrentConfig = async (): Promise<SessionConfig | null> => {
      try {
        const response = await fetch(`${API_BASE_URL}/session/config`, { headers });
        if (!response.ok) return null;
        const payload = await response.json();
        if (!payload?.data) return null;
        const data = payload.data as SessionConfig;
        return {
          origin_lat: Number(data.origin_lat),
          origin_lon: Number(data.origin_lon),
          spawn_radius_km: clamp(Number(data.spawn_radius_km) || 5, 1, 100),
        };
      } catch {
        return null;
      }
    };

    const persistConfig = async (next: SessionConfig) => {
      try {
        await fetch(`${API_BASE_URL}/session/config`, {
          method: 'PUT',
          headers,
          body: JSON.stringify(next),
        });
      } catch {
        // non-blocking; UI keeps safe fallback
      }
    };

    const initSessionConfig = async () => {
      const base = (await readCurrentConfig()) ?? {
        origin_lat: NEW_DELHI_COORDS.lat,
        origin_lon: NEW_DELHI_COORDS.lon,
        spawn_radius_km: 5,
      };

      if (!navigator.geolocation) {
        setLocationPermission('denied');
        localStorage.setItem(LOCATION_PERMISSION_KEY, 'denied');
        setSessionConfig(base);
        setSessionConfigReady(true);
        await persistConfig(base);
        return;
      }

      try {
        const position = await new Promise<GeolocationPosition>((resolve, reject) => {
          navigator.geolocation.getCurrentPosition(resolve, reject, {
            enableHighAccuracy: false,
            timeout: 5000,
            maximumAge: 10 * 60 * 1000,
          });
        });
        const nextConfig: SessionConfig = {
          origin_lat: position.coords.latitude,
          origin_lon: position.coords.longitude,
          spawn_radius_km: clamp(base.spawn_radius_km, 1, 100),
        };
        localStorage.setItem(
          LOCATION_KEY,
          JSON.stringify({ lat: nextConfig.origin_lat, lon: nextConfig.origin_lon })
        );
        localStorage.setItem(LOCATION_PERMISSION_KEY, 'granted');
        setLocationPermission('granted');
        setSessionConfig(nextConfig);
        setSessionConfigReady(true);
        await persistConfig(nextConfig);
      } catch {
        const fallback: SessionConfig = {
          origin_lat: NEW_DELHI_COORDS.lat,
          origin_lon: NEW_DELHI_COORDS.lon,
          spawn_radius_km: clamp(base.spawn_radius_km, 1, 100),
        };
        localStorage.setItem(LOCATION_PERMISSION_KEY, 'denied');
        setLocationPermission('denied');
        setSessionConfig(fallback);
        setSessionConfigReady(true);
        await persistConfig(fallback);
      }
    };

    initSessionConfig();
  }, [sessionId]);

  const handleRadiusChange = async (nextRadiusKm: number) => {
    const clampedRadius = clamp(nextRadiusKm, 1, 100);
    setSessionConfig((prev) => ({ ...prev, spawn_radius_km: clampedRadius }));
    setIsUpdatingRadius(true);
    try {
      const response = await fetch(`${API_BASE_URL}/session/config`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'X-Session-ID': sessionId,
        },
        body: JSON.stringify({ spawn_radius_km: clampedRadius }),
      });
      if (!response.ok) throw new Error('Failed to update spawn radius');
    } catch {
      // status message handled by caller if needed
    } finally {
      setIsUpdatingRadius(false);
    }
  };

  return {
    sessionConfig,
    setSessionConfig,
    sessionConfigReady,
    isUpdatingRadius,
    locationPermission,
    setLocationPermission,
    handleRadiusChange,
  };
}
