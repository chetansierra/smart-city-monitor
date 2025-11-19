# Postman Collection Guide

This guide explains how to use the Smart City Monitor Postman collection for API testing.

## Table of Contents
- [Installation](#installation)
- [Collection Overview](#collection-overview)
- [Environment Variables](#environment-variables)
- [Testing Workflows](#testing-workflows)
- [Global Scripts](#global-scripts)
- [WebSocket Testing](#websocket-testing)

## Installation

### 1. Import Collection
1. Open Postman
2. Click **Import** button (top left)
3. Select `Smart-City-Monitor.postman_collection.json`
4. Click **Import**

### 2. Import Environment
1. Click **Environments** in left sidebar
2. Click **Import** button
3. Select `Smart-City-Monitor.postman_environment.json`
4. Select **Smart City Monitor - Local** from environment dropdown

### 3. Verify Environment
Ensure these variables are set:
- `base_url`: http://localhost:8080
- `api_version`: v1
- `sensor_id`: (empty - will be auto-populated)
- `alert_id`: (empty - will be auto-populated)
- `ws_url`: ws://localhost:8080/ws

## Collection Overview

The collection includes **26 requests** organized into 6 folders:

### Health Check (1 request)
- **Get Health Status** - Verify API is running

### Sensors (6 requests)
- **Get All Sensors** - List all sensors with filtering
- **Get Sensor by ID** - Retrieve specific sensor
- **Create Sensor** - Add new sensor (auto-saves ID)
- **Update Sensor** - Modify existing sensor
- **Delete Sensor** - Remove sensor
- **Get Sensor Alerts** - Retrieve alerts for a sensor

### Readings (2 requests)
- **Get Latest Readings** - Most recent readings across all sensors
- **Get Readings by Sensor** - Historical data for specific sensor

### Analytics (5 requests)
- **Get Statistics** - Statistical analysis (avg, min, max, count)
- **Get Time Series Data** - Time-bucketed aggregations
- **Get Sensor Comparison** - Compare sensors of same type
- **Get Trends** - Trend analysis (increasing/decreasing/stable)
- **Get Heatmap Data** - Geographic data for mapping

### Alerts (3 requests)
- **Get All Alerts** - List all alerts with filtering
- **Get Alert by ID** - Retrieve specific alert
- **Acknowledge Alert** - Mark alert as handled

### WebSocket (1 request)
- **WebSocket Connection Info** - Instructions for WebSocket testing

## Environment Variables

### Auto-Populated Variables
The collection automatically sets these variables:

#### sensor_id
- Set by: **Create Sensor** request
- Set by: **Get Sensor by ID** request
- Used by: All sensor-specific endpoints

#### alert_id
- Set by: **Get All Alerts** request (uses first alert)
- Used by: Alert detail and acknowledgment endpoints

### Manual Variables
You can manually set or override:
- `base_url` - Change for production/staging
- `api_version` - Update for different API versions

## Testing Workflows

### Workflow 1: Complete Sensor Testing
Run these requests in order:

1. **Get Health Status**
   - Verify API is running
   - Check database and Redis connectivity

2. **Create Sensor**
   - Creates new test sensor
   - Auto-saves `sensor_id` variable
   - Edit request body to customize sensor

3. **Get Sensor by ID**
   - Retrieves the created sensor
   - Verifies creation was successful

4. **Get Readings by Sensor**
   - Check if sensor has any readings
   - May be empty for new sensor

5. **Update Sensor**
   - Modify sensor properties
   - Edit request body as needed

6. **Get Sensor Alerts**
   - Check for any alerts generated
   - May be empty if no threshold violations

7. **Delete Sensor** (optional)
   - Clean up test sensor
   - WARNING: Cannot be undone

### Workflow 2: Alert Management
Run these requests in order:

1. **Get All Alerts**
   - Lists all alerts
   - Auto-saves first alert ID
   - Try filters: `?active=true&severity=critical`

2. **Get Alert by ID**
   - Retrieves specific alert details

3. **Acknowledge Alert**
   - Marks alert as handled
   - Customize `acknowledged_by` field

4. **Get All Alerts** (again)
   - Verify alert is now acknowledged
   - Use `?active=false` to see acknowledged alerts

### Workflow 3: Analytics & Monitoring
Run these requests to analyze data:

1. **Get Latest Readings**
   - See most recent sensor data
   - Adjust `limit` parameter

2. **Get Statistics**
   - Overall statistics
   - Filter by type: `?type=temperature`

3. **Get Time Series Data**
   - Requires `sensor_id` variable
   - Try different intervals: `5m`, `15m`, `1h`, `24h`

4. **Get Trends**
   - Analyze sensor trends
   - Adjust `hours` parameter (default: 24)

5. **Get Sensor Comparison**
   - Compare all sensors of same type
   - Good for identifying outliers

6. **Get Heatmap Data**
   - Geographic distribution
   - Useful for map visualizations

## Global Scripts

The collection includes global test scripts that run on **every request**:

### Performance Test
```javascript
pm.test('Response time is less than 2000ms', function () {
    pm.expect(pm.response.responseTime).to.be.below(2000);
});
```
Ensures all endpoints respond within 2 seconds.

### Content Type Validation
```javascript
pm.test('Response has correct content type', function () {
    pm.expect(pm.response.headers.get('Content-Type')).to.include('application/json');
});
```
Verifies JSON responses.

### Success Field Validation
```javascript
pm.test('Response has success field', function () {
    const jsonData = pm.response.json();
    pm.expect(jsonData).to.have.property('success');
});
```
Ensures standard response format.

### Request-Specific Scripts

#### Create Sensor
Auto-saves created sensor ID:
```javascript
if (pm.response.code === 201) {
    const jsonData = pm.response.json();
    pm.collectionVariables.set('sensor_id', jsonData.data.id);
}
```

#### Get All Alerts
Auto-saves first alert ID:
```javascript
if (jsonData.data && jsonData.data.length > 0) {
    pm.collectionVariables.set('alert_id', jsonData.data[0].id);
}
```

## WebSocket Testing

### Using Postman (v10.18+)
1. Click **New** → **WebSocket Request**
2. Enter URL: `ws://localhost:8080/ws`
3. Click **Connect**

4. Send subscription message:
```json
{
  "type": "subscribe",
  "sensor_types": ["temperature", "pollution"]
}
```

5. Receive messages:
```json
// Reading updates
{
  "type": "reading",
  "data": {
    "sensor_id": "uuid",
    "value": 25.5,
    "timestamp": "2024-01-15T10:30:00Z"
  }
}

// Alert notifications
{
  "type": "alert",
  "data": {
    "alert_id": "uuid",
    "severity": "high",
    "message": "High temperature detected"
  }
}

// Ping (heartbeat)
{
  "type": "ping"
}
```

### Using Other WebSocket Clients

#### wscat (CLI)
```bash
npm install -g wscat
wscat -c ws://localhost:8080/ws

# Send subscription
> {"type":"subscribe","sensor_types":["temperature"]}
```

#### Browser JavaScript
```javascript
const ws = new WebSocket('ws://localhost:8080/ws');

ws.onopen = () => {
  ws.send(JSON.stringify({
    type: 'subscribe',
    sensor_types: ['temperature', 'pollution']
  }));
};

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Received:', data);
};
```

## Query Parameter Reference

### Pagination
All list endpoints support:
- `limit` - Results per page (1-1000, default: 50-100)
- `page` - Page number (default: 1)

### Filtering

#### Sensors
- `type` - temperature, pollution, humidity, noise
- `active` - true/false

#### Alerts
- `active` - true (unacknowledged), false (acknowledged)
- `severity` - low, medium, high, critical

#### Analytics
- `type` - Sensor type filter
- `start_time` - RFC3339 timestamp
- `end_time` - RFC3339 timestamp
- `interval` - Time bucket (5m, 15m, 1h, 24h)
- `hours` - Lookback period for trends

## Tips & Best Practices

### 1. Use Collection Runner
Run entire folders to test multiple endpoints:
1. Click folder (e.g., "Sensors")
2. Click **Run** button
3. Select environment
4. Click **Run Smart City Monitor...**

### 2. Create Test Environments
Duplicate the environment for different setups:
- **Local Development** - http://localhost:8080
- **Docker** - http://localhost:8080 (if using Docker)
- **Staging** - https://staging.example.com
- **Production** - https://api.example.com

### 3. Save Responses as Examples
After running requests:
1. Click **Save Response**
2. Choose **Save as Example**
3. Helps document expected responses

### 4. Chain Requests
Use pre-request scripts to run dependent requests:
```javascript
// Get a sensor ID before running reading request
pm.sendRequest({
    url: pm.environment.get('base_url') + '/api/v1/sensors',
    method: 'GET'
}, (err, res) => {
    const sensors = res.json().data;
    if (sensors.length > 0) {
        pm.collectionVariables.set('sensor_id', sensors[0].id);
    }
});
```

### 5. Monitor API Performance
Use **Postman Monitor** to:
- Schedule automated collection runs
- Track response times over time
- Get alerts for failed requests
- Monitor uptime

## Troubleshooting

### Connection Refused
**Problem**: Cannot connect to `http://localhost:8080`

**Solutions**:
1. Verify API Gateway is running:
   ```bash
   cd backend
   go run ./cmd/api-gateway
   ```
2. Check port 8080 is not in use:
   ```bash
   lsof -i :8080
   ```
3. Verify environment variable `base_url` is correct

### sensor_id Not Set
**Problem**: Requests fail with "Invalid sensor ID"

**Solutions**:
1. Run **Create Sensor** request first
2. Run **Get All Sensors** and manually copy a sensor ID
3. Set `sensor_id` variable manually in environment

### Empty Responses
**Problem**: Endpoints return empty arrays

**Solutions**:
1. Ensure data ingestion service is running
2. Ensure sensor simulator is generating data
3. Check time range parameters aren't too restrictive
4. Verify sensors exist in database

### 401/403 Errors
**Problem**: Authentication/authorization errors

**Note**: Current API has no authentication. If you see these:
1. Check if authentication was added to the API
2. Update collection with auth headers
3. Contact API administrator

### WebSocket Connection Fails
**Problem**: Cannot connect to WebSocket

**Solutions**:
1. Verify API Gateway is running
2. Check WebSocket endpoint is `/ws` not `/ws/`
3. Use `ws://` protocol (not `http://`)
4. Check firewall settings

## Running Collection with Newman (CLI)

### Install Newman
```bash
npm install -g newman
```

### Run Collection
```bash
newman run Smart-City-Monitor.postman_collection.json \
  -e Smart-City-Monitor.postman_environment.json \
  --reporters cli,json \
  --reporter-json-export results.json
```

### Run Specific Folder
```bash
newman run Smart-City-Monitor.postman_collection.json \
  -e Smart-City-Monitor.postman_environment.json \
  --folder "Sensors"
```

### CI/CD Integration
Add to your CI pipeline:
```yaml
# GitHub Actions example
- name: Run API Tests
  run: |
    npm install -g newman
    newman run Smart-City-Monitor.postman_collection.json \
      -e Smart-City-Monitor.postman_environment.json \
      --bail
```

## Next Steps

1. **Explore the Collection** - Run requests and see responses
2. **Customize for Your Needs** - Add custom tests and scripts
3. **Create Documentation** - Use Postman's documentation feature
4. **Share with Team** - Publish to Postman workspace
5. **Automate Testing** - Set up monitors or Newman in CI/CD

## Additional Resources

- [Official API Documentation](./API.md)
- [Postman Learning Center](https://learning.postman.com/)
- [Newman Documentation](https://learning.postman.com/docs/running-collections/using-newman-cli/command-line-integration-with-newman/)
- [WebSocket API Testing](https://learning.postman.com/docs/sending-requests/websocket/websocket/)

---

**Last Updated**: 2024-01-15
**API Version**: v1.0.0
**Collection Version**: 1.0.0
