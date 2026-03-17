# Smart City Monitor — SSE (Server-Sent Events) Architecture

## Role

SSE is the only real-time transport from the backend to frontend clients. All live sensor updates, anomaly detections, and pattern events are delivered through SSE.

## Endpoint

```
GET /stream?session_id=<uuid>
```

The `session_id` query parameter associates the SSE connection with a browser session for session-scoped filtering and metrics.

## Event Types

| Event Type | Source | Payload Description |
|------------|--------|---------------------|
| `sensor_update` | Kafka `sensor-readings` topic | Live sensor reading (id, type, value, unit, location, timestamp) |
| `anomaly` | Kafka `sensor-anomalies` topic | Anomaly detection (sensor_id, value, z_score, expected_mean, expected_stddev, zone) |
| `scenario_detected` | Kafka `sensor-events` topic | Zone-level pattern detection (event_type, zone, severity, description, sensor_ids) |

## Architecture

```
Kafka (3 topics) → API Gateway Kafka Consumer → SSE Broadcaster → Connected Clients
```

The SSE broadcaster is implemented in `backend/internal/sse/`. It maintains a set of connected clients and fans out messages from the API Gateway's Kafka consumer to all active connections.

**Important:** The API Gateway consumes Kafka directly with its own consumer group (`gateway-group`). It does NOT use Redis Pub/Sub for real-time data delivery. Legacy `sensor:updates` and `sensor:anomalies` Redis channels are no longer used.

## Client Connection

```javascript
const sessionId = localStorage.getItem('SESSION_KEY') || crypto.randomUUID();
const eventSource = new EventSource(`/stream?session_id=${sessionId}`);

eventSource.onmessage = (event) => {
  const payload = JSON.parse(event.data);
  switch (payload.type) {
    case 'sensor_update':
      // Update map markers and stat cards
      break;
    case 'anomaly':
      // Append to anomaly feed panel
      break;
    case 'scenario_detected':
      // Show scenario banner
      break;
  }
};
```

## Connection Statistics

```
GET /stream/stats
```

Returns current SSE connection count and per-session information.

## Session Event History

```
GET /api/v1/stream/history
```

Returns recent SSE events for the session (from Redis `session:events:{session_id}` stream, max 180 entries, 90s TTL).

## Behavior Notes

- The frontend uses SSE as the primary data source. REST polling is a fallback only where SSE data is insufficient.
- SSE connections are long-lived. The browser's EventSource API handles automatic reconnection.
- The API Gateway broadcasts all Kafka messages to all connected clients. Client-side filtering is done in the frontend.
- Connection count is tracked in the nerd stats aggregator and exposed via `/api/v1/metrics/nerds`.
