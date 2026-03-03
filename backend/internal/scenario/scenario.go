package scenario

import (
	"encoding/json"
	"time"
)

// ScenarioType represents different scenario types
type ScenarioType string

const (
	ScenarioNormal             ScenarioType = "normal"
	ScenarioRushHour           ScenarioType = "rush_hour"
	ScenarioHeatwave           ScenarioType = "heatwave"
	ScenarioIndustrialIncident ScenarioType = "industrial_incident"
)

// Scenario represents a simulation scenario configuration
type Scenario struct {
	Type        ScenarioType          `json:"type"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Duration    time.Duration         `json:"duration"`
	Modifiers   map[string]*Modifier  `json:"modifiers"`
	ZoneFilters map[string]bool       `json:"zone_filters,omitempty"` // Apply to specific zones only
	StartedAt   time.Time             `json:"started_at,omitempty"`
	Active      bool                  `json:"active"`
}

// Modifier represents changes to sensor values
type Modifier struct {
	SensorType    string  `json:"sensor_type"`    // "pollution", "temperature", "noise"
	Operation     string  `json:"operation"`      // "add", "multiply", "set"
	Value         float64 `json:"value"`          // The value to apply
	GradualChange bool    `json:"gradual_change"` // Whether to gradually apply changes
	ZoneSpecific  string  `json:"zone_specific,omitempty"` // Apply only to specific zone
}

// GetNormalScenario returns the baseline normal operations scenario
func GetNormalScenario() *Scenario {
	return &Scenario{
		Type:        ScenarioNormal,
		Name:        "Normal Operations",
		Description: "Standard city operations with natural variations",
		Duration:    0, // Runs indefinitely
		Modifiers:   map[string]*Modifier{},
		Active:      false,
	}
}

// GetRushHourScenario returns the rush hour traffic scenario
func GetRushHourScenario() *Scenario {
	return &Scenario{
		Type:        ScenarioRushHour,
		Name:        "Rush Hour Traffic",
		Description: "Peak traffic hours with increased noise and pollution in downtown areas",
		Duration:    10 * time.Minute, // Default 10 minutes, configurable
		Modifiers: map[string]*Modifier{
			"noise": {
				SensorType:    "noise",
				Operation:     "multiply",
				Value:         1.4, // +40%
				GradualChange: true,
			},
			"pollution_downtown": {
				SensorType:    "pollution",
				Operation:     "multiply",
				Value:         1.6, // +60%
				GradualChange: true,
				ZoneSpecific:  "downtown",
			},
			"pollution_general": {
				SensorType:    "pollution",
				Operation:     "multiply",
				Value:         1.2, // +20% for other areas
				GradualChange: true,
			},
		},
		Active: false,
	}
}

// GetHeatwaveScenario returns the extreme heat scenario
func GetHeatwaveScenario() *Scenario {
	return &Scenario{
		Type:        ScenarioHeatwave,
		Name:        "Heatwave Alert",
		Description: "Extreme temperatures affecting the entire city",
		Duration:    15 * time.Minute,
		Modifiers: map[string]*Modifier{
			"temperature": {
				SensorType:    "temperature",
				Operation:     "add",
				Value:         15.0, // +15°C
				GradualChange: true,
			},
			"humidity": {
				SensorType:    "humidity",
				Operation:     "add",
				Value:         -20.0, // -20%
				GradualChange: true,
			},
		},
		Active: false,
	}
}

// GetIndustrialIncidentScenario returns the industrial pollution incident
func GetIndustrialIncidentScenario() *Scenario {
	return &Scenario{
		Type:        ScenarioIndustrialIncident,
		Name:        "Industrial Incident",
		Description: "Chemical leak causing severe air quality issues in industrial zone",
		Duration:    8 * time.Minute,
		Modifiers: map[string]*Modifier{
			"pollution_industrial": {
				SensorType:    "pollution",
				Operation:     "set",
				Value:         250.0, // AQI 250+ (Very Unhealthy)
				GradualChange: false, // Immediate spike
				ZoneSpecific:  "industrial",
			},
			"pollution_spread": {
				SensorType:    "pollution",
				Operation:     "multiply",
				Value:         1.3, // +30% spread to nearby zones
				GradualChange: true,
			},
		},
		ZoneFilters: map[string]bool{
			"industrial": true,
			"downtown":   true, // Spreads to downtown
		},
		Active: false,
	}
}

// GetAllScenarios returns all available scenarios
func GetAllScenarios() map[ScenarioType]*Scenario {
	return map[ScenarioType]*Scenario{
		ScenarioNormal:             GetNormalScenario(),
		ScenarioRushHour:           GetRushHourScenario(),
		ScenarioHeatwave:           GetHeatwaveScenario(),
		ScenarioIndustrialIncident: GetIndustrialIncidentScenario(),
	}
}

// ToJSON converts scenario to JSON string
func (s *Scenario) ToJSON() (string, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON parses scenario from JSON string
func FromJSON(jsonStr string) (*Scenario, error) {
	var s Scenario
	err := json.Unmarshal([]byte(jsonStr), &s)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// ApplyModifier applies a modifier to a sensor value
func ApplyModifier(currentValue float64, modifier *Modifier, progress float64) float64 {
	if modifier == nil {
		return currentValue
	}

	// Calculate effective value based on gradual change
	effectiveValue := modifier.Value
	if modifier.GradualChange && progress < 1.0 {
		// Gradually ramp up the effect
		switch modifier.Operation {
		case "add":
			effectiveValue = modifier.Value * progress
		case "multiply":
			// Interpolate between 1.0 and target multiplier
			effectiveValue = 1.0 + (modifier.Value-1.0)*progress
		}
	}

	// Apply the operation
	switch modifier.Operation {
	case "add":
		return currentValue + effectiveValue
	case "multiply":
		return currentValue * effectiveValue
	case "set":
		if modifier.GradualChange {
			// Interpolate from current to target
			return currentValue + (effectiveValue-currentValue)*progress
		}
		return effectiveValue
	default:
		return currentValue
	}
}
