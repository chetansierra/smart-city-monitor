# Smart City Monitor API Documentation

**Version:** 1.0
**Base URL:** `http://localhost:8080`
**Last Updated:** November 19, 2025

---

## Table of Contents

1. [Overview](#overview)
2. [Authentication](#authentication)
3. [Response Format](#response-format)
4. [Error Codes](#error-codes)
5. [Endpoints](#endpoints)
   - [Health Check](#health-check)
   - [Sensors](#sensors)
   - [Readings](#readings)
   - [Analytics](#analytics)
   - [Alerts](#alerts)
   - [SSE](#sse)

---

## Overview

The Smart City Monitor API provides real-time access to environmental sensor data across a city. It includes:

- **21 REST API endpoints**
- **1 SSE endpoint** for real-time updates
- **4 sensor types**: Temperature, Pollution, Humidity, Noise
- **Real-time alerts** when environmental thresholds are exceeded

---

## Authentication

**Current Version:** No authentication required (development mode)

**Future Versions:** Will implement JWT-based authentication

---

## Response Format

### Success Response

```json
{
  "success": true,
  "data": { ... },
  "meta": {
    "page": 1,
    "limit": 10,
    "total": 100
  }
}
```

### Error Response

```json
{
  "success": false,
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error message"
  }
}
```

---

## Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `ERROR_400` | 400 | Bad Request - Invalid input parameters |
| `ERROR_404` | 404 | Not Found - Resource doesn't exist |
| `ERROR_500` | 500 | Internal Server Error |
| `SENSOR_NOT_FOUND` | 404 | Sensor with specified ID not found |
| `INVALID_SENSOR_ID` | 400 | Invalid UUID format for sensor ID |
| `INVALID_TIME_RANGE` | 400 | Invalid from/to timestamp parameters |
| `ALERT_NOT_FOUND` | 404 | Alert not found or already acknowledged |
| `DATABASE_ERROR` | 500 | Database query failed |

---

## Endpoints

### Health Check

#### `GET /health`

Check the health status of the API and its dependencies.

**Response:**

```json
{
  "success": true,
  "data": {
    "status": "healthy",
    "timestamp": "2025-11-19T21:30:00Z",
    "services": {
      "postgres": "healthy",
      "redis": "healthy",
      "kafka": "healthy"
    }
  }
}
```

**cURL Example:**

```bash
curl http://localhost:8080/health
```

---

### Sensors

#### `GET /api/v1/sensors`

List all sensors with optional filtering and pagination.

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `type` | string | - | Filter by sensor type: `temperature`, `pollution`, `humidity`, `noise` |
| `page` | integer | 1 | Page number |
| `limit` | integer | 10 | Results per page (max: 1000) |

**Response:**

```json
{
  "success": true,
  "data": [
    {
      "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
      "name": "Downtown Temp 1",
      "type": "temperature",
      "location": {
        "latitude": 40.7128,
        "longitude": -74.0060
      },
      "status": "active",
      "last_reading": "2025-11-19T21:30:00Z"
    }
  ],
  "meta": {
    "page": 1,
    "limit": 10,
    "total": 50
  }
}
```

**cURL Examples:**

```bash
# Get all sensors
curl http://localhost:8080/api/v1/sensors

# Get temperature sensors only
curl "http://localhost:8080/api/v1/sensors?type=temperature"

# Get page 2 with 20 results
curl "http://localhost:8080/api/v1/sensors?page=2&limit=20"
```

---

#### `GET /api/v1/sensors/:id`

Get details of a specific sensor.

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | UUID | Sensor ID |

**Response:**

```json
{
  "success": true,
  "data": {
    "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "name": "Downtown Temp 1",
    "type": "temperature",
    "location": {
      "latitude": 40.7128,
      "longitude": -74.0060
    },
    "status": "active",
    "last_reading": "2025-11-19T21:30:00Z",
    "unit": "celsius"
  }
}
```

**cURL Example:**

```bash
curl http://localhost:8080/api/v1/sensors/a1b2c3d4-e5f6-7890-abcd-ef1234567890
```

---

#### `GET /api/v1/sensors/:id/latest`

Get the latest reading from a sensor (from Redis cache).

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | UUID | Sensor ID |

**Response:**

```json
{
  "success": true,
  "data": {
    "sensor_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "sensor_type": "temperature",
    "value": 22.5,
    "unit": "celsius",
    "timestamp": "2025-11-19T21:30:00Z"
  }
}
```

**cURL Example:**

```bash
curl http://localhost:8080/api/v1/sensors/a1b2c3d4-e5f6-7890-abcd-ef1234567890/latest
```

---

#### `GET /api/v1/sensors/:id/readings`

Get historical readings for a specific sensor.

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | UUID | Sensor ID |

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `from` | timestamp | - | Start time (RFC3339 format) |
| `to` | timestamp | - | End time (RFC3339 format) |
| `limit` | integer | 100 | Max results (max: 10000) |

**Response:**

```json
{
  "success": true,
  "data": [
    {
      "sensor_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
      "value": 22.5,
      "timestamp": "2025-11-19T21:30:00Z"
    }
  ],
  "meta": {
    "total": 150
  }
}
```

**cURL Example:**

```bash
curl "http://localhost:8080/api/v1/sensors/a1b2c3d4-e5f6-7890-abcd-ef1234567890/readings?from=2025-11-19T00:00:00Z&to=2025-11-19T23:59:59Z&limit=100"
```

---

#### `GET /api/v1/sensors/:id/alerts`

Get all alerts for a specific sensor.

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | UUID | Sensor ID |

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | integer | 50 | Max results |

**Response:**

```json
{
  "success": true,
  "data": [
    {
      "id": 123,
      "sensor_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
      "sensor_name": "Downtown Temp 1",
      "alert_type": "high_temperature",
      "severity": "high",
      "message": "High temperature detected at Downtown Temp 1: 38.50°C",
      "value": 38.5,
      "threshold": 35.0,
      "timestamp": "2025-11-19T21:30:00Z",
      "acknowledged": false
    }
  ],
  "meta": {
    "total": 5
  }
}
```

---

### Readings

#### `GET /api/v1/readings`

Get readings from all sensors with filtering and pagination.

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `sensor_id` | UUID | - | Filter by sensor ID |
| `sensor_type` | string | - | Filter by sensor type |
| `from` | timestamp | - | Start time |
| `to` | timestamp | - | End time |
| `page` | integer | 1 | Page number |
| `limit` | integer | 100 | Results per page |
| `sort` | string | `desc` | Sort by timestamp: `asc` or `desc` |

**Response:**

```json
{
  "success": true,
  "data": [
    {
      "id": 12345,
      "sensor_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
      "sensor_name": "Downtown Temp 1",
      "sensor_type": "temperature",
      "value": 22.5,
      "unit": "celsius",
      "timestamp": "2025-11-19T21:30:00Z"
    }
  ],
  "meta": {
    "page": 1,
    "limit": 100,
    "total": 5000
  }
}
```

**cURL Examples:**

```bash
# Get all readings
curl http://localhost:8080/api/v1/readings

# Get temperature readings from last hour
curl "http://localhost:8080/api/v1/readings?sensor_type=temperature&from=2025-11-19T20:00:00Z"

# Get readings for specific sensor
curl "http://localhost:8080/api/v1/readings?sensor_id=a1b2c3d4-e5f6-7890-abcd-ef1234567890"
```

---

#### `GET /api/v1/readings/latest`

Get the latest reading from all sensors (from Redis cache).

**Response:**

```json
{
  "success": true,
  "data": {
    "temperature": [...],
    "pollution": [...],
    "humidity": [...],
    "noise": [...]
  }
}
```

**cURL Example:**

```bash
curl http://localhost:8080/api/v1/readings/latest
```

---

### Analytics

#### `GET /api/v1/analytics/city-stats`

Get city-wide statistics for all sensor types.

**Caching:** 5 minutes

**Response:**

```json
{
  "success": true,
  "data": {
    "avg_temperature": 22.5,
    "avg_pollution": 45.2,
    "avg_humidity": 65.0,
    "avg_noise": 58.3,
    "timestamp": "2025-11-19T21:30:00Z"
  }
}
```

**cURL Example:**

```bash
curl http://localhost:8080/api/v1/analytics/city-stats
```

---

#### `GET /api/v1/analytics/sensors/:id/hourly`

Get hourly aggregated statistics for a sensor.

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | UUID | Sensor ID |

**Query Parameters:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `from` | timestamp | Yes | Start time |
| `to` | timestamp | Yes | End time |

**Response:**

```json
{
  "success": true,
  "data": [
    {
      "sensor_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
      "sensor_type": "temperature",
      "avg_value": 22.5,
      "min_value": 20.0,
      "max_value": 25.0,
      "count": 202,
      "period_start": "2025-11-19T10:00:00Z",
      "period_end": "2025-11-19T11:00:00Z"
    }
  ]
}
```

**cURL Example:**

```bash
curl "http://localhost:8080/api/v1/analytics/sensors/a1b2c3d4-e5f6-7890-abcd-ef1234567890/hourly?from=2025-11-19T00:00:00Z&to=2025-11-19T23:59:59Z"
```

---

#### `GET /api/v1/analytics/top-polluted`

Get the most polluted areas in the city.

**Caching:** 10 minutes

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | integer | 10 | Number of results |

**Response:**

```json
{
  "success": true,
  "data": [
    {
      "sensor_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
      "sensor_name": "Highway Air 2",
      "sensor_type": "pollution",
      "avg_value": 120.5,
      "unit": "µg/m³",
      "location": {
        "latitude": 40.745,
        "longitude": -73.995
      }
    }
  ],
  "meta": {
    "total": 5
  }
}
```

**cURL Example:**

```bash
curl "http://localhost:8080/api/v1/analytics/top-polluted?limit=5"
```

---

#### `GET /api/v1/analytics/top-temperature`

Get the hottest areas in the city.

**Caching:** 10 minutes

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | integer | 10 | Number of results |

**Response:**

```json
{
  "success": true,
  "data": [
    {
      "sensor_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
      "sensor_name": "Park Temp 1",
      "sensor_type": "temperature",
      "avg_value": 38.5,
      "unit": "celsius",
      "location": {
        "latitude": 40.7812,
        "longitude": -73.9665
      }
    }
  ],
  "meta": {
    "total": 5
  }
}
```

**cURL Example:**

```bash
curl "http://localhost:8080/api/v1/analytics/top-temperature?limit=5"
```

---

#### `GET /api/v1/analytics/quietest`

Get the quietest areas in the city.

**Caching:** 10 minutes

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | integer | 10 | Number of results |

**Response:**

```json
{
  "success": true,
  "data": [
    {
      "sensor_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
      "sensor_name": "Park Noise 1",
      "sensor_type": "noise",
      "avg_value": 45.2,
      "unit": "dB",
      "location": {
        "latitude": 40.7785,
        "longitude": -73.9645
      }
    }
  ],
  "meta": {
    "total": 5
  }
}
```

---

### Alerts

#### `GET /api/v1/alerts`

Get all alerts with filtering and pagination.

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `active` | boolean | - | Filter by acknowledgment status (`true` = unacknowledged) |
| `severity` | string | - | Filter by severity: `low`, `medium`, `high`, `critical` |
| `page` | integer | 1 | Page number |
| `limit` | integer | 50 | Results per page (max: 1000) |

**Response:**

```json
{
  "success": true,
  "data": [
    {
      "id": 123,
      "sensor_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
      "sensor_name": "Highway Air 2",
      "sensor_type": "pollution",
      "alert_type": "critical_pollution",
      "severity": "critical",
      "message": "Critical air pollution at Highway Air 2: 155.23 µg/m³",
      "value": 155.23,
      "threshold": 150.0,
      "timestamp": "2025-11-19T21:30:00Z",
      "acknowledged": false,
      "acknowledged_at": null,
      "acknowledged_by": null
    }
  ],
  "meta": {
    "page": 1,
    "limit": 50,
    "total": 150
  }
}
```

**cURL Examples:**

```bash
# Get all active (unacknowledged) alerts
curl "http://localhost:8080/api/v1/alerts?active=true"

# Get critical alerts
curl "http://localhost:8080/api/v1/alerts?severity=critical"

# Get high and critical unacknowledged alerts
curl "http://localhost:8080/api/v1/alerts?active=true&severity=high"
```

---

#### `GET /api/v1/alerts/:id`

Get details of a specific alert.

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | integer | Alert ID |

**Response:**

```json
{
  "success": true,
  "data": {
    "id": 123,
    "sensor_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "sensor_name": "Highway Air 2",
    "sensor_type": "pollution",
    "alert_type": "critical_pollution",
    "severity": "critical",
    "message": "Critical air pollution at Highway Air 2: 155.23 µg/m³",
    "value": 155.23,
    "threshold": 150.0,
    "timestamp": "2025-11-19T21:30:00Z",
    "acknowledged": false
  }
}
```

**cURL Example:**

```bash
curl http://localhost:8080/api/v1/alerts/123
```

---

#### `POST /api/v1/alerts/:id/acknowledge`

Acknowledge an alert to mark it as handled.

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | integer | Alert ID |

**Request Body:**

```json
{
  "acknowledged_by": "admin"
}
```

**Response:**

```json
{
  "success": true,
  "data": {
    "id": 123,
    "acknowledged": true,
    "acknowledged_at": "2025-11-19T21:35:00Z",
    "acknowledged_by": "admin"
  }
}
```

**cURL Example:**

```bash
curl -X POST http://localhost:8080/api/v1/alerts/123/acknowledge \
  -H "Content-Type: application/json" \
  -d '{"acknowledged_by": "admin"}'
```

---

### SSE

#### `GET /stream`

SSE endpoint for real-time sensor updates and alerts.

**Connection:**

```javascript
const eventSource = new EventSource('http://localhost:8080/stream?session_id=<session-id>');
```

**Client handling:**

```javascript
eventSource.onopen = () => {
  console.log('SSE connection established');
};

eventSource.onmessage = (event) => {
  const payload = JSON.parse(event.data);
  console.log(payload.type, payload.data);
};

eventSource.onerror = (err) => {
  console.error('SSE error', err);
};
```

**Server payloads:**

```json
{
  "type": "sensor_update",
  "data": {
    "sensor_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "sensor_type": "temperature",
    "value": 22.5,
    "unit": "celsius",
    "timestamp": "2025-11-19T21:30:00Z"
  }
}
```

**Alert:**
```json
{
  "type": "alert",
  "data": {
    "alert_id": "uuid",
    "sensor_id": "uuid",
    "alert_type": "high_temperature",
    "severity": "high",
    "value": 38.5,
    "threshold": 35.0,
    "message": "High temperature detected...",
    "timestamp": "2025-11-19T21:30:00Z"
  }
}
```

**SSE Stats:**

```bash
# Get SSE connection statistics
curl http://localhost:8080/stream/stats
```

---

## Rate Limiting

**Coming in v1.1:**
- 100 requests per minute per IP address
- SSE: 100 messages per minute per connection

---

## Best Practices

1. **Timestamps**: Always use RFC3339 format (e.g., `2025-11-19T21:30:00Z`)
2. **Pagination**: Use pagination for large datasets
3. **Caching**: Take advantage of cached endpoints for better performance
4. **SSE**: Subscribe only to sensors you need to reduce bandwidth
5. **Error Handling**: Always check the `success` field in responses

---

## Support

For issues or questions:
- GitHub: [smart-city-monitor](https://github.com/chetansierra/smart-city-monitor)
- Email: support@smartcity.example.com

---

**API Version:** 1.0
**Last Updated:** November 19, 2025
