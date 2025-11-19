package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	apiBaseURL = "http://localhost:8080"
)

// Helper to check if API is available
func isAPIAvailable() bool {
	resp, err := http.Get(apiBaseURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// Helper to make HTTP requests
func makeRequest(t *testing.T, method, path string, body io.Reader) *http.Response {
	req, err := http.NewRequest(method, apiBaseURL+path, body)
	require.NoError(t, err)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	return resp
}

// Test Health Endpoint
func TestHealthEndpoint(t *testing.T) {
	if !isAPIAvailable() {
		t.Skip("API is not running. Start the API gateway with: go run cmd/api-gateway/main.go")
	}

	resp := makeRequest(t, "GET", "/health", nil)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	assert.True(t, result["healthy"].(bool))
	assert.NotNil(t, result["services"])
}

// Test Sensor Endpoints
func TestGetSensors(t *testing.T) {
	if !isAPIAvailable() {
		t.Skip("API is not running")
	}

	resp := makeRequest(t, "GET", "/api/v1/sensors", nil)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	assert.True(t, result["success"].(bool))
	assert.NotNil(t, result["data"])
}

func TestGetSensorByID(t *testing.T) {
	if !isAPIAvailable() {
		t.Skip("API is not running")
	}

	// First get a sensor ID
	resp := makeRequest(t, "GET", "/api/v1/sensors", nil)
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	resp.Body.Close()

	if data, ok := result["data"].([]interface{}); ok && len(data) > 0 {
		if sensor, ok := data[0].(map[string]interface{}); ok {
			sensorID := sensor["id"].(string)

			// Test getting specific sensor
			resp = makeRequest(t, "GET", "/api/v1/sensors/"+sensorID, nil)
			defer resp.Body.Close()

			assert.Equal(t, 200, resp.StatusCode)

			body, _ = io.ReadAll(resp.Body)
			var sensorResult map[string]interface{}
			json.Unmarshal(body, &sensorResult)

			assert.True(t, sensorResult["success"].(bool))
			assert.NotNil(t, sensorResult["data"])
		}
	}
}

func TestGetSensorByID_NotFound(t *testing.T) {
	if !isAPIAvailable() {
		t.Skip("API is not running")
	}

	resp := makeRequest(t, "GET", "/api/v1/sensors/00000000-0000-0000-0000-000000000000", nil)
	defer resp.Body.Close()

	assert.Equal(t, 404, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	assert.False(t, result["success"].(bool))
}

func TestGetSensorByID_InvalidUUID(t *testing.T) {
	if !isAPIAvailable() {
		t.Skip("API is not running")
	}

	resp := makeRequest(t, "GET", "/api/v1/sensors/invalid-uuid", nil)
	defer resp.Body.Close()

	assert.Equal(t, 400, resp.StatusCode)
}

func TestGetSensorLatest(t *testing.T) {
	if !isAPIAvailable() {
		t.Skip("API is not running")
	}

	resp := makeRequest(t, "GET", "/api/v1/sensors", nil)
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	resp.Body.Close()

	if data, ok := result["data"].([]interface{}); ok && len(data) > 0 {
		if sensor, ok := data[0].(map[string]interface{}); ok {
			sensorID := sensor["id"].(string)

			resp = makeRequest(t, "GET", "/api/v1/sensors/"+sensorID+"/latest", nil)
			defer resp.Body.Close()

			assert.Equal(t, 200, resp.StatusCode)
		}
	}
}

// Test Readings Endpoints
func TestGetAllReadings(t *testing.T) {
	if !isAPIAvailable() {
		t.Skip("API is not running")
	}

	resp := makeRequest(t, "GET", "/api/v1/readings", nil)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	assert.True(t, result["success"].(bool))
	assert.NotNil(t, result["data"])
	assert.NotNil(t, result["meta"])
}

func TestGetReadingsWithFilters(t *testing.T) {
	if !isAPIAvailable() {
		t.Skip("API is not running")
	}

	tests := []struct {
		name     string
		queryStr string
	}{
		{"With limit", "/api/v1/readings?limit=10"},
		{"With page", "/api/v1/readings?page=1&limit=10"},
		{"With date range", fmt.Sprintf("/api/v1/readings?from=%s&to=%s",
			time.Now().Add(-24*time.Hour).Format(time.RFC3339),
			time.Now().Format(time.RFC3339))},
		{"With sort", "/api/v1/readings?sort=desc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := makeRequest(t, "GET", tt.queryStr, nil)
			defer resp.Body.Close()

			assert.Equal(t, 200, resp.StatusCode)
		})
	}
}

func TestGetLatestReadings(t *testing.T) {
	if !isAPIAvailable() {
		t.Skip("API is not running")
	}

	resp := makeRequest(t, "GET", "/api/v1/readings/latest", nil)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	assert.True(t, result["success"].(bool))
}

// Test Analytics Endpoints
func TestGetCityStats(t *testing.T) {
	if !isAPIAvailable() {
		t.Skip("API is not running")
	}

	resp := makeRequest(t, "GET", "/api/v1/analytics/city-stats", nil)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	assert.True(t, result["success"].(bool))
	assert.NotNil(t, result["data"])
}

func TestGetTopPolluted(t *testing.T) {
	if !isAPIAvailable() {
		t.Skip("API is not running")
	}

	resp := makeRequest(t, "GET", "/api/v1/analytics/top-polluted?limit=5", nil)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
}

func TestGetTopTemperature(t *testing.T) {
	if !isAPIAvailable() {
		t.Skip("API is not running")
	}

	resp := makeRequest(t, "GET", "/api/v1/analytics/top-temperature?limit=5", nil)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
}

func TestGetQuietest(t *testing.T) {
	if !isAPIAvailable() {
		t.Skip("API is not running")
	}

	resp := makeRequest(t, "GET", "/api/v1/analytics/quietest?limit=5", nil)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)
}

// Test Alerts Endpoints
func TestGetAllAlerts(t *testing.T) {
	if !isAPIAvailable() {
		t.Skip("API is not running")
	}

	resp := makeRequest(t, "GET", "/api/v1/alerts", nil)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	assert.True(t, result["success"].(bool))
}

func TestGetAlertsWithFilters(t *testing.T) {
	if !isAPIAvailable() {
		t.Skip("API is not running")
	}

	tests := []struct {
		name     string
		queryStr string
	}{
		{"Active alerts", "/api/v1/alerts?active=true"},
		{"By severity", "/api/v1/alerts?severity=high"},
		{"With pagination", "/api/v1/alerts?page=1&limit=10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := makeRequest(t, "GET", tt.queryStr, nil)
			defer resp.Body.Close()

			assert.Equal(t, 200, resp.StatusCode)
		})
	}
}

