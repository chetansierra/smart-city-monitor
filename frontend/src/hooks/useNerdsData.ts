import { useEffect, useRef, useState } from 'react';
import { API_BASE_URL, GOOGLE_MAPS_API_KEY } from '../constants/app';
import type { NerdStatsData, SensorFootprintData, SessionNerdStatsData, StreamMessage } from '../types/app';

interface Params {
  sessionId: string;
  activeNav: string;
}

export function useNerdsData({ sessionId, activeNav }: Params) {
  const [nerdStats, setNerdStats] = useState<NerdStatsData | null>(null);
  const [sessionNerdStats, setSessionNerdStats] = useState<SessionNerdStatsData | null>(null);
  const [nerdStatsLoading, setNerdStatsLoading] = useState(false);
  const [sessionNerdStatsLoading, setSessionNerdStatsLoading] = useState(false);
  const [nerdStatsError, setNerdStatsError] = useState<string>('');
  const [globalFootprint, setGlobalFootprint] = useState<SensorFootprintData | null>(null);
  const [globalFootprintLoading, setGlobalFootprintLoading] = useState(false);
  const lastSessionRefreshRef = useRef(0);

  useEffect(() => {
    if (activeNav !== 'nerds') return;

    let cancelled = false;
    const streamUrl = API_BASE_URL.replace(/\/api\/v1\/?$/, '/stream');
    const streamEndpoint = new URL(streamUrl, window.location.origin);
    streamEndpoint.searchParams.set('session_id', sessionId);

    const fetchGlobalNerdStats = async () => {
      if (!cancelled && !nerdStats) setNerdStatsLoading(true);
      try {
        const response = await fetch(`${API_BASE_URL}/metrics/nerds/global`, {
          cache: 'no-store',
        });
        if (!response.ok) throw new Error('Failed to fetch nerd stats');
        const payload = await response.json();
        if (!cancelled) {
          setNerdStats(payload?.data ?? null);
          setNerdStatsError('');
        }
      } catch {
        if (!cancelled) setNerdStatsError('Could not load nerd stats right now.');
      } finally {
        if (!cancelled) setNerdStatsLoading(false);
      }
    };

    const fetchSessionNerdStats = async () => {
      if (!cancelled) setSessionNerdStatsLoading(true);
      try {
        const response = await fetch(`${API_BASE_URL}/metrics/nerds/session`, {
          cache: 'no-store',
          headers: { 'X-Session-ID': sessionId },
        });
        if (!response.ok) throw new Error('Failed to fetch session nerd stats');
        const payload = await response.json();
        if (!cancelled) {
          setSessionNerdStats(payload?.data ?? null);
          setNerdStatsError('');
        }
      } catch {
        if (!cancelled) setNerdStatsError('Could not load session-level stats right now.');
      } finally {
        if (!cancelled) setSessionNerdStatsLoading(false);
      }
    };

    const fetchGlobalFootprint = async () => {
      if (!GOOGLE_MAPS_API_KEY) return;
      if (!cancelled) setGlobalFootprintLoading(true);
      try {
        const response = await fetch(`${API_BASE_URL}/sensors/footprint?scope=global`, {
          cache: 'no-store',
        });
        if (!response.ok) throw new Error('Failed to fetch global footprint');
        const payload = await response.json();
        if (!cancelled) setGlobalFootprint(payload?.data ?? null);
      } catch {
        if (!cancelled) setNerdStatsError('Could not load global map footprint right now.');
      } finally {
        if (!cancelled) setGlobalFootprintLoading(false);
      }
    };

    setNerdStatsError('');
    void fetchGlobalNerdStats();
    void fetchSessionNerdStats();
    void fetchGlobalFootprint();
    const globalTimer = window.setInterval(fetchGlobalNerdStats, 20000);
    const sessionTimer = window.setInterval(fetchSessionNerdStats, 12000);
    const footprintTimer = window.setInterval(fetchGlobalFootprint, 45000);

    const maybeRefreshSession = () => {
      const now = Date.now();
      if (now - lastSessionRefreshRef.current < 1500) return;
      lastSessionRefreshRef.current = now;
      void fetchSessionNerdStats();
    };

    const source = new EventSource(streamEndpoint.toString());
    source.onmessage = (event) => {
      try {
        const payload = JSON.parse(event.data) as StreamMessage;
        if (payload.type === 'stats_update' && payload.data) {
          if (!cancelled) {
            setNerdStats(payload.data as NerdStatsData);
            setNerdStatsError('');
            setNerdStatsLoading(false);
          }
          maybeRefreshSession();
          return;
        }

        if (payload.type === 'sensor_update') {
          maybeRefreshSession();
        }
      } catch {
        // ignore malformed stream payload
      }
    };
    source.onerror = () => {
      // keep fallback polling active; avoid noisy UI
    };

    const refreshOnVisible = () => {
      if (document.visibilityState !== 'visible') return;
      void fetchGlobalNerdStats();
      void fetchSessionNerdStats();
      void fetchGlobalFootprint();
    };
    window.addEventListener('focus', refreshOnVisible);
    document.addEventListener('visibilitychange', refreshOnVisible);

    return () => {
      cancelled = true;
      source.close();
      window.clearInterval(globalTimer);
      window.clearInterval(sessionTimer);
      window.clearInterval(footprintTimer);
      window.removeEventListener('focus', refreshOnVisible);
      document.removeEventListener('visibilitychange', refreshOnVisible);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeNav, sessionId]);

  return {
    nerdStats,
    sessionNerdStats,
    nerdStatsLoading,
    sessionNerdStatsLoading,
    nerdStatsError,
    globalFootprint,
    globalFootprintLoading,
  };
}
