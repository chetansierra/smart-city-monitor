import { useEffect, useMemo, useRef, useState, type PointerEvent } from 'react';
import { API_BASE_URL, METRIC_KEYS } from '../constants/app';
import type { HomeSummary, LatestReading, MetricKey, SensorRecord } from '../types/app';

const MAX_SENSORS_PER_SESSION = 20;

interface Params {
  sessionId: string;
  sessionConfigReady: boolean;
  isUpdatingRadius: boolean;
  setLatestBySensor: (updater: (prev: Record<string, LatestReading>) => Record<string, LatestReading>) => void;
  hasUserSensor: boolean;
  setHasUserSensor: (value: boolean) => void;
  userSensors: SensorRecord[];
  setUserSensors: (updater: (prev: SensorRecord[]) => SensorRecord[]) => void;
  setSummary: (updater: (prev: HomeSummary) => HomeSummary) => void;
}

export function useUserSensors({
  sessionId,
  sessionConfigReady,
  isUpdatingRadius,
  setLatestBySensor,
  hasUserSensor,
  setHasUserSensor,
  userSensors,
  setUserSensors,
  setSummary,
}: Params) {
  const [isCreatingSensor, setIsCreatingSensor] = useState(false);
  const [statusMessage, setStatusMessage] = useState<string>('');

  // Ref kept in sync so event handlers see the latest list without stale closures
  const userSensorsRef = useRef<SensorRecord[]>(userSensors);
  useEffect(() => {
    userSensorsRef.current = userSensors;
  }, [userSensors]);

  const holdStateRef = useRef<{ cancelled: boolean; timerId: number | null } | null>(null);
  const suppressNextClickRef = useRef(false);

  // Initial fetch of this session's sensors
  useEffect(() => {
    const checkUserSensors = async () => {
      try {
        const response = await fetch(`${API_BASE_URL}/sensors?page=1&limit=1000`);
        if (!response.ok) return;
        const payload = await response.json();
        const sensors: SensorRecord[] = Array.isArray(payload?.data) ? payload.data : [];
        const owned = sensors.filter(
          (sensor) => sensor?.session_id?.toLowerCase() === sessionId.toLowerCase()
        );
        setUserSensors(() => owned);
        setHasUserSensor(owned.length > 0);
      } catch {
        // keep UI usable even if API is unavailable
      }
    };

    checkUserSensors();
  }, [sessionId, setHasUserSensor, setUserSensors]);

  const userSensorCounts = useMemo(
    () =>
      userSensors.reduce<Record<MetricKey, number>>(
        (acc, sensor) => {
          const key = (sensor.type || '').toLowerCase() as MetricKey;
          if (METRIC_KEYS.includes(key)) acc[key] += 1;
          return acc;
        },
        { temperature: 0, humidity: 0, pollution: 0, noise: 0 }
      ),
    [userSensors]
  );

  const handleAddSensor = async (sensorType: MetricKey = 'temperature') => {
    if (isCreatingSensor || !sessionConfigReady) return;
    if (userSensorsRef.current.length >= MAX_SENSORS_PER_SESSION) {
      setStatusMessage(`Maximum ${MAX_SENSORS_PER_SESSION} sensors per session.`);
      return;
    }
    setIsCreatingSensor(true);
    setStatusMessage('');

    try {
      const response = await fetch(`${API_BASE_URL}/sensors/start?type=${sensorType}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-Session-ID': sessionId },
        body: '{}',
      });

      if (!response.ok) {
        if (response.status === 409) {
          try {
            const errPayload = await response.json();
            const code = errPayload?.error?.code;
            const msg = errPayload?.error?.message;
            if (code === 'GLOBAL_SENSOR_LIMIT_REACHED') {
              setStatusMessage(msg || 'Global sensor limit reached. Please try again later.');
            } else {
              setStatusMessage(msg || `Maximum ${MAX_SENSORS_PER_SESSION} sensors per session.`);
            }
          } catch {
            setStatusMessage(`Maximum ${MAX_SENSORS_PER_SESSION} sensors per session.`);
          }
          return;
        }
        throw new Error('Sensor creation failed');
      }

      const payload = await response.json();
      const createdSensor = payload?.data as SensorRecord | undefined;
      setHasUserSensor(true);
      setSummary((prev) => ({
        ...prev,
        totalSensors: Math.max(1, prev.totalSensors + 1),
        activeSensors: Math.max(1, prev.activeSensors + 1),
      }));
      if (createdSensor?.id) {
        setUserSensors((prev) => [createdSensor, ...prev]);
      }
    } catch {
      setStatusMessage('Could not add sensor right now. Please try again.');
    } finally {
      setIsCreatingSensor(false);
    }
  };

  const handleDeleteSensor = async (sensorId: string) => {
    if (isCreatingSensor) return;
    setStatusMessage('');
    try {
      const response = await fetch(`${API_BASE_URL}/sensors/${sensorId}`, {
        method: 'DELETE',
        headers: { 'X-Session-ID': sessionId },
      });
      if (!response.ok) throw new Error('Sensor deletion failed');

      setUserSensors((prev) => {
        const next = prev.filter((sensor) => sensor.id !== sensorId);
        setHasUserSensor(next.length > 0);
        setLatestBySensor((oldMap) => {
          const nextMap = { ...oldMap };
          delete nextMap[sensorId];
          return nextMap;
        });
        return next;
      });
    } catch {
      setStatusMessage('Could not remove sensor right now. Please try again.');
    }
  };

  const handleAdjustSensorCount = async (sensorType: MetricKey, direction: 'up' | 'down') => {
    if (direction === 'up') {
      await handleAddSensor(sensorType);
      return;
    }
    const target = userSensors.find((sensor) => (sensor.type || '').toLowerCase() === sensorType);
    if (!target) {
      setStatusMessage(`No ${sensorType} sensor available to remove.`);
      return;
    }
    await handleDeleteSensor(target.id);
  };

  const getSensorCountByType = (sensorType: MetricKey): number =>
    userSensorsRef.current.filter(
      (sensor) => ((sensor.type || '').toLowerCase() as MetricKey) === sensorType
    ).length;

  const stopContinuousAdjust = () => {
    const holdState = holdStateRef.current;
    if (!holdState) return;
    holdState.cancelled = true;
    if (holdState.timerId !== null) window.clearTimeout(holdState.timerId);
    holdStateRef.current = null;
  };

  const runContinuousAdjust = async (
    sensorType: MetricKey,
    direction: 'up' | 'down',
    delayMs: number
  ) => {
    const holdState = holdStateRef.current;
    if (!holdState || holdState.cancelled) return;

    if (direction === 'up' && userSensorsRef.current.length >= MAX_SENSORS_PER_SESSION) {
      stopContinuousAdjust();
      return;
    }
    if (direction === 'down' && getSensorCountByType(sensorType) <= 0) {
      stopContinuousAdjust();
      return;
    }

    await handleAdjustSensorCount(sensorType, direction);

    const current = holdStateRef.current;
    if (!current || current.cancelled) return;
    if (direction === 'up' && userSensorsRef.current.length >= MAX_SENSORS_PER_SESSION) {
      stopContinuousAdjust();
      return;
    }
    if (direction === 'down' && getSensorCountByType(sensorType) <= 0) {
      stopContinuousAdjust();
      return;
    }

    const nextDelay = Math.max(80, Math.floor(delayMs * 0.8));
    current.timerId = window.setTimeout(() => {
      void runContinuousAdjust(sensorType, direction, nextDelay);
    }, delayMs);
  };

  const startContinuousAdjust = (sensorType: MetricKey, direction: 'up' | 'down') => {
    if (holdStateRef.current || isUpdatingRadius || !sessionConfigReady) return;
    const holdState = { cancelled: false, timerId: null as number | null };
    holdStateRef.current = holdState;
    holdState.timerId = window.setTimeout(() => {
      const current = holdStateRef.current;
      if (!current || current.cancelled) return;
      suppressNextClickRef.current = true;
      void runContinuousAdjust(sensorType, direction, 260);
    }, 260);
  };

  const handleAdjustClick = async (sensorType: MetricKey, direction: 'up' | 'down') => {
    if (suppressNextClickRef.current) {
      suppressNextClickRef.current = false;
      return;
    }
    await handleAdjustSensorCount(sensorType, direction);
  };

  const handleAdjustPointerDown = (
    event: PointerEvent<HTMLButtonElement>,
    sensorType: MetricKey,
    direction: 'up' | 'down'
  ) => {
    if (event.pointerType === 'mouse' && event.button !== 0) return;
    startContinuousAdjust(sensorType, direction);
  };

  // Clean up any in-flight hold timer on unmount
  useEffect(() => stopContinuousAdjust, []);

  return {
    isCreatingSensor,
    statusMessage,
    setStatusMessage,
    handleAddSensor,
    handleDeleteSensor,
    handleAdjustClick,
    handleAdjustPointerDown,
    stopContinuousAdjust,
    holdStateRef,
    suppressNextClickRef,
    userSensorsRef,
    userSensorCounts,
    hasUserSensor,
  };
}
