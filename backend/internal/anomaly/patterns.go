package anomaly

import (
	"fmt"
	"sync"
	"time"

	"github.com/chetansierra/smart-city-monitor/internal/models"
	"github.com/chetansierra/smart-city-monitor/internal/zones"
	"github.com/google/uuid"
)

const (
	windowDuration    = 2 * time.Minute
	debounceDuration  = 5 * time.Minute
	eventExpiry       = 10 * time.Minute
)

type zoneReading struct {
	SensorID   uuid.UUID
	SensorType models.SensorType
	Value      float64
	Timestamp  time.Time
}

// PatternDetector detects zone-level patterns from sliding windows of sensor readings.
type PatternDetector struct {
	mu          sync.Mutex
	windows     map[string][]zoneReading // zone -> recent readings
	lastDetected map[string]time.Time    // "zone:pattern" -> last detection time
}

// NewPatternDetector creates a new pattern detector.
func NewPatternDetector() *PatternDetector {
	return &PatternDetector{
		windows:      make(map[string][]zoneReading),
		lastDetected: make(map[string]time.Time),
	}
}

// ProcessReading adds a reading to the zone window and checks for patterns.
func (p *PatternDetector) ProcessReading(reading *models.SensorReading) *models.DetectedEvent {
	zone := zones.GetZone(reading.Latitude, reading.Longitude)
	if zone == "unknown" {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Add to window
	p.windows[zone] = append(p.windows[zone], zoneReading{
		SensorID:   reading.SensorID,
		SensorType: reading.SensorType,
		Value:      reading.Value,
		Timestamp:  reading.Timestamp,
	})

	// Prune old entries
	cutoff := time.Now().Add(-windowDuration)
	entries := p.windows[zone]
	pruned := entries[:0]
	for _, e := range entries {
		if e.Timestamp.After(cutoff) {
			pruned = append(pruned, e)
		}
	}
	p.windows[zone] = pruned

	// Check patterns
	if event := p.checkIndustrialIncident(zone, pruned); event != nil {
		return event
	}
	if event := p.checkHeatwave(zone, pruned); event != nil {
		return event
	}
	if event := p.checkRushHour(zone, pruned); event != nil {
		return event
	}

	return nil
}

func (p *PatternDetector) debounced(zone, pattern string) bool {
	key := fmt.Sprintf("%s:%s", zone, pattern)
	if last, ok := p.lastDetected[key]; ok {
		if time.Since(last) < debounceDuration {
			return true
		}
	}
	return false
}

func (p *PatternDetector) markDetected(zone, pattern string) {
	key := fmt.Sprintf("%s:%s", zone, pattern)
	p.lastDetected[key] = time.Now()
}

// checkIndustrialIncident: 3+ pollution sensors > 150 µg/m³ in same zone within 2 min
func (p *PatternDetector) checkIndustrialIncident(zone string, entries []zoneReading) *models.DetectedEvent {
	if p.debounced(zone, "industrial_incident") {
		return nil
	}

	sensorIDs := make(map[uuid.UUID]bool)
	for _, e := range entries {
		if e.SensorType == models.SensorTypePollution && e.Value > 150 {
			sensorIDs[e.SensorID] = true
		}
	}

	if len(sensorIDs) >= 3 {
		p.markDetected(zone, "industrial_incident")
		ids := make([]uuid.UUID, 0, len(sensorIDs))
		for id := range sensorIDs {
			ids = append(ids, id)
		}
		return &models.DetectedEvent{
			EventType:   "industrial_incident",
			Zone:        zone,
			Description: fmt.Sprintf("Industrial incident detected: %d pollution sensors above 150 µg/m³ in %s", len(sensorIDs), zone),
			SensorIDs:   ids,
			Timestamp:   time.Now(),
			ExpiresAt:   time.Now().Add(eventExpiry),
		}
	}
	return nil
}

// checkHeatwave: zone avg temp > 38°C AND avg humidity < 30%
func (p *PatternDetector) checkHeatwave(zone string, entries []zoneReading) *models.DetectedEvent {
	if p.debounced(zone, "heatwave") {
		return nil
	}

	var tempSum, humSum float64
	var tempCount, humCount int
	tempSensors := make(map[uuid.UUID]bool)
	humSensors := make(map[uuid.UUID]bool)

	for _, e := range entries {
		switch e.SensorType {
		case models.SensorTypeTemperature:
			tempSum += e.Value
			tempCount++
			tempSensors[e.SensorID] = true
		case models.SensorTypeHumidity:
			humSum += e.Value
			humCount++
			humSensors[e.SensorID] = true
		}
	}

	if tempCount > 0 && humCount > 0 {
		avgTemp := tempSum / float64(tempCount)
		avgHum := humSum / float64(humCount)

		if avgTemp > 38 && avgHum < 30 {
			p.markDetected(zone, "heatwave")
			ids := make([]uuid.UUID, 0)
			for id := range tempSensors {
				ids = append(ids, id)
			}
			for id := range humSensors {
				ids = append(ids, id)
			}
			return &models.DetectedEvent{
				EventType:   "heatwave",
				Zone:        zone,
				Description: fmt.Sprintf("Heatwave detected in %s: avg temp %.1f°C, avg humidity %.1f%%", zone, avgTemp, avgHum),
				SensorIDs:   ids,
				Timestamp:   time.Now(),
				ExpiresAt:   time.Now().Add(eventExpiry),
			}
		}
	}
	return nil
}

// checkRushHour: zone avg noise > 70 dB AND avg pollution > 80 µg/m³
func (p *PatternDetector) checkRushHour(zone string, entries []zoneReading) *models.DetectedEvent {
	if p.debounced(zone, "rush_hour") {
		return nil
	}

	var noiseSum, pollutionSum float64
	var noiseCount, pollutionCount int
	sensorIDs := make(map[uuid.UUID]bool)

	for _, e := range entries {
		switch e.SensorType {
		case models.SensorTypeNoise:
			noiseSum += e.Value
			noiseCount++
			sensorIDs[e.SensorID] = true
		case models.SensorTypePollution:
			pollutionSum += e.Value
			pollutionCount++
			sensorIDs[e.SensorID] = true
		}
	}

	if noiseCount > 0 && pollutionCount > 0 {
		avgNoise := noiseSum / float64(noiseCount)
		avgPollution := pollutionSum / float64(pollutionCount)

		if avgNoise > 70 && avgPollution > 80 {
			p.markDetected(zone, "rush_hour")
			ids := make([]uuid.UUID, 0, len(sensorIDs))
			for id := range sensorIDs {
				ids = append(ids, id)
			}
			return &models.DetectedEvent{
				EventType:   "rush_hour",
				Zone:        zone,
				Description: fmt.Sprintf("Rush hour detected in %s: avg noise %.1f dB, avg pollution %.1f µg/m³", zone, avgNoise, avgPollution),
				SensorIDs:   ids,
				Timestamp:   time.Now(),
				ExpiresAt:   time.Now().Add(eventExpiry),
			}
		}
	}
	return nil
}
