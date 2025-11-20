import axios from 'axios';
import type { Sensor, LatestReadingResponse, Alert } from '../types/sensor';
import type { CityStats } from '../types/realtime';

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080/api/v1';

const api = axios.create({
  baseURL: API_BASE_URL,
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
});

export const getSensors = async (): Promise<Sensor[]> => {
  const response = await api.get<{ data: Sensor[] }>('/sensors');
  return response.data.data;
};

export const getLatestReadings = async (): Promise<LatestReadingResponse[]> => {
  const response = await api.get<{ data: LatestReadingResponse[] }>('/readings/latest');
  return response.data.data;
};

export const getCityStats = async (): Promise<CityStats> => {
  const response = await api.get<{ data: CityStats }>('/analytics/city-stats');
  return response.data.data;
};

export const getAlerts = async (): Promise<Alert[]> => {
  const response = await api.get<{ data: Alert[] }>('/alerts');
  return response.data.data;
};

export const getSensorReadings = async (
  sensorId: string,
  startTime?: string,
  endTime?: string
) => {
  const params = new URLSearchParams();
  if (startTime) params.append('start_time', startTime);
  if (endTime) params.append('end_time', endTime);

  const response = await api.get(`/sensors/${sensorId}/readings?${params.toString()}`);
  return response.data.data;
};

export const getTopPollutedAreas = async () => {
  const response = await api.get('/analytics/top-polluted');
  return response.data.data;
};

export default api;
