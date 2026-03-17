package anomaly

import (
	"math"
	"sync"

	"github.com/chetansierra/smart-city-monitor/internal/models"
	"github.com/google/uuid"
)

const (
	minSamples     = 30
	zScoreThreshold = 2.5
)

// WelfordState tracks running mean and variance using Welford's online algorithm.
type WelfordState struct {
	Count int64
	Mean  float64
	M2    float64
}

func (w *WelfordState) Update(value float64) {
	w.Count++
	delta := value - w.Mean
	w.Mean += delta / float64(w.Count)
	delta2 := value - w.Mean
	w.M2 += delta * delta2
}

func (w *WelfordState) Variance() float64 {
	if w.Count < 2 {
		return 0
	}
	return w.M2 / float64(w.Count-1)
}

func (w *WelfordState) StdDev() float64 {
	return math.Sqrt(w.Variance())
}

// AnomalyDetector uses Welford's algorithm for O(1) per-reading anomaly detection.
type AnomalyDetector struct {
	mu     sync.RWMutex
	states map[uuid.UUID]*WelfordState
}

// NewAnomalyDetector creates a new detector.
func NewAnomalyDetector() *AnomalyDetector {
	return &AnomalyDetector{
		states: make(map[uuid.UUID]*WelfordState),
	}
}

// ProcessReading updates running stats and returns an AnomalyEvent if z-score exceeds threshold.
func (d *AnomalyDetector) ProcessReading(reading *models.SensorReading) *models.AnomalyEvent {
	d.mu.Lock()
	defer d.mu.Unlock()

	state, ok := d.states[reading.SensorID]
	if !ok {
		state = &WelfordState{}
		d.states[reading.SensorID] = state
	}

	// Update stats with new value
	state.Update(reading.Value)

	// Only detect anomalies after enough samples
	if state.Count < minSamples {
		return nil
	}

	stddev := state.StdDev()
	if stddev < 0.001 {
		return nil
	}

	zScore := math.Abs(reading.Value-state.Mean) / stddev
	if zScore > zScoreThreshold {
		return &models.AnomalyEvent{
			SensorID:       reading.SensorID,
			SensorType:     reading.SensorType,
			Value:          reading.Value,
			ExpectedMean:   math.Round(state.Mean*100) / 100,
			ExpectedStdDev: math.Round(stddev*100) / 100,
			ZScore:         math.Round(zScore*100) / 100,
			Timestamp:      reading.Timestamp,
		}
	}

	return nil
}
