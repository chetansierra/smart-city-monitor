package main

import (
	"context"
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sync"

	"github.com/chetansierra/smart-city-monitor/internal/config"
	"github.com/chetansierra/smart-city-monitor/internal/kafka"
	"github.com/chetansierra/smart-city-monitor/internal/logger"
	"github.com/chetansierra/smart-city-monitor/internal/models"
	"github.com/chetansierra/smart-city-monitor/internal/postgres"
	"github.com/chetansierra/smart-city-monitor/internal/redis"
	"github.com/chetansierra/smart-city-monitor/internal/scenario"
	"github.com/chetansierra/smart-city-monitor/internal/simulation"
	"github.com/google/uuid"
	rds "github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}
	if err := cfg.Validate("sensor-simulator"); err != nil {
		log.Fatal().Err(err).Msg("Invalid configuration")
	}

	// Setup logger
	logger.Setup(logger.Config{
		Level:      cfg.App.LogLevel,
		Service:    "sensor-simulator",
		PrettyLogs: cfg.App.Environment == "development",
	})

	log.Info().Msg("Starting Smart City Sensor Simulator")

	// Connect to database to load sensors
	dbCfg := postgres.Config{
		Host:               cfg.Postgres.Host,
		Port:               cfg.Postgres.Port,
		Database:           cfg.Postgres.Database,
		User:               cfg.Postgres.User,
		Password:           cfg.Postgres.Password,
		SSLMode:            cfg.Postgres.SSLMode,
		MaxConnections:     cfg.Postgres.MaxConnections,
		MaxIdleConnections: cfg.Postgres.MaxIdleConnections,
	}

	db, err := postgres.Connect(dbCfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	// Load sensors from database
	ctx := context.Background()
	sensors, err := db.GetAllSensors(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load sensors")
	}
	log.Info().Int("count", len(sensors)).Msg("Loaded sensors from database")

	// Create Kafka producer
	producer, err := kafka.NewProducer(kafka.ProducerConfig{
		Brokers: cfg.Kafka.Brokers,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Kafka producer")
	}
	defer producer.Close()

	// Connect to Redis
	redisCfg := redis.Config{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}
	redisClient, err := redis.Connect(redisCfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	defer redisClient.Close()

	// Handle StartEmpty flag
	initialSensors := sensors
	if cfg.Simulator.StartEmpty {
		log.Info().Msg("Simulator started in empty mode (waiting for commands)")
		initialSensors = []models.Sensor{}
	}

	// Create simulator
	simulator := NewSimulator(initialSensors, producer, redisClient, db, cfg)

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start simulation
	go simulator.Start()

	// Wait for shutdown signal
	<-sigChan
	log.Info().Msg("Shutting down simulator")
	simulator.Stop()
	log.Info().Msg("Simulator stopped gracefully")
}

// Simulator generates sensor data
type Simulator struct {
	sensors      []models.Sensor
	producer     *kafka.Producer
	redis        *redis.Client
	db           *postgres.DB
	config       *config.Config
	stopChan     chan bool
	rand         *rand.Rand
	randMu       sync.Mutex
	sensorStates map[uuid.UUID]*sensorState
	controls     runtimeControls
	mu           sync.RWMutex
}

// NewSimulator creates a new simulator
func NewSimulator(sensors []models.Sensor, producer *kafka.Producer, redisClient *redis.Client, db *postgres.DB, cfg *config.Config) *Simulator {
	sim := &Simulator{
		sensors:      sensors,
		producer:     producer,
		redis:        redisClient,
		db:           db,
		config:       cfg,
		stopChan:     make(chan bool),
		rand:         rand.New(rand.NewSource(time.Now().UnixNano())),
		sensorStates: make(map[uuid.UUID]*sensorState),
		controls: runtimeControls{
			simulationConfig: simulation.NewSimulationConfig(),
			scenario:         scenario.GetNormalScenario(),
		},
	}
	for _, sensor := range sensors {
		sim.sensorStates[sensor.ID] = sim.newSensorState(sensor)
	}
	return sim
}

// Start begins the simulation
func (s *Simulator) Start() {
	log.Info().
		Int("sensor_count", len(s.sensors)).
		Int("interval_ms", s.config.Simulator.GenerationIntervalMS).
		Msg("Starting simulation")

	// Start command listener
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.listenForCommands(ctx)
	go s.syncSensorsFromDB(ctx)
	go s.syncRuntimeControls(ctx)

	ticker := time.NewTicker(time.Duration(s.config.Simulator.GenerationIntervalMS) * time.Millisecond)
	defer ticker.Stop()

	messageCount := 0

	for {
		select {
		case <-ticker.C:
			s.mu.RLock()
			sensorsToProcess := make([]models.Sensor, len(s.sensors))
			copy(sensorsToProcess, s.sensors)
			s.mu.RUnlock()

			// Generate readings for all sensors
			for _, sensor := range sensorsToProcess {
				reading, ok := s.generateReading(sensor)
				if !ok {
					continue
				}

				var sessionIDStr string
				if sensor.SessionID != nil {
					sessionIDStr = sensor.SessionID.String()
				}

				// Convert to Kafka message format
				kafkaMsg := models.KafkaMessage{
					SensorID:   sensor.ID.String(),
					SessionID:  sessionIDStr,
					SensorType: string(sensor.Type),
					Value:      reading.Value,
					Unit:       reading.Unit,
					Location: models.Location{
						Latitude:  sensor.Latitude,
						Longitude: sensor.Longitude,
					},
					Timestamp: reading.Timestamp.Format(time.RFC3339Nano),
				}

				// Send to Kafka
				if err := s.producer.SendMessage(s.config.Kafka.TopicSensorReadings, sensor.ID.String(), kafkaMsg); err != nil {
					log.Error().
						Err(err).
						Str("sensor_id", sensor.ID.String()).
						Str("sensor_type", string(sensor.Type)).
						Msg("Failed to send message to Kafka")
				}
				messageCount++
			}

			if messageCount%100 == 0 && messageCount > 0 {
				log.Debug().Int("messages_sent", messageCount).Msg("Kafka messages sent")
			}

		case <-s.stopChan:
			log.Info().Int("total_messages", messageCount).Msg("Simulation stopped")
			return
		}
	}
}

func (s *Simulator) syncSensorsFromDB(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			dbSensors, err := s.db.GetAllSensors(ctx)
			if err != nil {
				log.Error().Err(err).Msg("Failed to sync sensors from database")
				continue
			}

			s.mu.Lock()

			existing := make(map[uuid.UUID]int, len(s.sensors))
			for i, sensor := range s.sensors {
				existing[sensor.ID] = i
			}

			presentInDB := make(map[uuid.UUID]models.Sensor, len(dbSensors))
			for _, dbSensor := range dbSensors {
				presentInDB[dbSensor.ID] = dbSensor
				if idx, ok := existing[dbSensor.ID]; ok {
					s.sensors[idx].Type = dbSensor.Type
					s.sensors[idx].Latitude = dbSensor.Latitude
					s.sensors[idx].Longitude = dbSensor.Longitude
					s.sensors[idx].SessionID = dbSensor.SessionID
					if _, exists := s.sensorStates[dbSensor.ID]; !exists {
						s.sensorStates[dbSensor.ID] = s.newSensorState(dbSensor)
					}
					continue
				}
				s.sensors = append(s.sensors, dbSensor)
				s.sensorStates[dbSensor.ID] = s.newSensorState(dbSensor)
			}

			filteredSensors := make([]models.Sensor, 0, len(s.sensors))
			for _, sensor := range s.sensors {
				if _, ok := presentInDB[sensor.ID]; ok {
					filteredSensors = append(filteredSensors, sensor)
				} else {
					delete(s.sensorStates, sensor.ID)
				}
			}
			s.sensors = filteredSensors

			s.mu.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

func (s *Simulator) listenForCommands(ctx context.Context) {
	log.Info().Str("channel", redis.ChannelSimulation).Msg("Subscribing to simulation commands")
	pubsub := s.redis.Subscribe(ctx, redis.ChannelSimulation)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for msg := range ch {
		log.Info().Str("payload", msg.Payload).Msg("Received simulation command")
		var cmd redis.SimulationCommand
		if err := json.Unmarshal([]byte(msg.Payload), &cmd); err != nil {
			log.Error().Err(err).Msg("Failed to unmarshal simulation command")
			continue
		}

		s.handleCommand(cmd)
	}
}

func (s *Simulator) handleCommand(cmd redis.SimulationCommand) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch cmd.Type {
	case redis.CommandCreateSensor:
		payload, ok := cmd.Payload.(map[string]interface{})
		if !ok {
			log.Error().Msg("Invalid payload for CreateSensor command")
			return
		}

		// Use payload values if provided, fall back to defaults
		sensorID := uuid.New()
		if idStr, ok := payload["id"].(string); ok {
			if parsedID, err := uuid.Parse(idStr); err == nil {
				sensorID = parsedID
			}
		}

		sensorType := models.SensorTypeTemperature
		if t, ok := payload["type"].(string); ok {
			sensorType = models.SensorType(t)
		}

		latitude := 40.7128 + (s.randFloat()*0.02 - 0.01) // NYC default
		if lat, ok := payload["lat"].(float64); ok {
			latitude = lat
		}

		longitude := -74.0060 + (s.randFloat()*0.02 - 0.01)
		if lon, ok := payload["lon"].(float64); ok {
			longitude = lon
		}

		newSensor := models.Sensor{
			ID:        sensorID,
			SessionID: &cmd.SessionID,
			Type:      sensorType,
			Latitude:  latitude,
			Longitude: longitude,
			Status:    models.SensorStatusActive,
		}

		s.sensors = append(s.sensors, newSensor)
		s.sensorStates[newSensor.ID] = s.newSensorState(newSensor)
		log.Info().
			Str("session_id", cmd.SessionID.String()).
			Str("sensor_id", newSensor.ID.String()).
			Str("type", string(newSensor.Type)).
			Msg("Dynamically added sensor to simulation")

	case redis.CommandShutdownSession:
		newSensors := []models.Sensor{}
		count := 0
		for _, sensor := range s.sensors {
			if sensor.SessionID == nil || sensor.SessionID.String() != cmd.SessionID.String() {
				newSensors = append(newSensors, sensor)
			} else {
				count++
			}
		}
		s.sensors = newSensors
		for sensorID := range s.sensorStates {
			keep := false
			for _, sensor := range s.sensors {
				if sensor.ID == sensorID {
					keep = true
					break
				}
			}
			if !keep {
				delete(s.sensorStates, sensorID)
			}
		}
		log.Info().
			Str("session_id", cmd.SessionID.String()).
			Int("removed_count", count).
			Msg("Shut down session simulation")

	case redis.CommandDeleteSensor:
		payload, ok := cmd.Payload.(map[string]interface{})
		if !ok {
			log.Error().Msg("Invalid payload for delete sensor command")
			return
		}

		idStr, ok := payload["id"].(string)
		if !ok {
			log.Error().Msg("Missing id in delete sensor payload")
			return
		}
		sensorID, err := uuid.Parse(idStr)
		if err != nil {
			log.Error().Err(err).Str("id", idStr).Msg("Invalid sensor ID in delete sensor payload")
			return
		}

		newSensors := make([]models.Sensor, 0, len(s.sensors))
		removed := false
		for _, sensor := range s.sensors {
			if sensor.ID == sensorID {
				removed = true
				continue
			}
			newSensors = append(newSensors, sensor)
		}
		s.sensors = newSensors
		delete(s.sensorStates, sensorID)

		if removed {
			log.Info().Str("sensor_id", sensorID.String()).Msg("Removed sensor from simulation")
		}
	}
}

// Stop stops the simulation
func (s *Simulator) Stop() {
	close(s.stopChan)
}

// generateReading generates a realistic sensor reading
func (s *Simulator) generateReading(sensor models.Sensor) (models.SensorReading, bool) {
	now := time.Now()
	baseValue, unit, minValue, maxValue := s.baseSignal(sensor, now)
	state := s.getSensorState(sensor)

	// Stateful autocorrelation so values evolve over time instead of fully independent random jumps.
	alpha := 0.18 + s.randFloat()*0.08
	candidate := state.LastValue*(1-alpha) + baseValue*alpha + s.randn()*state.Volatility

	controls := s.getRuntimeControls()
	candidate = s.applyScenarioModifiers(sensor, candidate, controls.scenario, now)
	candidate = s.applySimulationBehavior(sensor, candidate, controls.simulationConfig, state, now)

	if controls.simulationConfig != nil && controls.simulationConfig.ChaosMode != nil && controls.simulationConfig.ChaosMode.Enabled {
		chaos := controls.simulationConfig.ChaosMode
		if s.randFloat()*100 < chaos.FailureRate {
			return models.SensorReading{}, false
		}
		if s.randFloat()*100 < chaos.DataCorruptionRate {
			candidate = candidate * (0.4 + s.randFloat()*1.8)
		}
	}

	value := clamp(candidate, minValue, maxValue)
	state.LastValue = value
	state.LastTimestamp = now

	readingTime := now
	if controls.simulationConfig != nil && controls.simulationConfig.ChaosMode != nil && controls.simulationConfig.ChaosMode.Enabled {
		chaos := controls.simulationConfig.ChaosMode
		if chaos.NetworkLagMax > 0 {
			minLag := chaos.NetworkLagMin
			maxLag := chaos.NetworkLagMax
			if maxLag < minLag {
				maxLag = minLag
			}
			lagMs := minLag + int(s.randFloat()*float64(maxLag-minLag+1))
			readingTime = readingTime.Add(-time.Duration(lagMs) * time.Millisecond)
		}
	}

	return models.SensorReading{
		SensorID:   sensor.ID,
		SensorType: sensor.Type,
		Value:      math.Round(value*100) / 100,
		Unit:       unit,
		Latitude:   sensor.Latitude,
		Longitude:  sensor.Longitude,
		Timestamp:  readingTime,
	}, true
}

type sensorState struct {
	LastValue     float64
	LastTimestamp time.Time
	Volatility    float64
}

type runtimeControls struct {
	scenario         *scenario.Scenario
	simulationConfig *simulation.SimulationConfig
}

func (s *Simulator) syncRuntimeControls(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			controls := runtimeControls{
				scenario:         scenario.GetNormalScenario(),
				simulationConfig: simulation.NewSimulationConfig(),
			}

			configJSON, err := s.redis.Get(ctx, "simulation:config").Result()
			if err == nil && configJSON != "" {
				if cfg, parseErr := simulation.FromJSON(configJSON); parseErr == nil {
					controls.simulationConfig = cfg
				}
			}

			scenarioJSON, err := s.redis.Get(ctx, "scenario:active").Result()
			if err == nil && scenarioJSON != "" {
				if scn, parseErr := scenario.FromJSON(scenarioJSON); parseErr == nil {
					controls.scenario = scn
				}
			}
			if err != nil && err != rds.Nil {
				log.Debug().Err(err).Msg("Failed to fetch runtime controls from Redis")
			}

			s.mu.Lock()
			s.controls = controls
			s.mu.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

func (s *Simulator) getRuntimeControls() runtimeControls {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.controls
}

func (s *Simulator) getSensorState(sensor models.Sensor) *sensorState {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.sensorStates[sensor.ID]
	if !ok {
		state = s.newSensorState(sensor)
		s.sensorStates[sensor.ID] = state
	}
	return state
}

func (s *Simulator) newSensorState(sensor models.Sensor) *sensorState {
	value, _, minValue, maxValue := s.baseSignal(sensor, time.Now())
	if value < minValue || value > maxValue {
		value = (minValue + maxValue) / 2
	}
	return &sensorState{
		LastValue:     value + s.randn()*0.5,
		LastTimestamp: time.Now(),
		Volatility:    s.defaultVolatility(sensor.Type),
	}
}

func (s *Simulator) defaultVolatility(sensorType models.SensorType) float64 {
	switch sensorType {
	case models.SensorTypeTemperature:
		return 0.45
	case models.SensorTypePollution:
		return 2.8
	case models.SensorTypeHumidity:
		return 1.1
	case models.SensorTypeNoise:
		return 1.9
	default:
		return 1.0
	}
}

func (s *Simulator) baseSignal(sensor models.Sensor, now time.Time) (value float64, unit string, minValue float64, maxValue float64) {
	hour := float64(now.Hour()) + float64(now.Minute())/60.0
	rushHour := isRushHour(now)

	switch sensor.Type {
	case models.SensorTypeTemperature:
		base := 22.0
		daily := 0.0
		if s.config.Simulator.EnableWeatherPatterns {
			// Warmest in late afternoon.
			daily = 11.0 * math.Sin(2*math.Pi*(hour-8)/24.0)
		}
		value = base + daily + s.randn()*1.4
		return value, "celsius", -15, 46
	case models.SensorTypePollution:
		base := 58.0
		traffic := 0.0
		if s.config.Simulator.EnableRushHour && rushHour {
			traffic = 40.0
		}
		value = base + traffic + s.randn()*8.0
		return value, "µg/m³", 5, 280
	case models.SensorTypeHumidity:
		base := 58.0
		daily := 0.0
		if s.config.Simulator.EnableWeatherPatterns {
			// Typically more humid overnight/morning.
			daily = -18.0 * math.Sin(2*math.Pi*(hour-6)/24.0)
		}
		value = base + daily + s.randn()*4.0
		return value, "%", 10, 98
	case models.SensorTypeNoise:
		base := 52.0
		day := 0.0
		if hour >= 6 && hour <= 22 {
			day = 14.0
		}
		traffic := 0.0
		if s.config.Simulator.EnableRushHour && rushHour {
			traffic = 10.0
		}
		value = base + day + traffic + s.randn()*3.5
		return value, "dB", 35, 110
	default:
		return 0, "unknown", -1e9, 1e9
	}
}

func (s *Simulator) applyScenarioModifiers(sensor models.Sensor, value float64, scn *scenario.Scenario, now time.Time) float64 {
	if scn == nil || len(scn.Modifiers) == 0 {
		return value
	}

	progress := 1.0
	if scn.Duration > 0 && !scn.StartedAt.IsZero() {
		elapsed := now.Sub(scn.StartedAt).Seconds()
		total := scn.Duration.Seconds()
		if total > 0 {
			progress = clamp(elapsed/total, 0, 1)
		}
	}

	result := value
	for _, modifier := range scn.Modifiers {
		if modifier == nil || modifier.SensorType != string(sensor.Type) {
			continue
		}
		// We currently don't have explicit zone metadata on backend sensor records.
		if modifier.ZoneSpecific != "" {
			continue
		}
		result = scenario.ApplyModifier(result, modifier, progress)
	}
	return result
}

func (s *Simulator) applySimulationBehavior(sensor models.Sensor, value float64, cfg *simulation.SimulationConfig, state *sensorState, now time.Time) float64 {
	if cfg == nil {
		return value
	}

	behavior, ok := cfg.SensorBehaviors[sensor.ID.String()]
	if !ok || behavior == nil || !behavior.Enabled {
		return value
	}

	if behavior.UpdateRate > 0 && !state.LastTimestamp.IsZero() {
		if now.Sub(state.LastTimestamp) < time.Duration(behavior.UpdateRate)*time.Second {
			return state.LastValue
		}
	}

	result := value
	switch behavior.Pattern {
	case simulation.PatternSteady:
		result = state.LastValue + s.randn()*(behavior.Variance/100.0)
	case simulation.PatternSineWave:
		period := 20.0
		phase := float64(now.Unix()%int64(period*60)) / (period * 60)
		amp := (behavior.MaxValue - behavior.MinValue) / 2
		mid := (behavior.MaxValue + behavior.MinValue) / 2
		result = mid + amp*math.Sin(2*math.Pi*phase) + s.randn()*(behavior.Variance/80.0)
	case simulation.PatternRandomSpike:
		result = value
		if s.randFloat() < 0.05 {
			result = result + (behavior.MaxValue-behavior.MinValue)*(0.3+s.randFloat()*0.7)
		}
	case simulation.PatternLinear:
		drift := (behavior.MaxValue - behavior.MinValue) / 120.0
		result = state.LastValue + drift + s.randn()*(behavior.Variance/120.0)
	case simulation.PatternChaotic:
		result = value + s.randn()*(behavior.Variance/20.0)
	}

	if behavior.MinValue != 0 || behavior.MaxValue != 0 {
		result = clamp(result, behavior.MinValue, behavior.MaxValue)
	}
	return result
}

func isRushHour(now time.Time) bool {
	hour := now.Hour()
	return (hour >= 7 && hour <= 10) || (hour >= 16 && hour <= 20)
}

func clamp(v, minV, maxV float64) float64 {
	return math.Max(minV, math.Min(maxV, v))
}

func (s *Simulator) randFloat() float64 {
	s.randMu.Lock()
	defer s.randMu.Unlock()
	return s.rand.Float64()
}

func (s *Simulator) randn() float64 {
	// Approximate normal noise via Box-Muller transform.
	u1 := math.Max(1e-9, s.randFloat())
	u2 := s.randFloat()
	return math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
}
