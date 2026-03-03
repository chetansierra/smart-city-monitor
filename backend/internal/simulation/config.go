package simulation

import (
	"encoding/json"
	"time"
)

// SensorBehaviorPattern represents different patterns for sensor data generation
type SensorBehaviorPattern string

const (
	PatternSteady     SensorBehaviorPattern = "steady"      // Consistent values with minimal variance
	PatternSineWave   SensorBehaviorPattern = "sine_wave"   // Sinusoidal pattern over time
	PatternRandomSpike SensorBehaviorPattern = "random_spike" // Random occasional spikes
	PatternLinear     SensorBehaviorPattern = "linear"      // Linear increase/decrease
	PatternChaotic    SensorBehaviorPattern = "chaotic"     // Highly variable random values
)

// ThresholdConfig represents alert threshold configuration for a sensor
type ThresholdConfig struct {
	SensorID      string  `json:"sensor_id"`
	SensorType    string  `json:"sensor_type"`
	WarningMin    float64 `json:"warning_min"`
	WarningMax    float64 `json:"warning_max"`
	CriticalMin   float64 `json:"critical_min"`
	CriticalMax   float64 `json:"critical_max"`
	Enabled       bool    `json:"enabled"`
	CustomMessage string  `json:"custom_message,omitempty"`
}

// DefaultThresholds returns default threshold configurations by sensor type
func DefaultThresholds(sensorType string) *ThresholdConfig {
	defaults := map[string]*ThresholdConfig{
		"pollution": {
			SensorType:  "pollution",
			WarningMin:  0,
			WarningMax:  100,
			CriticalMin: 0,
			CriticalMax: 150,
			Enabled:     true,
		},
		"temperature": {
			SensorType:  "temperature",
			WarningMin:  -10,
			WarningMax:  35,
			CriticalMin: -20,
			CriticalMax: 45,
			Enabled:     true,
		},
		"noise": {
			SensorType:  "noise",
			WarningMin:  0,
			WarningMax:  70,
			CriticalMin: 0,
			CriticalMax: 85,
			Enabled:     true,
		},
	}

	if config, exists := defaults[sensorType]; exists {
		return config
	}

	// Generic default
	return &ThresholdConfig{
		SensorType:  sensorType,
		WarningMin:  0,
		WarningMax:  100,
		CriticalMin: 0,
		CriticalMax: 150,
		Enabled:     true,
	}
}

// SensorBehavior represents custom behavior configuration for a sensor
type SensorBehavior struct {
	SensorID     string                `json:"sensor_id"`
	Pattern      SensorBehaviorPattern `json:"pattern"`
	MinValue     float64               `json:"min_value"`
	MaxValue     float64               `json:"max_value"`
	Variance     float64               `json:"variance"`      // Percentage variance (0-100)
	UpdateRate   int                   `json:"update_rate"`   // Seconds between updates
	Enabled      bool                  `json:"enabled"`
	Location     *Location             `json:"location,omitempty"`
}

// Location represents custom geographic coordinates
type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Zone      string  `json:"zone,omitempty"`
}

// ZoneControl represents bulk control settings for a zone
type ZoneControl struct {
	ZoneName    string                 `json:"zone_name"`
	SensorIDs   []string               `json:"sensor_ids"`
	Modifiers   map[string]float64     `json:"modifiers"`    // sensor_type -> multiplier
	Behavior    *SensorBehavior        `json:"behavior,omitempty"`
	Active      bool                   `json:"active"`
	AppliedAt   time.Time              `json:"applied_at"`
}

// TimeCompression represents time acceleration settings
type TimeCompression struct {
	Multiplier float64   `json:"multiplier"` // 1.0 = normal, 2.0 = 2x speed, etc.
	Active     bool      `json:"active"`
	StartedAt  time.Time `json:"started_at"`
}

// ChaosConfig represents chaos engineering configuration
type ChaosConfig struct {
	Enabled             bool    `json:"enabled"`
	FailureRate         float64 `json:"failure_rate"`          // Percentage of sensors that can fail (0-100)
	NetworkLagMin       int     `json:"network_lag_min"`       // Min lag in milliseconds
	NetworkLagMax       int     `json:"network_lag_max"`       // Max lag in milliseconds
	DataCorruptionRate  float64 `json:"data_corruption_rate"`  // Percentage of corrupted readings (0-100)
	RecoveryTimeMin     int     `json:"recovery_time_min"`     // Min recovery time in seconds
	RecoveryTimeMax     int     `json:"recovery_time_max"`     // Max recovery time in seconds
	AffectedSensors     []string `json:"affected_sensors,omitempty"`
}

