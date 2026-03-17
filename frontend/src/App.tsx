import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import './App.css';
import {
  API_BASE_URL,
  DEFAULT_METRIC_FILTER,
  GOOGLE_MAPS_API_KEY,
  METRIC_COLORS,
  METRIC_FILTER_KEY,
  METRIC_KEYS,
  METRIC_UNITS,
  NAV_KEY,
  NERD_SCOPE_KEY,
  SENSOR_TYPE_LABEL,
  SESSION_KEY,
  THEME_KEY,
  TYPING_LINES,
  ZOOM_LEVEL_KEY,
} from './constants/app';
import { buildMetricPath, clamp, formatMetricName } from './lib/chart';
import { loadGoogleMaps } from './lib/maps';
import { useLiveSensorData } from './hooks/useLiveSensorData';
import { useSSEConnection } from './hooks/useSSEConnection';
import { useNerdsData } from './hooks/useNerdsData';
import { usePipelineData } from './hooks/usePipelineData';
import { useSessionConfig } from './hooks/useSessionConfig';
import { useUserSensors } from './hooks/useUserSensors';
import type {
  AnomalyEvent,
  DetectedEvent,
  MetricKey,
  NavItem,
  SensorRecord,
  Theme,
} from './types/app';

function App() {
  // ── Core UI state ─────────────────────────────────────────────────────────
  const [theme, setTheme] = useState<Theme>(() => {
    const stored = localStorage.getItem(THEME_KEY);
    if (stored === 'light' || stored === 'dark') return stored;
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  });
  const [activeNav, setActiveNav] = useState<NavItem>(() => {
    const stored = localStorage.getItem(NAV_KEY);
    return stored === 'nerds' ? 'nerds' : 'home';
  });
  const [sessionId] = useState<string>(() => {
    const existing = localStorage.getItem(SESSION_KEY);
    if (existing) return existing;
    const created = crypto.randomUUID();
    localStorage.setItem(SESSION_KEY, created);
    return created;
  });
  const [nerdScope, setNerdScope] = useState<'session' | 'global'>(() => {
    const stored = localStorage.getItem(NERD_SCOPE_KEY);
    return stored === 'session' ? 'session' : 'global';
  });
  const [metricFilter, setMetricFilter] = useState<Record<MetricKey, boolean>>(() => {
    try {
      const stored = localStorage.getItem(METRIC_FILTER_KEY);
      if (!stored) return DEFAULT_METRIC_FILTER;
      const parsed = JSON.parse(stored) as Partial<Record<MetricKey, unknown>>;
      return {
        temperature: typeof parsed.temperature === 'boolean' ? parsed.temperature : true,
        humidity: typeof parsed.humidity === 'boolean' ? parsed.humidity : true,
        pollution: typeof parsed.pollution === 'boolean' ? parsed.pollution : true,
        noise: typeof parsed.noise === 'boolean' ? parsed.noise : true,
      };
    } catch {
      return DEFAULT_METRIC_FILTER;
    }
  });
  const [zoomLevel, setZoomLevel] = useState<number>(() => {
    const raw = localStorage.getItem(ZOOM_LEVEL_KEY);
    const parsed = raw ? Number(raw) : 2;
    return Number.isFinite(parsed) ? clamp(parsed, 1, 8) : 2;
  });

  // ── Shared sensor state (owned here, passed to hooks) ─────────────────────
  const [hasUserSensor, setHasUserSensor] = useState(false);
  const [userSensors, setUserSensors] = useState<SensorRecord[]>([]);

  // ── Anomaly / Event state ─────────────────────────────────────────────────
  const [anomalies, setAnomalies] = useState<AnomalyEvent[]>([]);
  const [detectedEvents, setDetectedEvents] = useState<DetectedEvent[]>([]);

  // ── Typing animation state ────────────────────────────────────────────────
  const [typedLines, setTypedLines] = useState<string[]>(() => TYPING_LINES.map(() => ''));
  const [activeTypingLine, setActiveTypingLine] = useState(0);
  const [isTypingDone, setIsTypingDone] = useState(false);

  // ── Map state ────────────────────────────────────────────────────────────
  const [mapReady, setMapReady] = useState(false);
  const [nerdMapReady, setNerdMapReady] = useState(false);
  const [mapContainerNode, setMapContainerNode] = useState<HTMLDivElement | null>(null);
  const mapContainerRef = useCallback((node: HTMLDivElement | null) => { setMapContainerNode(node); }, []);
  const mapRef = useRef<any>(null);
  const mapMarkersRef = useRef<Record<string, any>>({});
  const mapInfoWindowsRef = useRef<Record<string, any>>({});
  const mapRadiusCircleRef = useRef<any>(null);
  const nerdMapContainerRef = useRef<HTMLDivElement | null>(null);
  const nerdMapRef = useRef<any>(null);
  const nerdMapMarkersRef = useRef<Record<string, any>>({});

  // ── Custom hooks ─────────────────────────────────────────────────────────
  const { sessionConfig, setSessionConfig, sessionConfigReady, isUpdatingRadius, locationPermission, handleRadiusChange } =
    useSessionConfig({ sessionId });

  const { latestBySensor, setLatestBySensor, setSummary, displaySummary, trendHistory } =
    useLiveSensorData({ sessionId, hasUserSensor, userSensors, activeNav });

  const {
    isCreatingSensor,
    statusMessage,
    handleAddSensor,
    handleAdjustClick,
    handleAdjustPointerDown,
    stopContinuousAdjust,
    userSensorCounts,
  } = useUserSensors({
    sessionId,
    sessionConfigReady,
    isUpdatingRadius,
    setLatestBySensor,
    hasUserSensor,
    setHasUserSensor,
    userSensors,
    setUserSensors,
    setSummary,
  });

  const { nerdStats, sessionNerdStats, nerdStatsLoading, sessionNerdStatsLoading, nerdStatsError, globalFootprint, globalFootprintLoading, refresh: refreshNerds } =
    useNerdsData({ sessionId, activeNav });

  const { pipelineStats, pipelineLoading, refreshPipeline } = usePipelineData({ activeNav });

  // ── SSE anomaly/event handler ────────────────────────────────────────────
  useSSEConnection(sessionId, hasUserSensor, useCallback((payload) => {
    if (payload.type === 'anomaly' && payload.data) {
      const event = payload.data as AnomalyEvent;
      setAnomalies((prev) => [event, ...prev].slice(0, 20));
    }
    if (payload.type === 'scenario_detected' && payload.data) {
      const event = payload.data as DetectedEvent;
      setDetectedEvents((prev) => {
        const updated = [event, ...prev.filter((e) => e.event_type !== event.event_type || e.zone !== event.zone)];
        return updated.slice(0, 10);
      });
    }
  }, []));

  // Auto-expire detected events
  useEffect(() => {
    if (detectedEvents.length === 0) return;
    const timer = window.setInterval(() => {
      const now = new Date().toISOString();
      setDetectedEvents((prev) => prev.filter((e) => e.expires_at > now));
    }, 5000);
    return () => window.clearInterval(timer);
  }, [detectedEvents.length]);

  // ── Derived values ────────────────────────────────────────────────────────
  // Map should init regardless of whether sensors exist yet — prevents race condition
  // where map init waits for sensor API response. Markers are added independently.
  const canInitMap = locationPermission === 'granted' && Boolean(GOOGLE_MAPS_API_KEY);
  const canShowMap = hasUserSensor && canInitMap;

  const rollingWindowLabel = useMemo(() => {
    if (trendHistory.length < 2) return 'Live';
    const first = trendHistory[0].timestamp;
    const last = trendHistory[trendHistory.length - 1].timestamp;
    const deltaSec = Math.max(1, Math.round((last - first) / 1000));
    if (deltaSec < 60) return `Last ${deltaSec}s`;
    const mins = Math.round(deltaSec / 60);
    if (mins < 60) return `Last ${mins}m`;
    return `Last ${Math.round(mins / 60)}h`;
  }, [trendHistory]);

  const enabledMetrics = useMemo(
    () => METRIC_KEYS.filter((metric) => metricFilter[metric]),
    [metricFilter]
  );

  // ── localStorage sync effects ─────────────────────────────────────────────
  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem(THEME_KEY, theme);
  }, [theme]);

  useEffect(() => { localStorage.setItem(NAV_KEY, activeNav); }, [activeNav]);
  useEffect(() => { localStorage.setItem(METRIC_FILTER_KEY, JSON.stringify(metricFilter)); }, [metricFilter]);
  useEffect(() => { localStorage.setItem(ZOOM_LEVEL_KEY, String(zoomLevel)); }, [zoomLevel]);
  useEffect(() => { localStorage.setItem(NERD_SCOPE_KEY, nerdScope); }, [nerdScope]);

  // ── Session heartbeat ───────────────────────────────────────────────────────
  // No client-side teardown on unload — reload vs close is indistinguishable.
  // Instead, the backend session heartbeat (renewed every stats poll) has a
  // short TTL. A server-side monitor deletes sensors when the heartbeat expires.
  // The frontend just keeps the heartbeat alive while the tab is open.
  useEffect(() => {
    // Send an immediate heartbeat on load
    fetch(`${API_BASE_URL}/session/config`, {
      headers: { 'X-Session-ID': sessionId },
    }).catch(() => {});

    // Heartbeat is also renewed by the session nerd-stats polling (every 1s)
  }, [sessionId]);

  // ── Typing animation ──────────────────────────────────────────────────────
  useEffect(() => {
    let cancelled = false;

    const typeNext = (lineIndex: number, charIndex: number) => {
      if (cancelled) return;
      if (lineIndex >= TYPING_LINES.length) {
        setIsTypingDone(true);
        return;
      }
      setActiveTypingLine(lineIndex);
      const currentLine = TYPING_LINES[lineIndex];
      if (charIndex <= currentLine.length) {
        setTypedLines((prev) => {
          const next = [...prev];
          next[lineIndex] = currentLine.slice(0, charIndex);
          return next;
        });
      }
      const isLineFinished = charIndex >= currentLine.length;
      window.setTimeout(() => {
        if (cancelled) return;
        isLineFinished ? typeNext(lineIndex + 1, 0) : typeNext(lineIndex, charIndex + 1);
      }, isLineFinished ? 450 : 42);
    };

    typeNext(0, 0);
    return () => { cancelled = true; };
  }, []);

  // ── Main map init (independent of sensor existence) ──────────────────────
  useEffect(() => {
    if (!canInitMap || !mapContainerNode) {
      setMapReady(false);
      mapRef.current = null;
      mapRadiusCircleRef.current = null;
      return;
    }
    let cancelled = false;

    const initMap = async () => {
      try {
        await loadGoogleMaps(GOOGLE_MAPS_API_KEY);
        if (cancelled || !mapContainerNode || !window.google?.maps) return;
        if (!mapRef.current) {
          mapRef.current = new window.google.maps.Map(mapContainerNode, {
            center: { lat: sessionConfig.origin_lat, lng: sessionConfig.origin_lon },
            zoom: 12,
            mapTypeControl: false,
            fullscreenControl: false,
            streetViewControl: false,
          });
        }
        setMapReady(true);
      } catch {
        // keep page functional without map
      }
    };

    initMap();
    return () => {
      cancelled = true;
      setMapReady(false);
      // Clear stale markers so they get re-created on the new map instance
      Object.values(mapMarkersRef.current).forEach((m: any) => m.setMap(null));
      mapMarkersRef.current = {};
      Object.values(mapInfoWindowsRef.current).forEach((w: any) => w.close());
      mapInfoWindowsRef.current = {};
      mapRef.current = null;
      mapRadiusCircleRef.current = null;
    };
  }, [canInitMap, mapContainerNode, sessionConfig.origin_lat, sessionConfig.origin_lon]);

  // ── Main map center + radius circle ──────────────────────────────────────
  useEffect(() => {
    if (!canInitMap || !mapReady || !mapRef.current || !window.google?.maps) return;

    const center = { lat: sessionConfig.origin_lat, lng: sessionConfig.origin_lon };
    mapRef.current.setCenter(center);

    if (!mapRadiusCircleRef.current) {
      mapRadiusCircleRef.current = new window.google.maps.Circle({
        map: mapRef.current,
        center,
        radius: sessionConfig.spawn_radius_km * 1000,
        strokeColor: '#0f5f3f',
        strokeOpacity: 0.45,
        strokeWeight: 1,
        fillColor: '#0f5f3f',
        fillOpacity: 0.06,
      });
    } else {
      mapRadiusCircleRef.current.setCenter(center);
      mapRadiusCircleRef.current.setRadius(sessionConfig.spawn_radius_km * 1000);
    }
  }, [canInitMap, mapReady, sessionConfig.origin_lat, sessionConfig.origin_lon, sessionConfig.spawn_radius_km]);

  // ── Main map markers ──────────────────────────────────────────────────────
  useEffect(() => {
    if (!canShowMap || !mapReady || !mapRef.current || !window.google?.maps) return;

    const mappedSensors = userSensors.filter(
      (sensor) =>
        typeof sensor.location?.latitude === 'number' &&
        Number.isFinite(sensor.location.latitude) &&
        typeof sensor.location?.longitude === 'number' &&
        Number.isFinite(sensor.location.longitude)
    );

    const nextIDs = new Set(mappedSensors.map((sensor) => sensor.id));

    Object.entries(mapMarkersRef.current).forEach(([sensorID, marker]) => {
      if (nextIDs.has(sensorID)) return;
      marker.setMap(null);
      delete mapMarkersRef.current[sensorID];
      const infoWindow = mapInfoWindowsRef.current[sensorID];
      if (infoWindow) {
        infoWindow.close();
        delete mapInfoWindowsRef.current[sensorID];
      }
    });

    mappedSensors.forEach((sensor) => {
      const sensorType = ((sensor.type || 'temperature').toLowerCase() as MetricKey);
      const safeType = METRIC_KEYS.includes(sensorType) ? sensorType : 'temperature';
      const latest = latestBySensor[sensor.id];
      const parsedValue = latest && typeof latest.value !== 'undefined' ? Number(latest.value) : Number.NaN;
      const hasValue = Number.isFinite(parsedValue);
      const markerPosition = {
        lat: Number(sensor.location?.latitude),
        lng: Number(sensor.location?.longitude),
      };

      let marker = mapMarkersRef.current[sensor.id];
      if (!marker) {
        marker = new window.google.maps.Marker({
          map: mapRef.current,
          position: markerPosition,
          title: hasValue ? `${parsedValue.toFixed(1)} ${METRIC_UNITS[safeType]}` : 'No live reading yet',
          icon: {
            path: window.google.maps.SymbolPath.CIRCLE,
            fillColor: METRIC_COLORS[safeType],
            fillOpacity: 0.95,
            strokeColor: '#ffffff',
            strokeOpacity: 1,
            strokeWeight: 2,
            scale: 7,
          },
        });
        mapMarkersRef.current[sensor.id] = marker;
      } else {
        marker.setPosition(markerPosition);
        marker.setTitle(hasValue ? `${parsedValue.toFixed(1)} ${METRIC_UNITS[safeType]}` : 'No live reading yet');
        marker.setIcon({
          path: window.google.maps.SymbolPath.CIRCLE,
          fillColor: METRIC_COLORS[safeType],
          fillOpacity: 0.95,
          strokeColor: '#ffffff',
          strokeOpacity: 1,
          strokeWeight: 2,
          scale: 7,
        });
      }

      let infoWindow = mapInfoWindowsRef.current[sensor.id];
      if (!infoWindow) {
        infoWindow = new window.google.maps.InfoWindow();
        mapInfoWindowsRef.current[sensor.id] = infoWindow;
        marker.addListener('mouseover', () => {
          infoWindow.open({ map: mapRef.current, anchor: marker });
        });
        marker.addListener('mouseout', () => infoWindow.close());
      }

      infoWindow.setContent(`
        <div style="
          min-width:150px;
          padding:10px 12px;
          font-family:Arial,sans-serif;
          border-radius:8px;
          border:1px solid ${METRIC_COLORS[safeType]}33;
          background:${METRIC_COLORS[safeType]}12;
        ">
          <div style="font-size:14px;font-weight:700;color:#0f172a;line-height:1.3;">
            ${formatMetricName(safeType)}: ${hasValue ? `${parsedValue.toFixed(1)} ${METRIC_UNITS[safeType]}` : 'No live reading yet'}
          </div>
        </div>
      `);
    });
  }, [canShowMap, mapReady, latestBySensor, userSensors]);

  // ── Main map resize on nav switch ─────────────────────────────────────────
  useEffect(() => {
    if (activeNav !== 'home' || !canInitMap || !mapReady || !mapRef.current || !window.google?.maps?.event) return;

    const center = { lat: sessionConfig.origin_lat, lng: sessionConfig.origin_lon };
    window.setTimeout(() => {
      if (!mapRef.current || !window.google?.maps?.event) return;
      window.google.maps.event.trigger(mapRef.current, 'resize');
      mapRef.current.setCenter(center);
    }, 0);
  }, [activeNav, canInitMap, mapReady, sessionConfig.origin_lat, sessionConfig.origin_lon]);

  // ── Nerd map init ─────────────────────────────────────────────────────────
  useEffect(() => {
    if (activeNav !== 'nerds' || nerdScope !== 'global' || !GOOGLE_MAPS_API_KEY || !nerdMapContainerRef.current) {
      setNerdMapReady(false);
      nerdMapRef.current = null;
      return;
    }
    let cancelled = false;

    const initNerdMap = async () => {
      try {
        await loadGoogleMaps(GOOGLE_MAPS_API_KEY);
        if (cancelled || !nerdMapContainerRef.current || !window.google?.maps) return;
        if (!nerdMapRef.current) {
          nerdMapRef.current = new window.google.maps.Map(nerdMapContainerRef.current, {
            center: { lat: 22.0, lng: 78.0 },
            zoom: 2,
            mapTypeControl: false,
            fullscreenControl: false,
            streetViewControl: false,
          });
        }
        setNerdMapReady(true);
        window.setTimeout(() => {
          if (!nerdMapRef.current || !window.google?.maps?.event) return;
          window.google.maps.event.trigger(nerdMapRef.current, 'resize');
          nerdMapRef.current.setCenter({ lat: 22.0, lng: 78.0 });
        }, 0);
      } catch {
        // fail-safe: stats page still works without map
      }
    };

    void initNerdMap();
    return () => {
      cancelled = true;
      nerdMapRef.current = null;
    };
  }, [activeNav, nerdScope]);

  // ── Nerd map markers ──────────────────────────────────────────────────────
  useEffect(() => {
    if (
      activeNav !== 'nerds' ||
      nerdScope !== 'global' ||
      !nerdMapReady ||
      !nerdMapRef.current ||
      !window.google?.maps ||
      !globalFootprint ||
      !Array.isArray(globalFootprint.points)
    ) {
      return;
    }

    const points = globalFootprint.points.filter(
      (point) =>
        Number.isFinite(point.lat) &&
        Number.isFinite(point.lon) &&
        typeof point.id === 'string' &&
        point.id.length > 0
    );
    const nextIDs = new Set(points.map((point) => point.id));

    Object.entries(nerdMapMarkersRef.current).forEach(([pointID, marker]) => {
      if (nextIDs.has(pointID)) return;
      marker.setMap(null);
      delete nerdMapMarkersRef.current[pointID];
    });

    points.forEach((point) => {
      const typeKey = point.type?.toLowerCase() as MetricKey;
      const color = METRIC_KEYS.includes(typeKey) ? METRIC_COLORS[typeKey] : '#16a34a';
      const position = { lat: point.lat, lng: point.lon };
      let marker = nerdMapMarkersRef.current[point.id];
      if (!marker) {
        marker = new window.google.maps.Marker({
          map: nerdMapRef.current,
          position,
          icon: {
            path: window.google.maps.SymbolPath.CIRCLE,
            fillColor: color,
            fillOpacity: 0.92,
            strokeColor: '#ffffff',
            strokeOpacity: 0.95,
            strokeWeight: 1.8,
            scale: 6,
          },
          title: formatMetricName(METRIC_KEYS.includes(typeKey) ? typeKey : 'temperature'),
        });
        nerdMapMarkersRef.current[point.id] = marker;
      } else {
        marker.setPosition(position);
        marker.setIcon({
          path: window.google.maps.SymbolPath.CIRCLE,
          fillColor: color,
          fillOpacity: 0.92,
          strokeColor: '#ffffff',
          strokeOpacity: 0.95,
          strokeWeight: 1.8,
          scale: 6,
        });
      }
    });

    if (points.length > 0) {
      const bounds = new window.google.maps.LatLngBounds();
      points.forEach((point) => bounds.extend({ lat: point.lat, lng: point.lon }));
      nerdMapRef.current.fitBounds(bounds);
      if (points.length === 1) nerdMapRef.current.setZoom(8);
    } else {
      nerdMapRef.current.setCenter({ lat: 22.0, lng: 78.0 });
      nerdMapRef.current.setZoom(2);
    }
  }, [activeNav, nerdScope, nerdMapReady, globalFootprint]);

  // ── Handlers ──────────────────────────────────────────────────────────────
  const toggleTheme = () => setTheme((prev) => (prev === 'light' ? 'dark' : 'light'));

  const metricLabel = (label: string, hint?: string) => {
    if (!hint) return label;
    return (
      <span
        className="metric-help"
        data-tooltip={hint}
        tabIndex={0}
        role="note"
        aria-label={`${label}: ${hint}`}
      >
        {label}
      </span>
    );
  };

  // ── Render ────────────────────────────────────────────────────────────────
  return (
    <div className="app-shell">
      <header className="app-header">
        <div className="brand">Smart City Monitor</div>
        <nav className="nav-links" aria-label="primary">
          <button
            className={`nav-link ${activeNav === 'home' ? 'active' : ''}`}
            type="button"
            onClick={() => setActiveNav('home')}
          >
            Home
          </button>
          <button
            className={`nav-link ${activeNav === 'nerds' ? 'active' : ''}`}
            type="button"
            onClick={() => setActiveNav('nerds')}
          >
            Stats for Nerds
          </button>
        </nav>
        <div className="header-right">
          <button className="theme-toggle" type="button" onClick={toggleTheme}>
            {theme === 'light' ? 'Dark Mode' : 'Light Mode'}
          </button>
        </div>
      </header>

      <main className={`app-main ${activeNav !== 'home' ? 'app-main-top' : ''}`}>
        {detectedEvents.length > 0 && (
          <div className="scenario-banners">
            {detectedEvents.map((evt) => (
              <div className={`scenario-banner scenario-banner-${evt.event_type}`} key={`${evt.event_type}-${evt.zone}`}>
                <strong>{evt.event_type.replace(/_/g, ' ').toUpperCase()}</strong>
                <span className="scenario-zone">{evt.zone}</span>
                <span className="scenario-desc">{evt.description}</span>
              </div>
            ))}
          </div>
        )}

        {activeNav === 'nerds' ? (
          <section className="nerds-page" aria-live="polite">
            <div className="nerds-header">
              <h2>Stats for Nerds</h2>
              <p>Scoped observability for your session and the full platform.</p>
            </div>
            <div className="nerds-scope-toggle" role="tablist" aria-label="Stats scope">
              <button
                className={`scope-btn ${nerdScope === 'session' ? 'active' : ''}`}
                type="button"
                onClick={() => setNerdScope('session')}
              >
                My Session
              </button>
              <button
                className={`scope-btn ${nerdScope === 'global' ? 'active' : ''}`}
                type="button"
                onClick={() => setNerdScope('global')}
              >
                Global Platform
              </button>
            </div>
            {nerdStatsError && <p className="status-text">{nerdStatsError}</p>}
            {(nerdStatsLoading || sessionNerdStatsLoading) && !nerdStats && !sessionNerdStats && (
              <p className="status-text">Loading nerd stats...</p>
            )}

            {nerdScope === 'session' && sessionNerdStats && (
              <>
              <div className="nerds-grid">
                <article className="summary-card">
                  <p className="summary-label">My Session</p>
                  <p className="nerd-row">Session Sensors: <strong>{sessionNerdStats.session.sensors}</strong></p>
                  <p className="nerd-row">Session Readings: <strong>{sessionNerdStats.session.readings}</strong></p>
                  <p className="nerd-row">{metricLabel('Readings / Min', 'Total sensor readings received in the last 60 seconds across all sensors in this session.')}: <strong>{sessionNerdStats.session.readings_last_min}</strong></p>
                  <p className="nerd-row">{metricLabel('Avg Value (all types)', 'Running average across all sensor readings in this session. Mixes sensor types so treat as a rough signal, not a precise metric.')}: <strong>{sessionNerdStats.session.avg_sensor_value.toFixed(2)}</strong></p>
                  <p className="nerd-row">Last Reading: <strong>{sessionNerdStats.session.last_reading_at ? new Date(sessionNerdStats.session.last_reading_at).toLocaleTimeString() : 'N/A'}</strong></p>
                </article>

                <article className="summary-card">
                  <p className="summary-label">Session Realtime</p>
                  <p className="nerd-row">{metricLabel('Session Throughput', 'Actual sensor readings per second for this session (readings in last minute / 60).')}: <strong>{sessionNerdStats.realtime.estimated_throughput_msg_sec.toFixed(2)} msg/s</strong></p>
                  <p className="nerd-row">{metricLabel('SSE Connected', 'Whether this browser session has an active Server-Sent Events stream connection.')}: <strong>{sessionNerdStats.realtime.sse_connected ? 'Yes' : 'No'}</strong></p>
                  <p className="nerd-row">Last Updated: <strong>{new Date(sessionNerdStats.timestamp).toLocaleTimeString()}</strong></p>
                </article>
              </div>
              {(() => {
                const s = pipelineStats;
                const dur = s ? Math.max(1.5, 8 - Math.min(s.throughput_msg_sec, 12) * 0.5) : 8;
                const durKey = Math.round(dur * 2);
                const edges = [
                  { id: 'e1', d: 'M 145,140 C 190,140 210,140 255,140' },
                  { id: 'e2a', d: 'M 345,140 C 390,140 400,90 445,90' },
                  { id: 'e2b', d: 'M 345,140 C 390,140 400,190 445,190' },
                  { id: 'e3', d: 'M 545,90 C 590,90 610,90 655,90' },
                  { id: 'e4', d: 'M 545,190 C 590,190 610,190 655,190' },
                ];
                const nodes = [
                  { x: 80, y: 140, w: 120, label: 'Sensors', stat: s ? `${s.sensors} active` : '---' },
                  { x: 300, y: 140, w: 90, label: 'Kafka', stat: s ? `${s.kafka_messages} msgs` : '---' },
                  { x: 495, y: 90, w: 90, label: 'Ingestion', stat: s ? `${s.throughput_msg_sec.toFixed(1)} msg/s` : '---' },
                  { x: 720, y: 90, w: 110, label: 'PostgreSQL', stat: s ? `${Math.max(0, s.database_records)} rows` : '---' },
                  { x: 495, y: 190, w: 110, label: 'API Gateway', stat: s ? `${s.kafka_topics ?? 3} topics` : '---' },
                  { x: 720, y: 190, w: 120, label: 'SSE / Clients', stat: s ? `${s.sse_clients} connected` : '---' },
                ];
                return (
                  <article className="summary-card pipeline-card">
                    <div className="pipeline-card-header">
                      <p className="summary-label">Data Pipeline</p>
                      {s && <span className="pipeline-throughput">{s.throughput_msg_sec.toFixed(1)} msg/s</span>}
                      {pipelineLoading && !s && <span className="pipeline-throughput">Loading...</span>}
                    </div>
                    <div className="pipeline-svg-wrapper">
                      <svg className="pipeline-svg" viewBox="0 0 900 280" preserveAspectRatio="xMidYMid meet">
                        <defs>
                          <filter id="pipeline-glow">
                            <feGaussianBlur stdDeviation="2.5" result="blur" />
                            <feMerge>
                              <feMergeNode in="blur" />
                              <feMergeNode in="SourceGraphic" />
                            </feMerge>
                          </filter>
                        </defs>
                        {edges.map((e) => (
                          <path key={e.id} d={e.d} className="pipeline-edge" />
                        ))}
                        {edges.map((e) =>
                          [0, 0.33, 0.66].map((offset) => (
                            <circle
                              key={`${e.id}-p-${offset}-${durKey}`}
                              r="4"
                              className="pipeline-particle"
                            >
                              <animateMotion
                                dur={`${dur}s`}
                                repeatCount="indefinite"
                                path={e.d}
                                begin={`${-offset * dur}s`}
                                calcMode="linear"
                              />
                            </circle>
                          ))
                        )}
                        {nodes.map((n) => (
                          <g key={n.label}>
                            <rect
                              x={n.x - n.w / 2}
                              y={n.y - 28}
                              width={n.w}
                              height={56}
                              rx={10}
                              className="pipeline-node-shape"
                            />
                            <text x={n.x} y={n.y - 6} className="pipeline-node-label">{n.label}</text>
                            <text x={n.x} y={n.y + 14} className="pipeline-node-stat">{n.stat}</text>
                          </g>
                        ))}
                      </svg>
                    </div>
                  </article>
                );
              })()}
              </>
            )}
            {nerdScope === 'session' && !sessionNerdStats && !sessionNerdStatsLoading && (
              <article className="summary-card">
                <p className="summary-label">My Session</p>
                <p className="nerd-row">No session-scoped stats yet for this browser session.</p>
                <p className="nerd-row">Session ID: <strong>{sessionId}</strong></p>
              </article>
            )}

            {nerdScope === 'global' && nerdStats && (
              <>
                <div className="nerds-refresh-row">
                  <button
                    className="scope-btn refresh-btn"
                    type="button"
                    onClick={() => { refreshNerds(); refreshPipeline(); }}
                    title="Refresh all stats now"
                  >
                    Refresh
                  </button>
                  <span className="polling-dot-note"><span className="polling-dot" /> Auto-updates every 5s</span>
                </div>
                <div className="nerds-grid">
                  <article className="summary-card">
                    <p className="summary-label">Platform Runtime</p>
                    <p className="nerd-row">Uptime: <strong>{Math.round(nerdStats.system.uptime_sec / 60)} min</strong></p>
                    <p className="nerd-row">{metricLabel('Go Routines', 'Number of live goroutines currently scheduled by the Go runtime.')}: <strong>{nerdStats.system.goroutines}</strong></p>
                    <p className="nerd-row">{metricLabel('Heap Alloc', 'Heap memory currently allocated and in active use by the process.')}: <strong>{nerdStats.system.heap_alloc_mb.toFixed(2)} MB</strong></p>
                    <p className="nerd-row">{metricLabel('Heap Sys', 'Heap memory obtained from the OS by the Go runtime, including unused reserved heap.')}: <strong>{nerdStats.system.heap_sys_mb.toFixed(2)} MB</strong></p>
                    <p className="nerd-row">{metricLabel('GC Cycles', 'Total completed Go garbage collection cycles since process start.')}: <strong>{nerdStats.system.gc_cycles_total}</strong></p>
                  </article>

                  <article className="summary-card">
                    <p className="summary-label">Sessions</p>
                    <p className="nerd-row">Total Visitors: <strong>{nerdStats.sessions.total_visitors}</strong></p>
                    <p className="nerd-row">Active Sessions: <strong>{nerdStats.sessions.active_now}</strong></p>
                    <p className="nerd-row">{metricLabel('Sessions With Sensors', 'Distinct sessions that currently own at least one sensor in the database.')}: <strong>{nerdStats.platform?.sessions_with_sensors ?? 0}</strong></p>
                    <p className="nerd-row">{metricLabel('SSE Clients', 'Open Server-Sent Events stream connections currently receiving live updates.')}: <strong>{nerdStats.sse.active_clients}</strong></p>
                  </article>

                  <article className="summary-card">
                    <p className="summary-label">Database</p>
                    <p className="nerd-row">{metricLabel('Active Sensors', 'Sensors currently registered and generating data.')}: <strong>{nerdStats.database.sensors}</strong></p>
                    <p className="nerd-row">{metricLabel('Total Sensors Created', 'Distinct sensors that have ever produced at least one reading.')}: <strong>{nerdStats.database.total_sensors_created}</strong></p>
                    <p className="nerd-row">Readings: <strong>{nerdStats.database.readings}</strong></p>
                    <p className="nerd-row">{metricLabel('Avg Readings/Sensor', 'Global retention density: total readings divided by active sensors.')}: <strong>{(nerdStats.platform?.avg_readings_per_sensor ?? 0).toFixed(2)}</strong></p>
                  </article>

                  <article className="summary-card">
                    <p className="summary-label">Redis</p>
                    <p className="nerd-row">Keys: <strong>{nerdStats.redis.key_count}</strong></p>
                    <p className="nerd-row">Clients: <strong>{nerdStats.redis.connected_clients}</strong></p>
                    <p className="nerd-row">Ping Latency: <strong>{nerdStats.redis.ping_latency_ms.toFixed(2)} ms</strong></p>
                    <p className="nerd-row">{metricLabel('Commands Processed', 'Total Redis commands executed since Redis started (reads, writes, pub/sub, etc.).')}: <strong>{nerdStats.redis.total_commands_processed}</strong></p>
                  </article>

                  <article className="summary-card">
                    <p className="summary-label">Kafka</p>
                    <p className="nerd-row">Brokers: <strong>{nerdStats.kafka.broker_count}</strong></p>
                    <p className="nerd-row">Topics: <strong>{nerdStats.kafka.topic_count}</strong></p>
                    <p className="nerd-row">{metricLabel('Partitions', 'Total partitions across all topics. More partitions increase parallelism capacity.')}: <strong>{nerdStats.kafka.total_partitions}</strong></p>
                    <p className="nerd-row">{metricLabel('Messages (est)', 'Estimated retained message count from (newest offset - oldest offset) across partitions.')}: <strong>{nerdStats.kafka.total_messages}</strong></p>
                  </article>

                  <article className="summary-card">
                    <p className="summary-label">Performance</p>
                    <p className="nerd-row">{metricLabel('Est Throughput', 'Approximate messages/second derived from Kafka message delta over time.')}: <strong>{nerdStats.performance.estimated_throughput_msg_sec.toFixed(2)} msg/s</strong></p>
                    <p className="nerd-row">Last Updated: <strong>{new Date(nerdStats.timestamp).toLocaleTimeString()}</strong></p>
                  </article>
                </div>
                <article className="summary-card global-map-card">
                  <div className="map-panel-header">
                    <p className="summary-label">Global Sensor Footprint</p>
                    <p className="summary-meta">
                      {globalFootprintLoading ? 'Refreshing map...' : `${globalFootprint?.total ?? 0} sensors`}
                    </p>
                  </div>
                  <div className="global-map-legend">
                    {METRIC_KEYS.map((metric) => (
                      <span className="latest-item" key={`legend-${metric}`}>
                        <span className="metric-dot" style={{ backgroundColor: METRIC_COLORS[metric] }} />
                        {formatMetricName(metric)}: <strong>{globalFootprint?.counts?.[metric] ?? 0}</strong>
                      </span>
                    ))}
                  </div>
                  {!GOOGLE_MAPS_API_KEY ? (
                    <p className="status-text">Map is disabled. Set <code>VITE_GOOGLE_MAPS_API_KEY</code> to enable it.</p>
                  ) : (
                    <div className="home-map nerd-global-map" ref={nerdMapContainerRef} />
                  )}
                </article>
              </>
            )}
          </section>
        ) : !hasUserSensor ? (
          <section className="hero-content">
            {TYPING_LINES.map((_, index) => (
              <h1 className="terminal-line" key={index}>
                {typedLines[index]}
                {!isTypingDone && activeTypingLine === index && (
                  <span className="thinking-cursor" aria-hidden="true" />
                )}
              </h1>
            ))}
            {isTypingDone && (
              <button
                className="add-sensor-btn"
                type="button"
                onClick={() => handleAddSensor('temperature')}
                disabled={isCreatingSensor || !sessionConfigReady}
              >
                {isCreatingSensor ? 'Adding sensor...' : 'Add Sensor'}
              </button>
            )}
            {statusMessage && <p className="status-text">{statusMessage}</p>}
          </section>
        ) : (
          <section className="home-shell" aria-live="polite">
            <div className="home-main">
              <article className="summary-card summary-avg">
                <p className="summary-label">Average Sensor Data</p>
                <div className="avg-grid">
                  <div className="avg-item">
                    <span>Temperature</span>
                    <strong>{displaySummary.averages.temperature ?? 'N/A'}{displaySummary.averages.temperature !== undefined ? '°C' : ''}</strong>
                  </div>
                  <div className="avg-item">
                    <span>Humidity</span>
                    <strong>{displaySummary.averages.humidity ?? 'N/A'}{displaySummary.averages.humidity !== undefined ? '%' : ''}</strong>
                  </div>
                  <div className="avg-item">
                    <span>Pollution</span>
                    <strong>{displaySummary.averages.pollution ?? 'N/A'}{displaySummary.averages.pollution !== undefined ? ' AQI' : ''}</strong>
                  </div>
                  <div className="avg-item">
                    <span>Noise</span>
                    <strong>{displaySummary.averages.noise ?? 'N/A'}{displaySummary.averages.noise !== undefined ? ' dB' : ''}</strong>
                  </div>
                </div>
              </article>

              <article className="summary-card summary-total">
                <p className="summary-label">Total Sensors</p>
                <h2>{displaySummary.totalSensors}</h2>
                <p className="summary-meta">All existing sensors are active</p>
              </article>

              <article className="summary-card home-map-panel">
                <div className="map-panel-header">
                  <p className="summary-label">Sensor Map</p>
                  <p className="summary-meta">Radius: {Math.round(sessionConfig.spawn_radius_km)} km</p>
                </div>
                {!GOOGLE_MAPS_API_KEY ? (
                  <p className="status-text">Map is disabled. Set <code>VITE_GOOGLE_MAPS_API_KEY</code> to enable it.</p>
                ) : locationPermission !== 'granted' ? (
                  <p className="status-text">Map is hidden. Allow location access to enable map view.</p>
                ) : (
                  <div className="home-map" ref={mapContainerRef} />
                )}
              </article>

              <article className="summary-card home-chart-panel">
                <div className="chart-filter-row">
                  {METRIC_KEYS.map((metric) => (
                    <label className="metric-checkbox" key={metric}>
                      <input
                        type="checkbox"
                        checked={metricFilter[metric]}
                        onChange={(e) => {
                          const checked = e.target.checked;
                          setMetricFilter((prev) => ({ ...prev, [metric]: checked }));
                        }}
                      />
                      <span className="metric-dot" style={{ backgroundColor: METRIC_COLORS[metric] }} />
                      {formatMetricName(metric)}
                    </label>
                  ))}
                  <div className="zoom-controls" role="group" aria-label="Chart zoom controls">
                    <span>Zoom</span>
                    <button
                      type="button"
                      onClick={() => setZoomLevel((prev) => clamp(prev / 2, 1, 8))}
                      disabled={zoomLevel <= 1}
                    >
                      -
                    </button>
                    <span className="zoom-value">{zoomLevel.toFixed(1)}x</span>
                    <button
                      type="button"
                      onClick={() => setZoomLevel((prev) => clamp(prev * 2, 1, 8))}
                      disabled={zoomLevel >= 8}
                    >
                      +
                    </button>
                  </div>
                </div>
                <div className="chart-meta-row">
                  <p className="window-label">{rollingWindowLabel}</p>
                  <div className="latest-legend">
                    {enabledMetrics.map((metric) => {
                      const value = displaySummary.averages[metric];
                      return (
                        <span className="latest-item" key={metric}>
                          <span className="metric-dot" style={{ backgroundColor: METRIC_COLORS[metric] }} />
                          {formatMetricName(metric)}:{' '}
                          <strong>
                            {typeof value === 'number' ? `${value.toFixed(1)} ${METRIC_UNITS[metric]}` : 'N/A'}
                          </strong>
                        </span>
                      );
                    })}
                  </div>
                </div>
                <div className="live-metric-chart" aria-label="Live sensor metrics chart">
                  <svg viewBox="0 0 1000 260" preserveAspectRatio="none">
                    <rect x="0" y="0" width="1000" height="260" fill="transparent" />
                    <text x="500" y="252" textAnchor="middle" className="axis-label">Time</text>
                    <text x="11" y="130" textAnchor="middle" transform="rotate(-90 11 130)" className="axis-label">Value</text>
                    {enabledMetrics.map((metric) => {
                      const path = buildMetricPath(trendHistory, metric, 1000, 260, 16, zoomLevel);
                      if (!path) return null;
                      return (
                        <path
                          key={metric}
                          d={path}
                          fill="none"
                          stroke={METRIC_COLORS[metric]}
                          strokeWidth="3"
                          strokeLinecap="round"
                          strokeLinejoin="round"
                        />
                      );
                    })}
                  </svg>
                </div>
              </article>
              {statusMessage && <p className="status-text">{statusMessage}</p>}
            </div>

            <aside className="home-sidebar">
              <article className="summary-card home-controls-panel">
                <div className="manage-toolbar">
                  <h2>Sensor Controls</h2>
                  <div className="radius-control">
                    <label htmlFor="spawn-radius">Spawn Radius: {Math.round(sessionConfig.spawn_radius_km)} km</label>
                    <input
                      id="spawn-radius"
                      type="range"
                      min={1}
                      max={100}
                      step={1}
                      value={Math.round(sessionConfig.spawn_radius_km)}
                      onChange={(e) => {
                        const value = Number(e.target.value);
                        setSessionConfig((prev) => ({ ...prev, spawn_radius_km: value }));
                      }}
                      onMouseUp={(e) => handleRadiusChange(Number((e.target as HTMLInputElement).value))}
                      onTouchEnd={(e) => handleRadiusChange(Number((e.target as HTMLInputElement).value))}
                      onBlur={(e) => handleRadiusChange(Number((e.target as HTMLInputElement).value))}
                      disabled={isUpdatingRadius}
                    />
                  </div>
                </div>
                <div className="sensor-type-grid">
                  {METRIC_KEYS.map((sensorType) => {
                    const count = userSensorCounts[sensorType];
                    return (
                      <article className={`sensor-type-card sensor-type-card-${sensorType}`} key={sensorType}>
                        <p className={`sensor-type-title sensor-type-title-${sensorType}`}>{SENSOR_TYPE_LABEL[sensorType]}</p>
                        <div className="sensor-type-controls">
                          <button
                            className="sensor-arrow-btn"
                            type="button"
                            aria-label={`Add ${sensorType} sensor`}
                            onClick={() => void handleAdjustClick(sensorType, 'up')}
                            onPointerDown={(event) => handleAdjustPointerDown(event, sensorType, 'up')}
                            onPointerUp={stopContinuousAdjust}
                            onPointerLeave={stopContinuousAdjust}
                            onPointerCancel={stopContinuousAdjust}
                            disabled={isUpdatingRadius || !sessionConfigReady || userSensors.length >= 50}
                          >
                            ▲
                          </button>
                          <p className="sensor-type-count" aria-live="polite">{count}</p>
                          <button
                            className="sensor-arrow-btn"
                            type="button"
                            aria-label={`Remove ${sensorType} sensor`}
                            onClick={() => void handleAdjustClick(sensorType, 'down')}
                            onPointerDown={(event) => handleAdjustPointerDown(event, sensorType, 'down')}
                            onPointerUp={stopContinuousAdjust}
                            onPointerLeave={stopContinuousAdjust}
                            onPointerCancel={stopContinuousAdjust}
                            disabled={count === 0 || isUpdatingRadius}
                          >
                            ▼
                          </button>
                        </div>
                      </article>
                    );
                  })}
                </div>
              </article>

              {anomalies.length > 0 && (
                <article className="summary-card anomaly-feed-panel">
                  <p className="summary-label">Anomaly Feed</p>
                  <ul className="anomaly-feed-list">
                    {anomalies.map((a, i) => (
                      <li className="anomaly-feed-item" key={`${a.sensor_id}-${a.timestamp}-${i}`}>
                        <span className="anomaly-type">{a.sensor_type}</span>
                        <span className="anomaly-value">{Number(a.value).toFixed(1)} (z={Number(a.z_score).toFixed(1)})</span>
                        <span className="anomaly-time">{new Date(a.timestamp).toLocaleTimeString()}</span>
                      </li>
                    ))}
                  </ul>
                </article>
              )}
            </aside>
          </section>
        )}
      </main>
    </div>
  );
}

export default App;
