import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import type { Sensor, LatestReading, LatestReadingResponse, Alert } from '../types/sensor';
import type {
  CityStats,
  CityTrendPoint,
  ConnectionStatus,
  PollutionArea,
  SensorHistoryPoint,
} from '../types/realtime';
import { getSensors, getLatestReadings, getCityStats, getAlerts } from '../services/api';

interface RealtimeContextValue {
  sensors: Sensor[];
  latestReadings: Record<string, LatestReading>;
  stats: CityStats | null;
  alerts: Alert[];
  connectionStatus: ConnectionStatus;
  loading: boolean;
  error: string | null;
  cityTrends: CityTrendPoint[];
  pollutionAreas: PollutionArea[];
  getSensorHistory: (sensorId: string) => SensorHistoryPoint[];
}

const RealtimeContext = createContext<RealtimeContextValue | undefined>(undefined);

const DEFAULT_STATS: CityStats = {
  avg_temperature: 0,
  avg_pollution: 0,
  avg_humidity: 0,
  avg_noise: 0,
  total_sensors: 0,
  active_sensors: 0,
  alert_count: 0,
};

const DEFAULT_UNITS: Record<string, string> = {
  temperature: '°C',
  humidity: '%',
  pollution: 'AQI',
  noise: 'dB',
};

type ReadingMap = Record<string, LatestReading>;
type HistoryMap = Record<string, SensorHistoryPoint[]>;

const toNumber = (value: unknown): number | undefined => {
  if (typeof value === 'number' && !Number.isNaN(value)) {
    return value;
  }
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value);
    if (!Number.isNaN(parsed)) {
      return parsed;
    }
  }
  return undefined;
};

const normalizeTimestamp = (value?: string | number): string => {
  // Backend now sends consistent RFC3339Nano timestamps
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Date.parse(value);
    if (!Number.isNaN(parsed)) {
      return new Date(parsed).toISOString();
    }
  }

  // Legacy fallback for numeric timestamps (Unix seconds or milliseconds)
  if (typeof value === 'number') {
    const ms = value > 1e12 ? value : value * 1000;
    return new Date(ms).toISOString();
  }

  // Last resort: current time
  console.warn('Invalid timestamp received, using current time:', value);
  return new Date().toISOString();
};

const normalizeReadingPayload = (
  payload: LatestReadingResponse,
  sensor?: Sensor
): LatestReading | null => {
  const sensorId = payload?.sensor_id || sensor?.id;
  if (!sensorId) {
    return null;
  }

  const sensorType = (payload?.sensor_type || sensor?.type || '').toLowerCase();
  if (!sensorType) {
    return null;
  }

  const value = toNumber(payload?.value);
  const readings: LatestReading['readings'] = {};

  if (sensorType === 'temperature' && typeof value === 'number') {
    readings.temperature = value;
  }
  if (sensorType === 'humidity' && typeof value === 'number') {
    readings.humidity = value;
  }
  if (sensorType === 'pollution' && typeof value === 'number') {
    readings.pollution = value;
  }
  if (sensorType === 'noise' && typeof value === 'number') {
    readings.noise = value;
  }

  const locationCandidate =
    payload?.location || sensor?.location || {
      lat: toNumber(payload?.latitude),
      lon: toNumber(payload?.longitude),
    };

  const location =
    typeof locationCandidate?.lat === 'number' && typeof locationCandidate?.lon === 'number'
      ? {
          lat: locationCandidate.lat,
          lon: locationCandidate.lon,
        }
      : sensor?.location
        ? { ...sensor.location }
        : undefined;

  return {
    sensor_id: sensorId,
    sensor_name: payload?.sensor_name || sensor?.name,
    sensor_type: sensorType,
    value,
    unit: payload?.unit || DEFAULT_UNITS[sensorType] || undefined,
    location,
    readings,
    timestamp: normalizeTimestamp(payload?.timestamp as string | number | undefined),
  };
};

