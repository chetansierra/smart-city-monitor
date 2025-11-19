export interface Sensor {
  id: string;
  name: string;
  type: string;
  location: {
    lat: number;
    lon: number;
  };
  status: string;
  metadata?: {
    area?: string;
    installation_date?: string;
    [key: string]: any;
  };
}

export interface SensorReading {
  sensor_id: string;
  reading_type: string;
  value: number;
  unit: string;
  timestamp: string;
  metadata?: {
    [key: string]: any;
  };
}

export interface LatestReading {
  sensor_id: string;
  sensor_name: string;
  sensor_type: string;
  location: {
    lat: number;
    lon: number;
  };
  readings: {
    temperature?: number;
    humidity?: number;
    pollution?: number;
    noise?: number;
  };
  timestamp: string;
}

export interface Alert {
  id: string;
  sensor_id: string;
  sensor_name: string;
  alert_type: string;
  severity: string;
  message: string;
  threshold_value: number;
  current_value: number;
  created_at: string;
  acknowledged: boolean;
}
