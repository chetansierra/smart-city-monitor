import { useCallback, useEffect, useRef, useState } from 'react';
import { API_BASE_URL, GOOGLE_MAPS_API_KEY } from '../constants/app';
import type { NerdStatsData, SensorFootprintData, SessionNerdStatsData, StreamMessage } from '../types/app';
import { useSSEConnection } from './useSSEConnection';

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

  const isNerds = activeNav === 'nerds';

  // Keep fetch functions in refs so refresh() always calls the latest versions
  const fetchFnsRef = useRef<{ global: () => Promise<void>; session: () => Promise<void>; footprint: () => Promise<void> } | null>(null);

  // SSE connection for live stats_update events (shared singleton)
  useSSEConnection(sessionId, isNerds, (payload: StreamMessage) => {
    if (payload.type === 'stats_update' && payload.data) {
      setNerdStats(payload.data as NerdStatsData);
      setNerdStatsError('');
      setNerdStatsLoading(false);
    }
  });

  // Polling for nerd stats, session stats, and global footprint
  useEffect(() => {
    if (!isNerds) return;

    let cancelled = false;

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

    fetchFnsRef.current = { global: fetchGlobalNerdStats, session: fetchSessionNerdStats, footprint: fetchGlobalFootprint };

    setNerdStatsError('');
    void fetchGlobalNerdStats();
    void fetchSessionNerdStats();
    void fetchGlobalFootprint();
    const globalTimer = window.setInterval(fetchGlobalNerdStats, 5000);
    const sessionTimer = window.setInterval(fetchSessionNerdStats, 1000);
    const footprintTimer = window.setInterval(fetchGlobalFootprint, 120000);

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
      fetchFnsRef.current = null;
      window.clearInterval(globalTimer);
      window.clearInterval(sessionTimer);
      window.clearInterval(footprintTimer);
      window.removeEventListener('focus', refreshOnVisible);
      document.removeEventListener('visibilitychange', refreshOnVisible);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isNerds, sessionId]);

  const refresh = useCallback(() => {
    const fns = fetchFnsRef.current;
    if (!fns) return;
    void fns.global();
    void fns.session();
    void fns.footprint();
  }, []);

  return {
    nerdStats,
    sessionNerdStats,
    nerdStatsLoading,
    sessionNerdStatsLoading,
    nerdStatsError,
    globalFootprint,
    globalFootprintLoading,
    refresh,
  };
}
