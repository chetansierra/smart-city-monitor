import { useMemo } from 'react';
import { MapContainer, TileLayer, Marker, Popup } from 'react-leaflet';
import L from 'leaflet';
import type { Sensor, LatestReading } from '../types/sensor';
import { useRealtimeData } from '../context/RealtimeContext';
import './Map.css';

// Fix for default marker icons in React-Leaflet
import icon from 'leaflet/dist/images/marker-icon.png';
import iconShadow from 'leaflet/dist/images/marker-shadow.png';

const DefaultIcon = L.icon({
  iconUrl: icon,
  shadowUrl: iconShadow,
  iconSize: [25, 41],
  iconAnchor: [12, 41],
  popupAnchor: [1, -34],
  shadowSize: [41, 41]
});

L.Marker.prototype.options.icon = DefaultIcon;

// Create custom icons based on sensor type
const createSensorIcon = (type: string, severity?: string) => {
  const colors: { [key: string]: string } = {
    temperature: '#FF6B6B',
    pollution: '#4ECDC4',
    humidity: '#45B7D1',
    noise: '#FFA07A',
    default: '#95E1D3'
  };

  const color = colors[type.toLowerCase()] || colors.default;
  const alertColor = severity === 'critical' ? '#FF0000' : severity === 'warning' ? '#FFA500' : color;

  return L.divIcon({
    className: 'custom-marker',
    html: `
      <div style="
        background-color: ${alertColor};
        width: 30px;
        height: 30px;
        border-radius: 50%;
        border: 3px solid white;
        box-shadow: 0 2px 5px rgba(0,0,0,0.3);
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 12px;
        color: white;
        font-weight: bold;
      ">
        ${type.charAt(0).toUpperCase()}
      </div>
    `,
    iconSize: [30, 30],
    iconAnchor: [15, 15],
    popupAnchor: [0, -15]
  });
};

interface SensorWithReading extends Sensor {
  latestReading?: LatestReading;
}

const Map = () => {
  const { sensors, latestReadings, loading, error } = useRealtimeData();
  const sensorsWithReadings: SensorWithReading[] = useMemo(
    () =>
      sensors.map((sensor) => ({
        ...sensor,
        latestReading: latestReadings[sensor.id],
      })),
    [latestReadings, sensors]
  );

  // Default center (can be adjusted to your city)
  const defaultCenter: [number, number] = [40.7128, -74.0060]; // New York
  const defaultZoom = 12;

  const formatReadingValue = (value: number | undefined, unit: string = '') => {
    if (value === undefined) return 'N/A';
    return `${value.toFixed(2)}${unit}`;
  };

  if (loading) {
    return (
      <div className="map-loading">
        <div className="spinner"></div>
        <p>Loading map and sensors...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="map-error">
        <p>{error}</p>
        <button onClick={() => window.location.reload()}>Retry</button>
      </div>
    );
  }

  // Calculate center based on sensors if available
  const findValidSensorPosition = (): [number, number] | null => {
    for (const sensor of sensors) {
      const lat = sensor.location?.lat;
      const lon = sensor.location?.lon;
      if (typeof lat === 'number' && typeof lon === 'number') {
        return [lat, lon];
      }
    }
    return null;
  };

  const mapCenter: [number, number] = findValidSensorPosition() || defaultCenter;

  return (
    <div className="map-container">
      <MapContainer
        center={mapCenter}
        zoom={defaultZoom}
        style={{ height: '100%', width: '100%' }}
      >
        <TileLayer
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
        />

        {sensorsWithReadings.map((sensor) => {
          const lat = sensor.location?.lat;
          const lon = sensor.location?.lon;
          if (typeof lat !== 'number' || typeof lon !== 'number') {
            return null;
          }
          return (
          <Marker
            key={sensor.id}
            position={[lat, lon]}
            icon={createSensorIcon(sensor.type)}
          >
            <Popup>
              <div className="sensor-popup">
                <h3>{sensor.name}</h3>
                <div className="sensor-info">
                  <p><strong>Type:</strong> {sensor.type}</p>
                  <p><strong>Status:</strong>
                    <span className={`status-badge status-${sensor.status}`}>
                      {sensor.status}
                    </span>
                  </p>
                  {sensor.metadata?.area && (
                    <p><strong>Area:</strong> {sensor.metadata.area}</p>
                  )}

                  {sensor.latestReading && sensor.latestReading.readings && (
                    <>
                      <hr />
                      <h4>Latest Readings</h4>
                      <div className="readings">
                        {sensor.latestReading.readings?.temperature !== undefined && (
                          <p>
                            <strong>Temperature:</strong> {formatReadingValue(sensor.latestReading.readings.temperature, '°C')}
                          </p>
                        )}
                        {sensor.latestReading.readings?.humidity !== undefined && (
                          <p>
                            <strong>Humidity:</strong> {formatReadingValue(sensor.latestReading.readings.humidity, '%')}
                          </p>
                        )}
                        {sensor.latestReading.readings?.pollution !== undefined && (
                          <p>
                            <strong>Pollution:</strong> {formatReadingValue(sensor.latestReading.readings.pollution, ' AQI')}
                          </p>
                        )}
                        {sensor.latestReading.readings?.noise !== undefined && (
                          <p>
                            <strong>Noise:</strong> {formatReadingValue(sensor.latestReading.readings.noise, ' dB')}
                          </p>
                        )}
                        <p className="timestamp">
                          <small>Updated: {new Date(sensor.latestReading.timestamp).toLocaleString()}</small>
                        </p>
                      </div>
                    </>
                  )}
                </div>
              </div>
            </Popup>
          </Marker>
        );
        })}
      </MapContainer>

      <div className="map-info">
        <p>Showing {sensorsWithReadings.length} sensor{sensorsWithReadings.length !== 1 ? 's' : ''}</p>
      </div>
    </div>
  );
};

export default Map;
