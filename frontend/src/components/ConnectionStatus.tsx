import { useMemo, useState, useEffect } from 'react';
import { useRealtimeData } from '../context/RealtimeContext';
import type { ConnectionStatus } from '../types/realtime';
import './ConnectionStatus.css';

const STATUS_COPY: Record<
  ConnectionStatus,
  { label: string; description: string; className: string }
> = {
  connected: { label: 'Live', description: 'Streaming sensor data', className: 'connected' },
  connecting: { label: 'Connecting', description: 'Negotiating WebSocket', className: 'connecting' },
  reconnecting: { label: 'Reconnecting', description: 'Attempting to rejoin', className: 'reconnecting' },
  disconnected: { label: 'Offline', description: 'WebSocket closed', className: 'disconnected' },
  error: { label: 'Network issue', description: 'Tap to retry', className: 'error' },
};

const ConnectionStatusIndicator = () => {
  const { connectionStatus, cityTrends } = useRealtimeData();
  const [isStale, setIsStale] = useState(false);

  const status = useMemo(() => STATUS_COPY[connectionStatus], [connectionStatus]);

  // Check for stale data (no updates in last 60 seconds)
  useEffect(() => {
    const checkStale = () => {
      if (cityTrends.length === 0) {
        setIsStale(false);
        return;
      }

      const lastTrend = cityTrends[cityTrends.length - 1];
      const lastTimestamp = new Date(lastTrend.timestamp).getTime();
      const now = Date.now();
      const ageInSeconds = (now - lastTimestamp) / 1000;

      setIsStale(ageInSeconds > 60);
    };

    checkStale();
    const interval = setInterval(checkStale, 10000); // Check every 10 seconds

    return () => clearInterval(interval);
  }, [cityTrends]);

  const displayStatus = useMemo(() => {
    if (isStale && connectionStatus === 'connected') {
      return {
        label: 'Stale',
        description: 'Data may be outdated',
        className: 'stale',
      };
    }
    return status;
  }, [isStale, connectionStatus, status]);

  return (
    <div className={`connection-status ${displayStatus.className}`} title={displayStatus.description}>
      <span className="connection-status__dot" aria-hidden="true" />
      <div className="connection-status__copy">
        <span className="connection-status__label">{displayStatus.label}</span>
        <span className="connection-status__description">{displayStatus.description}</span>
      </div>
    </div>
  );
};

export default ConnectionStatusIndicator;
