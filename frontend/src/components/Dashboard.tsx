import { useMemo, useState } from 'react';
import Map from './Map';
import StatsCard from './StatsCard';
import SensorSidebar from './SensorSidebar';
import Analytics from './Analytics';
import type { Sensor } from '../types/sensor';
import type { CityStats } from '../types/realtime';
import { useRealtimeData } from '../context/RealtimeContext';
import './Dashboard.css';

const Dashboard = () => {
  const { sensors, stats, loading } = useRealtimeData();
  const [selectedSensor, setSelectedSensor] = useState<Sensor | null>(null);

  const statsCopy: CityStats = useMemo(() => stats ?? {
    avg_temperature: 0,
    avg_pollution: 0,
    avg_humidity: 0,
    avg_noise: 0,
    total_sensors: sensors.length,
    active_sensors: sensors.filter((sensor) => sensor.status?.toLowerCase() === 'active').length,
    alert_count: 0,
  }, [stats, sensors]);

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
          value={statsCopy.avg_temperature.toFixed(1)}
          unit="°C"
          icon="🌡️"
          color="#ef4444"
        />
        <StatsCard
          title="Pollution"
          value={statsCopy.avg_pollution.toFixed(0)}
          unit="AQI"
          icon="🏭"
          color="#f59e0b"
        />
        <StatsCard
          title="Humidity"
          value={statsCopy.avg_humidity.toFixed(0)}
          unit="%"
          icon="💧"
          color="#3b82f6"
        />
        <StatsCard
          title="Noise"
          value={statsCopy.avg_noise.toFixed(0)}
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
      />
    </div>
  );
};

export default Dashboard;
