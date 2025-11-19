import { useState, useMemo } from 'react';
import type { Sensor } from '../types/sensor';
import './SensorSidebar.css';

interface SensorSidebarProps {
  sensors: Sensor[];
  onSensorClick?: (sensor: Sensor) => void;
}

const SensorSidebar = ({ sensors, onSensorClick }: SensorSidebarProps) => {
  const [searchTerm, setSearchTerm] = useState('');
  const [selectedType, setSelectedType] = useState<string>('all');
  const [selectedStatus, setSelectedStatus] = useState<string>('all');

  // Get unique sensor types
  const sensorTypes = useMemo(() => {
    const types = new Set(sensors.map(s => s.type));
    return Array.from(types).sort();
  }, [sensors]);

  // Filter sensors
  const filteredSensors = useMemo(() => {
    return sensors.filter(sensor => {
      const matchesSearch = sensor.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
                          sensor.id.toLowerCase().includes(searchTerm.toLowerCase());
      const matchesType = selectedType === 'all' || sensor.type === selectedType;
      const matchesStatus = selectedStatus === 'all' || sensor.status === selectedStatus;

      return matchesSearch && matchesType && matchesStatus;
    });
  }, [sensors, searchTerm, selectedType, selectedStatus]);

  const getStatusColor = (status: string) => {
    switch (status.toLowerCase()) {
      case 'active':
        return '#22c55e';
      case 'inactive':
        return '#94a3b8';
      case 'maintenance':
        return '#f59e0b';
      default:
        return '#64748b';
    }
  };

  const getTypeIcon = (type: string) => {
    switch (type.toLowerCase()) {
      case 'temperature':
        return '🌡️';
      case 'humidity':
        return '💧';
      case 'pollution':
        return '🏭';
      case 'noise':
        return '🔊';
      default:
        return '📡';
    }
  };

  return (
    <div className="sensor-sidebar">
      <div className="sidebar-header">
        <h2>Sensors</h2>
        <span className="sensor-count">{filteredSensors.length} / {sensors.length}</span>
      </div>

      <div className="sidebar-filters">
        <div className="search-box">
          <input
            type="text"
            placeholder="Search sensors..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="search-input"
          />
          <span className="search-icon">🔍</span>
        </div>

        <div className="filter-group">
          <label htmlFor="type-filter">Type</label>
          <select
            id="type-filter"
            value={selectedType}
            onChange={(e) => setSelectedType(e.target.value)}
            className="filter-select"
          >
            <option value="all">All Types</option>
            {sensorTypes.map(type => (
              <option key={type} value={type}>{type}</option>
            ))}
          </select>
        </div>

        <div className="filter-group">
          <label htmlFor="status-filter">Status</label>
          <select
            id="status-filter"
            value={selectedStatus}
            onChange={(e) => setSelectedStatus(e.target.value)}
            className="filter-select"
          >
            <option value="all">All Status</option>
            <option value="active">Active</option>
            <option value="inactive">Inactive</option>
            <option value="maintenance">Maintenance</option>
          </select>
        </div>
      </div>

      <div className="sensor-list">
        {filteredSensors.length === 0 ? (
          <div className="no-sensors">
            <p>No sensors found</p>
            <small>Try adjusting your filters</small>
          </div>
        ) : (
          filteredSensors.map(sensor => (
            <div
              key={sensor.id}
              className="sensor-item"
              onClick={() => onSensorClick?.(sensor)}
            >
              <div className="sensor-item-icon">
                {getTypeIcon(sensor.type)}
              </div>
              <div className="sensor-item-info">
                <h4 className="sensor-item-name">{sensor.name}</h4>
                <p className="sensor-item-type">{sensor.type}</p>
                {sensor.metadata?.area && (
                  <p className="sensor-item-area">📍 {sensor.metadata.area}</p>
                )}
              </div>
              <div className="sensor-item-status">
                <span
                  className="status-dot"
                  style={{ backgroundColor: getStatusColor(sensor.status) }}
                  title={sensor.status}
                />
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  );
};

export default SensorSidebar;