const buildInitialReadingState = (
  readings: LatestReadingResponse[],
  sensorLookup: Map<string, Sensor>
): { readingMap: ReadingMap; historyMap: HistoryMap; latestTimestamp: string } => {
  const readingMap: ReadingMap = {};
  const historyMap: HistoryMap = {};
  let latestTimestamp = '';

  readings.forEach((raw) => {
    const sensor = sensorLookup.get(raw.sensor_id);
    const normalized = normalizeReadingPayload(raw, sensor);
    if (!normalized || !sensor) {
      return;
    }

    readingMap[sensor.id] = normalized;

    if (typeof normalized.value === 'number') {
      historyMap[sensor.id] = [
        {
          sensorId: sensor.id,
          sensorType: normalized.sensor_type,
          value: normalized.value,
          timestamp: normalized.timestamp,
        },
      ];
    }

    if (!latestTimestamp || normalized.timestamp > latestTimestamp) {
      latestTimestamp = normalized.timestamp;
    }
  });

  return { readingMap, historyMap, latestTimestamp };
};

const computeStats = (readingMap: ReadingMap, sensors: Sensor[], alertCount: number): CityStats => {
  if (sensors.length === 0) {
    return { ...DEFAULT_STATS, alert_count: alertCount };
  }

  const buckets: Record<string, { total: number; count: number }> = {
    temperature: { total: 0, count: 0 },
    humidity: { total: 0, count: 0 },
    pollution: { total: 0, count: 0 },
    noise: { total: 0, count: 0 },
  };

  sensors.forEach((sensor) => {
    const latest = readingMap[sensor.id];
    if (!latest) {
      return;
    }

    const key = sensor.type.toLowerCase();
    const bucket = buckets[key];
    if (!bucket) {
      return;
    }

    const value =
      latest.readings[key as keyof LatestReading['readings']] ?? latest.value;

    if (typeof value === 'number') {
      bucket.total += value;
      bucket.count += 1;
    }
  });

  const average = (key: keyof typeof buckets): number => {
    const bucket = buckets[key];
    return bucket.count > 0 ? bucket.total / bucket.count : 0;
  };

  const activeSensors = sensors.filter((sensor) => sensor.status?.toLowerCase() === 'active');

  return {
    avg_temperature: Number(average('temperature').toFixed(2)),
    avg_humidity: Number(average('humidity').toFixed(2)),
    avg_pollution: Number(average('pollution').toFixed(2)),
    avg_noise: Number(average('noise').toFixed(2)),
    total_sensors: sensors.length,
    active_sensors: activeSensors.length,
    alert_count: alertCount,
  };
};

const resolveWsUrl = (): string => {
  const envUrl = import.meta.env.VITE_WS_URL;
  if (envUrl) {
    return envUrl;
  }

  const apiUrl = import.meta.env.VITE_API_URL || 'http://localhost:8080/api/v1';

  try {
    const parsed = new URL(apiUrl);
    parsed.protocol = parsed.protocol === 'https:' ? 'wss:' : 'ws:';
    parsed.pathname = '/ws';
    parsed.search = '';
    return parsed.toString();
  } catch {
    return 'ws://localhost:8080/ws';
  }
};

