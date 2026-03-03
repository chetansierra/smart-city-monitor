import { METRIC_DOMAINS } from '../constants/app';
import type { HomeSummary, LatestReading, MetricKey, SensorRecord, TrendPoint } from '../types/app';

const toNumber = (value: number | string | undefined): number | null => {
  if (typeof value === 'number' && Number.isFinite(value)) return value;
  if (typeof value === 'string') {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : null;
  }
  return null;
};

export const clamp = (value: number, min: number, max: number): number => Math.max(min, Math.min(max, value));

export const computeSummary = (
  sensors: SensorRecord[],
  readingsBySensor: Record<string, LatestReading>
): HomeSummary => {
  const buckets: Record<string, number[]> = {
    temperature: [],
    humidity: [],
    pollution: [],
    noise: [],
  };

  sensors.forEach((sensor) => {
    const reading = readingsBySensor[sensor.id];
    if (!reading) return;
    const key = (reading.sensor_type || sensor.type || '').toLowerCase();
    if (!(key in buckets)) return;
    const value = toNumber(reading.value);
    if (value === null) return;
    buckets[key].push(value);
  });

  const avg = (key: keyof typeof buckets): number | undefined => {
    const values = buckets[key];
    if (!values.length) return undefined;
    return Number((values.reduce((sum, v) => sum + v, 0) / values.length).toFixed(1));
  };

  return {
    totalSensors: sensors.length,
    activeSensors: sensors.length,
    averages: {
      temperature: avg('temperature'),
      humidity: avg('humidity'),
      pollution: avg('pollution'),
      noise: avg('noise'),
    },
  };
};

const resolveMetricDomain = (
  metric: MetricKey,
  zoomLevel: number
): { min: number; max: number } => {
  const base = METRIC_DOMAINS[metric];
  if (zoomLevel <= 1) return base;

  const fullRange = base.max - base.min;
  const targetRange = fullRange / zoomLevel;
  const center = (base.min + base.max) / 2;

  let nextMin = center - targetRange / 2;
  let nextMax = center + targetRange / 2;
  if (nextMin < base.min) {
    nextMax += base.min - nextMin;
    nextMin = base.min;
  }
  if (nextMax > base.max) {
    nextMin -= nextMax - base.max;
    nextMax = base.max;
  }

  return {
    min: clamp(nextMin, base.min, base.max),
    max: clamp(nextMax, base.min, base.max),
  };
};

export const formatMetricName = (metric: MetricKey): string => metric.charAt(0).toUpperCase() + metric.slice(1);

export const buildMetricPath = (
  points: TrendPoint[],
  metric: MetricKey,
  width: number,
  height: number,
  padding: number,
  zoomLevel: number
): string => {
  const values = points
    .map((point) => point[metric])
    .filter((value): value is number => typeof value === 'number' && Number.isFinite(value));
  if (values.length === 0) {
    return '';
  }

  const dynamicMin = Math.min(...values);
  const dynamicMax = Math.max(...values);
  const dynamicRange = dynamicMax - dynamicMin;
  const base = resolveMetricDomain(metric, zoomLevel);
  const fallbackSpan = (base.max - base.min) / Math.max(2, zoomLevel * 3);
  const span = Math.max(dynamicRange * 1.4, fallbackSpan);
  const center = (dynamicMin + dynamicMax) / 2;
  const min = center - span / 2;
  const max = center + span / 2;
  const range = max - min;
  const usableWidth = width - padding * 2;
  const usableHeight = height - padding * 2;

  let path = '';
  points.forEach((point, index) => {
    const value = point[metric];
    if (typeof value !== 'number' || !Number.isFinite(value)) {
      return;
    }
    const clamped = Math.max(min, Math.min(max, value));
    const x = padding + (points.length <= 1 ? 0 : (index / (points.length - 1)) * usableWidth);
    const y = padding + (1 - (clamped - min) / range) * usableHeight;
    path += path ? ` L ${x.toFixed(2)} ${y.toFixed(2)}` : `M ${x.toFixed(2)} ${y.toFixed(2)}`;
  });

  return path;
};
