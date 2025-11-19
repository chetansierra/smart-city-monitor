import { useState, useEffect, useCallback } from 'react';
import TimeSeriesChart from './TimeSeriesChart';
import PollutionBarChart from './PollutionBarChart';
import DateRangeSelector from './DateRangeSelector';
import { getSensorReadings, getTopPollutedAreas } from '../services/api';
import './Analytics.css';

interface AnalyticsProps {
  sensorId?: string;
  autoRefresh?: boolean;
  refreshInterval?: number;
}

const Analytics = ({ sensorId, autoRefresh = true, refreshInterval = 30000 }: AnalyticsProps) => {
  const [timeSeriesData, setTimeSeriesData] = useState<any[]>([]);
  const [pollutionData, setPollutionData] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [dateRange, setDateRange] = useState({ start: '', end: '' });

  const fetchAnalyticsData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      // Try to fetch pollution data
      try {
        const pollutionResult = await getTopPollutedAreas();
        setPollutionData(pollutionResult || []);
      } catch (err) {
        console.warn('Could not fetch pollution data, using sample data:', err);
        // Use sample pollution data
        setPollutionData(generateSamplePollutionData());
      }

      // If a specific sensor is selected, fetch its readings
      if (sensorId && dateRange.start && dateRange.end) {
        try {
          const readingsResult = await getSensorReadings(sensorId, dateRange.start, dateRange.end);
          const transformedData = transformReadingsData(readingsResult);
          setTimeSeriesData(transformedData);
        } catch (err) {
          console.warn('Could not fetch sensor readings, using sample data:', err);
          setTimeSeriesData(generateSampleData());
        }
      } else {
        // Generate sample time series data for demonstration
        setTimeSeriesData(generateSampleData());
      }
    } catch (error: any) {
      console.error('Error fetching analytics data:', error);
      setError(error.message || 'Failed to load analytics data');
      // Use sample data as fallback
      setTimeSeriesData(generateSampleData());
      setPollutionData(generateSamplePollutionData());
    } finally {
      setLoading(false);
    }
  }, [sensorId, dateRange]);

  useEffect(() => {
    // Set initial date range (last 24 hours)
    const now = new Date();
    const yesterday = new Date(now.getTime() - 24 * 60 * 60 * 1000);
    setDateRange({
      start: yesterday.toISOString(),
      end: now.toISOString()
    });
  }, []);

  useEffect(() => {
    if (dateRange.start && dateRange.end) {
      fetchAnalyticsData();
    }
  }, [dateRange, fetchAnalyticsData]);

  // Auto-refresh data
  useEffect(() => {
    if (!autoRefresh) return;

    const interval = setInterval(() => {
      fetchAnalyticsData();
    }, refreshInterval);

    return () => clearInterval(interval);
  }, [autoRefresh, refreshInterval, fetchAnalyticsData]);

  const handleDateRangeChange = (start: string, end: string) => {
    setDateRange({ start, end });
  };

  const transformReadingsData = (readings: any[]) => {
    // Group readings by timestamp
    const grouped = new Map();

    readings.forEach((reading: any) => {
      const timestamp = reading.timestamp;
      if (!grouped.has(timestamp)) {
        grouped.set(timestamp, { timestamp });
      }

      const entry = grouped.get(timestamp);
      entry[reading.reading_type] = reading.value;
    });

    return Array.from(grouped.values()).sort((a, b) =>
      new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime()
    );
  };

  const generateSampleData = () => {
    // Generate sample data for the last 24 hours
    const data = [];
    const now = new Date();

    for (let i = 23; i >= 0; i--) {
      const timestamp = new Date(now.getTime() - i * 60 * 60 * 1000);
      data.push({
        timestamp: timestamp.toISOString(),
        temperature: 20 + Math.random() * 10,
        humidity: 50 + Math.random() * 30,
        pollution: 40 + Math.random() * 60,
        noise: 50 + Math.random() * 30
      });
    }

    return data;
  };

  const generateSamplePollutionData = () => {
    // Generate sample pollution data for different areas
    const areas = ['Downtown', 'Industrial Zone', 'Residential Area', 'Park District', 'Harbor'];
    return areas.map(area => ({
      area,
      pollution: 30 + Math.random() * 120,
      sensor_count: Math.floor(Math.random() * 10) + 1
    })).sort((a, b) => b.pollution - a.pollution);
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
        <DateRangeSelector
          onRangeChange={handleDateRangeChange}
          defaultRange="24h"
        />
      </div>

      <div className="analytics-grid">
        <div className="analytics-full">
          <TimeSeriesChart
            data={timeSeriesData}
            title="Sensor Readings Over Time"
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
            data={timeSeriesData}
            title="Temperature Trend"
            dataKeys={['temperature']}
            height={350}
          />
        </div>
      </div>

      {autoRefresh && (
        <div className="analytics-footer">
          <span className="refresh-indicator">
            🔄 Auto-refreshing every {refreshInterval / 1000}s
          </span>
        </div>
      )}
    </div>
  );
};

export default Analytics;