// SimulationConfig represents the complete custom simulation configuration
type SimulationConfig struct {
	Thresholds       map[string]*ThresholdConfig `json:"thresholds"`        // sensor_id -> threshold config
	SensorBehaviors  map[string]*SensorBehavior  `json:"sensor_behaviors"`  // sensor_id -> behavior
	ZoneControls     map[string]*ZoneControl     `json:"zone_controls"`     // zone_name -> zone control
	TimeCompression  *TimeCompression            `json:"time_compression"`
	ChaosMode        *ChaosConfig                `json:"chaos_mode"`
	UpdatedAt        time.Time                   `json:"updated_at"`
}

// NewSimulationConfig creates a new simulation configuration with defaults
func NewSimulationConfig() *SimulationConfig {
	return &SimulationConfig{
		Thresholds:      make(map[string]*ThresholdConfig),
		SensorBehaviors: make(map[string]*SensorBehavior),
		ZoneControls:    make(map[string]*ZoneControl),
		TimeCompression: &TimeCompression{
			Multiplier: 1.0,
			Active:     false,
		},
		ChaosMode: &ChaosConfig{
			Enabled:            false,
			FailureRate:        0,
			NetworkLagMin:      0,
			NetworkLagMax:      0,
			DataCorruptionRate: 0,
			RecoveryTimeMin:    10,
			RecoveryTimeMax:    60,
		},
		UpdatedAt: time.Now(),
	}
}

// ToJSON converts simulation config to JSON string
func (sc *SimulationConfig) ToJSON() (string, error) {
	data, err := json.Marshal(sc)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON parses simulation config from JSON string
func FromJSON(jsonStr string) (*SimulationConfig, error) {
	var sc SimulationConfig
	err := json.Unmarshal([]byte(jsonStr), &sc)
	if err != nil {
		return nil, err
	}
	return &sc, nil
}

// ApplyThreshold applies a threshold override for a sensor
func (sc *SimulationConfig) ApplyThreshold(sensorID string, threshold *ThresholdConfig) {
	threshold.SensorID = sensorID
	sc.Thresholds[sensorID] = threshold
	sc.UpdatedAt = time.Now()
}

// ApplySensorBehavior applies custom behavior for a sensor
func (sc *SimulationConfig) ApplySensorBehavior(sensorID string, behavior *SensorBehavior) {
	behavior.SensorID = sensorID
	sc.SensorBehaviors[sensorID] = behavior
	sc.UpdatedAt = time.Now()
}

// ApplyZoneControl applies bulk control to a zone
func (sc *SimulationConfig) ApplyZoneControl(zoneName string, control *ZoneControl) {
	control.ZoneName = zoneName
	control.AppliedAt = time.Now()
	sc.ZoneControls[zoneName] = control
	sc.UpdatedAt = time.Now()
}

// SetTimeCompression sets time compression multiplier
func (sc *SimulationConfig) SetTimeCompression(multiplier float64) {
	sc.TimeCompression.Multiplier = multiplier
	sc.TimeCompression.Active = multiplier != 1.0
	if sc.TimeCompression.Active {
		sc.TimeCompression.StartedAt = time.Now()
	}
	sc.UpdatedAt = time.Now()
}

// SetChaosMode configures chaos engineering settings
func (sc *SimulationConfig) SetChaosMode(config *ChaosConfig) {
	sc.ChaosMode = config
	sc.UpdatedAt = time.Now()
}

// RemoveThreshold removes a threshold override
func (sc *SimulationConfig) RemoveThreshold(sensorID string) {
	delete(sc.Thresholds, sensorID)
	sc.UpdatedAt = time.Now()
}

// RemoveSensorBehavior removes custom behavior
func (sc *SimulationConfig) RemoveSensorBehavior(sensorID string) {
	delete(sc.SensorBehaviors, sensorID)
	sc.UpdatedAt = time.Now()
}

// RemoveZoneControl removes zone control
func (sc *SimulationConfig) RemoveZoneControl(zoneName string) {
	delete(sc.ZoneControls, zoneName)
	sc.UpdatedAt = time.Now()
}

// ResetToDefaults resets all custom configurations
func (sc *SimulationConfig) ResetToDefaults() {
	sc.Thresholds = make(map[string]*ThresholdConfig)
	sc.SensorBehaviors = make(map[string]*SensorBehavior)
	sc.ZoneControls = make(map[string]*ZoneControl)
	sc.TimeCompression = &TimeCompression{
		Multiplier: 1.0,
		Active:     false,
	}
	sc.ChaosMode = &ChaosConfig{
		Enabled:            false,
		FailureRate:        0,
		NetworkLagMin:      0,
		NetworkLagMax:      0,
		DataCorruptionRate: 0,
		RecoveryTimeMin:    10,
		RecoveryTimeMax:    60,
	}
	sc.UpdatedAt = time.Now()
}
