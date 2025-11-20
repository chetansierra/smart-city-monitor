import { useMemo, useState } from 'react';
import TimeSeriesChart from './TimeSeriesChart';
import PollutionBarChart from './PollutionBarChart';
import DateRangeSelector from './DateRangeSelector';
import { useRealtimeData } from '../context/RealtimeContext';
import './Analytics.css';

interface AnalyticsProps {
  sensorId?: string;
}

const generateSampleData = () => {
  const data = [];
  const now = new Date();

  for (let i = 23; i >= 0; i--) {
    const timestamp = new Date(now.getTime() - i * 60 * 60 * 1000);
    data.push({
      timestamp: timestamp.toISOString(),
      temperature: 20 + Math.random() * 10,
      humidity: 50 + Math.random() * 30,
      pollution: 40 + Math.random() * 60,
      noise: 50 + Math.random() * 30,
    });
  }

  return data;
};

const generateSamplePollutionData = () => {
  const areas = ['Downtown', 'Industrial Zone', 'Residential Area', 'Park District', 'Harbor'];
  return areas
    .map((area) => ({
      area,
      pollution: 30 + Math.random() * 120,
      sensor_count: Math.floor(Math.random() * 10) + 1,
    }))
    .sort((a, b) => b.pollution - a.pollution);
};

const Analytics = ({ sensorId }: AnalyticsProps) => {
  const { cityTrends, pollutionAreas, getSensorHistory, loading } = useRealtimeData();
  const [dateRange, setDateRange] = useState<{ start: string; end: string }>(() => {
    const now = new Date();
    const earlier = new Date(now.getTime() - 12 * 60 * 60 * 1000);
    return { start: earlier.toISOString(), end: now.toISOString() };
  });

  const history = useMemo(() => {
    if (!sensorId) {
      return [];
    }
    return getSensorHistory(sensorId);
  }, [getSensorHistory, sensorId]);

  const filteredHistory = useMemo(() => {
    if (!sensorId || !history.length || !dateRange.start || !dateRange.end) {
      return history;
    }

    return history.filter((entry) => entry.timestamp >= dateRange.start && entry.timestamp <= dateRange.end);
  }, [dateRange.end, dateRange.start, history, sensorId]);

  const sensorSeriesData = useMemo(() => {
    if (!sensorId || filteredHistory.length === 0) {
      return cityTrends.length ? cityTrends : generateSampleData();
    }

    return filteredHistory.map((entry) => ({
      timestamp: entry.timestamp,
      [entry.sensorType]: entry.value,
    }));
  }, [cityTrends, filteredHistory, sensorId]);

  const pollutionData = useMemo(() => {
    if (!pollutionAreas.length) {
      return generateSamplePollutionData();
    }

    return pollutionAreas.map((area) => ({
      area: area.area || area.name,
      pollution: area.pollution,
      sensor_count: 1,
    }));
  }, [pollutionAreas]);

  const cityTemperatureTrend = useMemo(() => {
    if (!cityTrends.length) {
      return generateSampleData();
    }

    return cityTrends.map((point) => ({
      timestamp: point.timestamp,
      temperature: point.temperature,
    }));
  }, [cityTrends]);

  const handleDateRangeChange = (start: string, end: string) => {
    setDateRange({ start, end });
  };

  if (loading) {
    return (
      <div className="analytics-loading">
        <div className="spinner-large"></div>
        <p>Loading analytics...</p>
      </div>
    );
  }

  return (
    <div className="analytics">
      <div className="analytics-header">
        <h2>Analytics Dashboard</h2>
        {sensorId && (
          <DateRangeSelector
            onRangeChange={handleDateRangeChange}
            defaultRange="24h"
          />
        )}
      </div>

      <div className="analytics-grid">
        <div className="analytics-full">
          <TimeSeriesChart
            data={sensorSeriesData}
            title={sensorId ? 'Selected Sensor Timeline' : 'City Trends (Live)'}
            height={350}
          />
        </div>

        <div className="analytics-half">
          <PollutionBarChart
            data={pollutionData}
            title="Top Polluted Areas"
            height={350}
          />
        </div>

        <div className="analytics-half">
          <TimeSeriesChart
            data={cityTemperatureTrend}
            title="Temperature Trend"
            dataKeys={['temperature']}
            height={350}
          />
        </div>
      </div>
    </div>
  );
};

export default Analytics;
