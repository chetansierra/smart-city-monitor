import { useEffect, useState } from 'react';
import Map from './Map';
import StatsCard from './StatsCard';
import SensorSidebar from './SensorSidebar';
import Analytics from './Analytics';
import { getSensors, getCityStats } from '../services/api';
import type { Sensor } from '../types/sensor';
import './Dashboard.css';

interface CityStats {
  avg_temperature: number;
  avg_pollution: number;
  avg_humidity: number;
  avg_noise: number;
  total_sensors: number;
  active_sensors: number;
  alert_count: number;
}

const Dashboard = () => {
  const [sensors, setSensors] = useState<Sensor[]>([]);
  const [stats, setStats] = useState<CityStats | null>(null);
  const [loading, setLoading] = useState(true);
  const [selectedSensor, setSelectedSensor] = useState<Sensor | null>(null);

  useEffect(() => {
    const fetchData = async () => {
      try {
        setLoading(true);
        const [sensorsData, statsData] = await Promise.all([
          getSensors(),
          getCityStats()
        ]);

        setSensors(sensorsData);
        setStats(statsData);
      } catch (error) {
        console.error('Error fetching dashboard data:', error);
      } finally {
        setLoading(false);
      }
    };

    fetchData();
  }, []);

  const handleSensorClick = (sensor: Sensor) => {
    setSelectedSensor(sensor);
    // Could add map pan/zoom to sensor location here
  };

  if (loading) {
    return (
      <div className="dashboard-loading">
        <div className="spinner-large"></div>
        <p>Loading dashboard...</p>
      </div>
    );
  }

  return (
    <div className="dashboard">
      {/* Stats Grid */}
      <div className="dashboard-stats">
        <StatsCard
          title="Temperature"
          value={stats?.avg_temperature.toFixed(1) || '0'}
          unit="°C"
          icon="🌡️"
          color="#ef4444"
        />
        <StatsCard
          title="Pollution"
          value={stats?.avg_pollution.toFixed(0) || '0'}
          unit="AQI"
          icon="🏭"
          color="#f59e0b"
        />
        <StatsCard
          title="Humidity"
          value={stats?.avg_humidity.toFixed(0) || '0'}
          unit="%"
          icon="💧"
          color="#3b82f6"
        />
        <StatsCard
          title="Noise"
          value={stats?.avg_noise.toFixed(0) || '0'}
          unit="dB"
          icon="🔊"
          color="#8b5cf6"
        />
      </div>

      {/* Main Content Area */}
      <div className="dashboard-main">
        {/* Sidebar */}
        <aside className="dashboard-sidebar">
          <SensorSidebar
            sensors={sensors}
            onSensorClick={handleSensorClick}
          />
        </aside>

        {/* Map Area */}
        <div className="dashboard-map">
          <Map />
        </div>
      </div>

      {/* Analytics Section */}
      <Analytics
        sensorId={selectedSensor?.id}
        autoRefresh={true}
        refreshInterval={30000}
      />
    </div>
  );
};

export default Dashboard;