func TestAcknowledgeAlert(t *testing.T) {
	if !isAPIAvailable() {
		t.Skip("API is not running")
	}

	// First get an alert
	resp := makeRequest(t, "GET", "/api/v1/alerts?limit=1", nil)
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	resp.Body.Close()

	if data, ok := result["data"].([]interface{}); ok && len(data) > 0 {
		if alert, ok := data[0].(map[string]interface{}); ok {
			alertID := alert["id"].(string)

			// Acknowledge the alert
			ackBody := map[string]string{"acknowledged_by": "test_user"}
			ackJSON, _ := json.Marshal(ackBody)

			resp = makeRequest(t, "POST", "/api/v1/alerts/"+alertID+"/acknowledge", bytes.NewReader(ackJSON))
			defer resp.Body.Close()

			assert.Equal(t, 200, resp.StatusCode)
		}
	}
}

// Test Error Cases
func TestInvalidPaginationParams(t *testing.T) {
	if !isAPIAvailable() {
		t.Skip("API is not running")
	}

	resp := makeRequest(t, "GET", "/api/v1/readings?limit=9999999", nil)
	defer resp.Body.Close()

	assert.Equal(t, 400, resp.StatusCode)
}

func TestInvalidDateFormat(t *testing.T) {
	if !isAPIAvailable() {
		t.Skip("API is not running")
	}

	resp := makeRequest(t, "GET", "/api/v1/readings?from=invalid-date", nil)
	defer resp.Body.Close()

	assert.Equal(t, 400, resp.StatusCode)
}

// Test CORS Headers
func TestCORSHeaders(t *testing.T) {
	if !isAPIAvailable() {
		t.Skip("API is not running")
	}

	req, _ := http.NewRequest("OPTIONS", apiBaseURL+"/api/v1/sensors", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Contains(t, resp.Header.Get("Access-Control-Allow-Origin"), "*")
}

// Test Security Headers
func TestSecurityHeaders(t *testing.T) {
	if !isAPIAvailable() {
		t.Skip("API is not running")
	}

	resp := makeRequest(t, "GET", "/api/v1/sensors", nil)
	defer resp.Body.Close()

	assert.NotEmpty(t, resp.Header.Get("X-Content-Type-Options"))
	assert.NotEmpty(t, resp.Header.Get("X-Frame-Options"))
}
