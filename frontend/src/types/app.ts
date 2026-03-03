export type Theme = 'light' | 'dark';
export type NavItem = 'home' | 'nerds';
export type MetricKey = 'temperature' | 'humidity' | 'pollution' | 'noise';
export type LocationPermission = 'granted' | 'denied' | 'unknown';

export interface SensorRecord {
  id: string;
  session_id?: string;
  name?: string;
  type?: string;
  location?: {
    latitude?: number;
    longitude?: number;
  };
}

export interface LatestReading {
  sensor_id: string;
  sensor_type?: string;
  value?: number | string;
  timestamp?: string;
}

export interface StreamMessage {
  type?: string;
  data?: unknown;
}

export interface HomeSummary {
  totalSensors: number;
  activeSensors: number;
  averages: {
    temperature?: number;
    humidity?: number;
    pollution?: number;
    noise?: number;
  };
}

export interface TrendPoint {
  timestamp: number;
  temperature?: number;
  humidity?: number;
  pollution?: number;
  noise?: number;
}

export interface SessionConfig {
  origin_lat: number;
  origin_lon: number;
  spawn_radius_km: number;
}

export interface SensorActionState {
  type: MetricKey;
  action: 'add' | 'remove';
}

export interface NerdStatsData {
  system: {
    uptime_sec: number;
    goroutines: number;
    heap_alloc_mb: number;
    heap_sys_mb: number;
    gc_cycles_total: number;
  };
  database: {
    sensors: number;
    readings: number;
  };
  redis: {
    key_count: number;
    ping_latency_ms: number;
    connected_clients: number;
    total_commands_processed: number;
  };
  kafka: {
    broker_count: number;
    topic_count: number;
    total_partitions: number;
    total_messages: number;
  };
  sse: {
    active_clients: number;
  };
  sessions: {
    total_visitors: number;
    active_now: number;
  };
  performance: {
    estimated_throughput_msg_sec: number;
  };
  platform?: {
    sessions_with_sensors: number;
    avg_readings_per_sensor: number;
  };
  timestamp: string;
}

export interface SessionNerdStatsData {
  session: {
    session_id: string;
    sensors: number;
    readings: number;
    readings_last_min: number;
    avg_sensor_value: number;
    last_reading_at?: string;
  };
  realtime: {
    estimated_throughput_msg_sec: number;
    active_sse_clients: number;
  };
  system: {
    uptime_sec: number;
    goroutines: number;
    global_goroutines?: number;
    session_worker_goroutines?: number;
  };
  timestamp: string;
}

export interface SensorFootprintPoint {
  id: string;
  type: string;
  lat: number;
  lon: number;
}

export interface SensorFootprintData {
  scope: 'session' | 'global';
  total: number;
  points: SensorFootprintPoint[];
  counts: Record<string, number>;
}