export const RealtimeProvider = ({ children }: { children: ReactNode }) => {
  const [sensors, setSensors] = useState<Sensor[]>([]);
  const [latestReadings, setLatestReadings] = useState<ReadingMap>({});
  const [stats, setStats] = useState<CityStats | null>(null);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>('connecting');
  const [historyBySensor, setHistoryBySensor] = useState<HistoryMap>({});
  const [cityTrends, setCityTrends] = useState<CityTrendPoint[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [lastStatsTimestamp, setLastStatsTimestamp] = useState<string>('');
  const wsUrl = useMemo(resolveWsUrl, []);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectRef = useRef<number | undefined>(undefined);

  const sensorLookup = useMemo(() => {
    const map = new Map<string, Sensor>();
    sensors.forEach((sensor) => map.set(sensor.id, sensor));
    return map;
  }, [sensors]);

  useEffect(() => {
    let isMounted = true;

    const loadInitialData = async () => {
      try {
        setLoading(true);
        setError(null);

        const [sensorData, statsData, latestData, alertsData] = await Promise.all([
          getSensors(),
          getCityStats().catch(() => DEFAULT_STATS),
          getLatestReadings(),
          getAlerts().catch(() => [] as Alert[]),
        ]);

        if (!isMounted) {
          return;
        }

        const lookup = new Map<string, Sensor>();
        sensorData.forEach((sensor) => lookup.set(sensor.id, sensor));

        const { readingMap, historyMap, latestTimestamp } = buildInitialReadingState(
          latestData,
          lookup
        );

        setSensors(sensorData);
        setLatestReadings(readingMap);
        setHistoryBySensor(historyMap);
        setLastStatsTimestamp(latestTimestamp || new Date().toISOString());
        setStats(statsData ?? DEFAULT_STATS);
        setAlerts(alertsData ?? []);
      } catch (err) {
        console.error(err);
        if (isMounted) {
          setError(err instanceof Error ? err.message : 'Failed to load realtime data');
        }
      } finally {
        if (isMounted) {
          setLoading(false);
        }
      }
    };

    loadInitialData();

    return () => {
      isMounted = false;
    };
  }, []);

  useEffect(() => {
    if (!sensors.length || Object.keys(latestReadings).length === 0) {
      return;
    }

    const computed = computeStats(latestReadings, sensors, alerts.length);
    setStats(computed);
  }, [alerts.length, latestReadings, sensors]);

  useEffect(() => {
    if (!stats || !lastStatsTimestamp) {
      return;
    }

    setCityTrends((prev) => {
      const nextPoint: CityTrendPoint = {
        timestamp: lastStatsTimestamp,
        temperature: stats.avg_temperature,
        humidity: stats.avg_humidity,
        pollution: stats.avg_pollution,
        noise: stats.avg_noise,
      };

      if (prev.length && prev[prev.length - 1].timestamp === nextPoint.timestamp) {
        const updated = [...prev];
        updated[updated.length - 1] = nextPoint;
        return updated;
      }

      const updated = [...prev, nextPoint];
      return updated.length > 200 ? updated.slice(updated.length - 200) : updated;
    });
  }, [stats, lastStatsTimestamp]);

  const appendHistory = useCallback((reading: LatestReading) => {
    if (typeof reading.value !== 'number') {
      return;
    }

    setHistoryBySensor((prev) => {
      const currentHistory = prev[reading.sensor_id] ?? [];
      const updatedHistory = [
        ...currentHistory,
        {
          sensorId: reading.sensor_id,
          sensorType: reading.sensor_type,
          value: reading.value!,
          timestamp: reading.timestamp,
        },
      ];

      return {
        ...prev,
        [reading.sensor_id]:
          updatedHistory.length > 200 ? updatedHistory.slice(updatedHistory.length - 200) : updatedHistory,
      };
    });
  }, []);

  const handleSensorUpdate = useCallback(
    (data: LatestReadingResponse) => {
      const sensor = sensorLookup.get(data.sensor_id);
      if (!sensor) {
        return;
      }

      const normalized = normalizeReadingPayload(data, sensor);
      if (!normalized) {
        return;
      }

      setLatestReadings((prev) => {
        const updated = {
          ...prev,
          [sensor.id]: normalized,
        };
        return updated;
      });

      setLastStatsTimestamp(normalized.timestamp);
      appendHistory(normalized);
    },
    [appendHistory, sensorLookup]
  );

  const handleAlertUpdate = useCallback((data: Alert) => {
    setAlerts((prev) => {
      const updated = [data, ...prev];
      return updated.slice(0, 20);
    });
  }, []);

  useEffect(() => {
    if (!sensors.length) {
      return;
    }

    let manuallyClosed = false;
    let heartbeatInterval: number | undefined;
    let lastPongTimestamp = Date.now();

    const connect = () => {
      setConnectionStatus((status) => (status === 'connected' ? status : 'connecting'));

      const socket = new WebSocket(wsUrl);
      wsRef.current = socket;

      socket.onopen = () => {
        setConnectionStatus('connected');
        socket.send(JSON.stringify({ action: 'subscribe', sensor_id: 'all' }));
        lastPongTimestamp = Date.now();

        // Start heartbeat: send ping every 30 seconds
        heartbeatInterval = window.setInterval(() => {
          if (socket.readyState === WebSocket.OPEN) {
            socket.send(JSON.stringify({ action: 'ping' }));

            // Check if we haven't received pong in 90 seconds
            if (Date.now() - lastPongTimestamp > 90000) {
              console.warn('No pong received in 90s, reconnecting...');
              socket.close();
            }
          }
        }, 30000);
      };

      socket.onmessage = (event) => {
        try {
          const payload = JSON.parse(event.data);
          if (!payload?.type) {
            return;
          }

          if (payload.type === 'sensor_update' && payload.data) {
            handleSensorUpdate(payload.data as LatestReadingResponse);
          } else if (payload.type === 'alert' && payload.data) {
            handleAlertUpdate(payload.data as Alert);
          } else if (payload.type === 'pong') {
            lastPongTimestamp = Date.now();
          }
        } catch (err) {
          console.error('Failed to parse WebSocket message', err);
        }
      };

      socket.onerror = () => {
        setConnectionStatus('error');
      };

      socket.onclose = () => {
        wsRef.current = null;
        if (heartbeatInterval) {
          window.clearInterval(heartbeatInterval);
        }

        if (manuallyClosed) {
          setConnectionStatus('disconnected');
          return;
        }

        setConnectionStatus('reconnecting');
        reconnectRef.current = window.setTimeout(connect, 3000);
      };
    };

    connect();

    return () => {
      manuallyClosed = true;
      if (heartbeatInterval) {
        window.clearInterval(heartbeatInterval);
      }
      if (reconnectRef.current) {
        window.clearTimeout(reconnectRef.current);
      }
      wsRef.current?.close();
    };
  }, [handleAlertUpdate, handleSensorUpdate, sensors.length, wsUrl]);

  const pollutionAreas = useMemo<PollutionArea[]>(() => {
    if (!sensors.length) {
      return [];
    }

    return Object.values(latestReadings)
      .filter((reading) => reading.sensor_type === 'pollution' && typeof reading.value === 'number')
      .map((reading) => {
        const sensor = sensorLookup.get(reading.sensor_id);
        return {
          sensorId: reading.sensor_id,
          name: sensor?.name || reading.sensor_name || 'Sensor',
          area: sensor?.metadata?.area,
          pollution: reading.value as number,
        };
      })
      .sort((a, b) => b.pollution - a.pollution)
      .slice(0, 5);
  }, [latestReadings, sensorLookup, sensors.length]);

  const getSensorHistory = useCallback(
    (sensorId: string) => historyBySensor[sensorId] || [],
    [historyBySensor]
  );

  const value: RealtimeContextValue = {
    sensors,
    latestReadings,
    stats,
    alerts,
    connectionStatus,
    loading,
    error,
    cityTrends,
    pollutionAreas,
    getSensorHistory,
  };

  return <RealtimeContext.Provider value={value}>{children}</RealtimeContext.Provider>;
};

export const useRealtimeData = (): RealtimeContextValue => {
  const context = useContext(RealtimeContext);
  if (!context) {
    throw new Error('useRealtimeData must be used within a RealtimeProvider');
  }
  return context;
};
