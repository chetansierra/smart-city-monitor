import { useEffect, useRef, useState } from 'react';
import { API_BASE_URL, TREND_MAX_POINTS } from '../constants/app';
import { computeSummary } from '../lib/chart';
import type { HomeSummary, LatestReading, MetricKey, SensorRecord, StreamMessage, TrendPoint } from '../types/app';
import { useSSEConnection } from './useSSEConnection';

interface Params {
  sessionId: string;
  hasUserSensor: boolean;
  userSensors: SensorRecord[];
  activeNav: string;
}

export function useLiveSensorData({ sessionId, hasUserSensor, userSensors, activeNav }: Params) {
  const [latestBySensor, setLatestBySensor] = useState<Record<string, LatestReading>>({});
  const [summary, setSummary] = useState<HomeSummary>({ totalSensors: 0, activeSensors: 0, averages: {} });
  const [displaySummary, setDisplaySummary] = useState<HomeSummary>({ totalSensors: 0, activeSensors: 0, averages: {} });
  const [trendHistory, setTrendHistory] = useState<TrendPoint[]>([]);
  const [historyBootstrapped, setHistoryBootstrapped] = useState(false);

  // Keep a ref so the SSE handler always sees the current set of owned sensor IDs
  const userSensorIdSetRef = useRef<Set<string>>(new Set());
  useEffect(() => {
    userSensorIdSetRef.current = new Set(userSensors.map((s) => s.id));
  }, [userSensors]);

  // 30-second baseline poll — merges into existing SSE-accumulated readings (Bug A fix)
  useEffect(() => {
    if (!hasUserSensor || activeNav !== 'home') return;
    let active = true;

    const loadLatest = async () => {
      try {
        const response = await fetch(`${API_BASE_URL}/readings/latest`);
        if (!response.ok) return;
        const payload = await response.json();
        const readings: LatestReading[] = Array.isArray(payload?.data) ? payload.data : [];
        const ids = new Set(userSensors.map((s) => s.id));
        const nextMap: Record<string, LatestReading> = {};
        readings.filter((r) => ids.has(r.sensor_id)).forEach((r) => {
          nextMap[r.sensor_id] = r;
        });
        if (active) {
          setLatestBySensor((prev) => ({ ...prev, ...nextMap }));
        }
      } catch {
        // keep existing state on error
      }
    };

    loadLatest();
    const timer = window.setInterval(loadLatest, 60000);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [hasUserSensor, activeNav, sessionId, userSensors]);

  // History bootstrap — pre-fills trendHistory from stream/history endpoint
  useEffect(() => {
    if (!hasUserSensor || historyBootstrapped || userSensors.length === 0) return;

    const parseTimestamp = (value: string | undefined): number => {
      if (!value) return Date.now();
      const parsed = Date.parse(value);
      return Number.isFinite(parsed) ? parsed : Date.now();
    };

    const bootstrapFromHistory = async () => {
      try {
        const response = await fetch(
          `${API_BASE_URL}/stream/history?session_id=${encodeURIComponent(sessionId)}&limit=400`
        );
        if (!response.ok) {
          setHistoryBootstrapped(true);
          return;
        }
        const payload = await response.json();
        const events: LatestReading[] = Array.isArray(payload?.data) ? payload.data : [];
        const ownedIds = new Set(userSensors.map((s) => s.id));
        const latestMap: Record<string, LatestReading> = {};
        const points: TrendPoint[] = [];

        events.forEach((reading) => {
          if (!reading?.sensor_id || !ownedIds.has(reading.sensor_id)) return;
          latestMap[reading.sensor_id] = reading;
          const snap = computeSummary(userSensors, latestMap);
          points.push({
            timestamp: parseTimestamp(reading.timestamp),
            temperature: snap.averages.temperature,
            humidity: snap.averages.humidity,
            pollution: snap.averages.pollution,
            noise: snap.averages.noise,
          });
        });

        if (points.length > 0) {
          setTrendHistory(points.slice(points.length - TREND_MAX_POINTS));
          setLatestBySensor((prev) => ({ ...prev, ...latestMap }));
        }
      } catch {
        // ignore bootstrap failures; live stream will fill the gap
      } finally {
        setHistoryBootstrapped(true);
      }
    };

    bootstrapFromHistory();
  }, [hasUserSensor, historyBootstrapped, sessionId, userSensors]);

  // Recompute summary whenever latestBySensor or userSensors change
  useEffect(() => {
    if (!hasUserSensor) return;
    setSummary(computeSummary(userSensors, latestBySensor));
  }, [hasUserSensor, latestBySensor, userSensors]);

  // Animate displaySummary towards summary
  useEffect(() => {
    let rafId = 0;
    const durationMs = 220;
    const startAt = performance.now();
    const from = displaySummary;
    const to = summary;

    const tween = (startValue?: number, endValue?: number, progress?: number): number | undefined => {
      if (typeof endValue !== 'number') return undefined;
      const safeStart = typeof startValue === 'number' ? startValue : endValue;
      return Number((safeStart + (endValue - safeStart) * (progress ?? 1)).toFixed(2));
    };

    const step = (now: number) => {
      const progress = Math.min(1, (now - startAt) / durationMs);
      setDisplaySummary({
        totalSensors: to.totalSensors,
        activeSensors: to.activeSensors,
        averages: {
          temperature: tween(from.averages.temperature, to.averages.temperature, progress),
          humidity: tween(from.averages.humidity, to.averages.humidity, progress),
          pollution: tween(from.averages.pollution, to.averages.pollution, progress),
          noise: tween(from.averages.noise, to.averages.noise, progress),
        },
      });
      if (progress < 1) {
        rafId = window.requestAnimationFrame(step);
      }
    };

    rafId = window.requestAnimationFrame(step);
    return () => window.cancelAnimationFrame(rafId);
    // displaySummary intentionally excluded — captured as "from" snapshot to animate from
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [summary]);

  // Append a trend point whenever summary.averages changes
  useEffect(() => {
    if (!hasUserSensor) {
      setTrendHistory([]);
      return;
    }
    const point: TrendPoint = {
      timestamp: Date.now(),
      temperature: summary.averages.temperature,
      humidity: summary.averages.humidity,
      pollution: summary.averages.pollution,
      noise: summary.averages.noise,
    };
    const hasAnyValue = (['temperature', 'humidity', 'pollution', 'noise'] as MetricKey[]).some(
      (metric) => typeof point[metric] === 'number'
    );
    if (!hasAnyValue) return;

    setTrendHistory((prev) => {
      const last = prev[prev.length - 1];
      if (
        last &&
        last.temperature === point.temperature &&
        last.humidity === point.humidity &&
        last.pollution === point.pollution &&
        last.noise === point.noise
      ) {
        return prev;
      }
      const next = [...prev, point];
      return next.length > TREND_MAX_POINTS ? next.slice(next.length - TREND_MAX_POINTS) : next;
    });
  }, [hasUserSensor, summary.averages]);

  // SSE connection for live sensor_update events (shared singleton)
  useSSEConnection(sessionId, hasUserSensor, (payload: StreamMessage) => {
    if (payload.type !== 'sensor_update' || !payload.data) return;
    const reading = payload.data as LatestReading;
    if (!reading.sensor_id || !userSensorIdSetRef.current.has(reading.sensor_id)) return;
    setLatestBySensor((prev) => ({ ...prev, [reading.sensor_id]: reading }));
  });

  return {
    latestBySensor,
    setLatestBySensor,
    summary,
    setSummary,
    displaySummary,
    trendHistory,
    historyBootstrapped,
  };
}
