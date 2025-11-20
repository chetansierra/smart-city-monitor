export type ConnectionStatus = 'connecting' | 'connected' | 'reconnecting' | 'disconnected' | 'error';

export interface CityStats {
  avg_temperature: number;
  avg_pollution: number;
  avg_humidity: number;
  avg_noise: number;
  total_sensors: number;
  active_sensors: number;
  alert_count: number;
}

export interface SensorHistoryPoint {
  sensorId: string;
  sensorType: string;
  value: number;
  timestamp: string;
}

export interface CityTrendPoint {
  timestamp: string;
  temperature?: number;
  humidity?: number;
  pollution?: number;
  noise?: number;
}

export interface PollutionArea {
  sensorId: string;
  name: string;
  area?: string;
  pollution: number;
}
