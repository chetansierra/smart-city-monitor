import type { MetricKey } from '../types/app';

export const THEME_KEY = 'smart_city_theme';
export const SESSION_KEY = 'smart_city_session_id';
export const NAV_KEY = 'smart_city_active_nav';
export const LOCATION_KEY = 'smart_city_user_location';
export const LOCATION_PERMISSION_KEY = 'smart_city_location_permission';
export const METRIC_FILTER_KEY = 'smart_city_metric_filter';
export const ZOOM_LEVEL_KEY = 'smart_city_chart_zoom';
export const NERD_SCOPE_KEY = 'smart_city_nerd_scope';
export const MAPS_SCRIPT_ID = 'smart-city-google-maps-script';

export const API_BASE_URL = (import.meta.env.VITE_API_URL || 'http://localhost:8080/api/v1').trim();
export const GOOGLE_MAPS_API_KEY = import.meta.env.VITE_GOOGLE_MAPS_API_KEY || '';
export const GOOGLE_MAPS_MAP_ID = '5d7a3303ff4999351327c3cf';

export const NEW_DELHI_COORDS = {
  lat: 28.6139,
  lon: 77.209,
};

export const TYPING_LINES = [
  'Hi User,',
  'welcome to the smart city.',
  "let's add your first sensor.",
];

export const TREND_MAX_POINTS = 120;

export const METRIC_KEYS: MetricKey[] = ['temperature', 'humidity', 'pollution', 'noise'];

export const DEFAULT_METRIC_FILTER: Record<MetricKey, boolean> = {
  temperature: true,
  humidity: true,
  pollution: true,
  noise: true,
};

export const SENSOR_TYPE_LABEL: Record<MetricKey, string> = {
  temperature: 'Temperature Sensor',
  humidity: 'Humidity Sensor',
  pollution: 'Pollution Sensor',
  noise: 'Noise Sensor',
};

export const METRIC_COLORS: Record<MetricKey, string> = {
  temperature: '#f97316',
  humidity: '#0ea5e9',
  pollution: '#ef4444',
  noise: '#a855f7',
};

export const METRIC_UNITS: Record<MetricKey, string> = {
  temperature: '°C',
  humidity: '%',
  pollution: 'AQI',
  noise: 'dB',
};

export const METRIC_DOMAINS: Record<MetricKey, { min: number; max: number }> = {
  temperature: { min: -10, max: 55 },
  humidity: { min: 0, max: 100 },
  pollution: { min: 0, max: 300 },
  noise: { min: 30, max: 130 },
};
