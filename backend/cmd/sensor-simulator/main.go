package main

import (
	"context"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chetansierra/smart-city-monitor/internal/config"
	"github.com/chetansierra/smart-city-monitor/internal/kafka"
	"github.com/chetansierra/smart-city-monitor/internal/logger"
	"github.com/chetansierra/smart-city-monitor/internal/models"
	"github.com/chetansierra/smart-city-monitor/internal/postgres"
	"github.com/rs/zerolog/log"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
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

	// Create simulator
	simulator := NewSimulator(sensors, producer, cfg)

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
	sensors  []models.Sensor
	producer *kafka.Producer
	config   *config.Config
	stopChan chan bool
	rand     *rand.Rand
}

// NewSimulator creates a new simulator
func NewSimulator(sensors []models.Sensor, producer *kafka.Producer, cfg *config.Config) *Simulator {
	return &Simulator{
		sensors:  sensors,
		producer: producer,
		config:   cfg,
		stopChan: make(chan bool),
		rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Start begins the simulation
func (s *Simulator) Start() {
	log.Info().
		Int("sensor_count", len(s.sensors)).
		Int("interval_ms", s.config.Simulator.GenerationIntervalMS).
		Msg("Starting simulation")

	ticker := time.NewTicker(time.Duration(s.config.Simulator.GenerationIntervalMS) * time.Millisecond)
	defer ticker.Stop()

	messageCount := 0

	for {
		select {
		case <-ticker.C:
			// Generate readings for all sensors
			for _, sensor := range s.sensors {
				reading := s.generateReading(sensor)

				// Convert to Kafka message format
				kafkaMsg := models.KafkaMessage{
					SensorID:   sensor.ID.String(),
					SensorType: string(sensor.Type),
					Value:      reading.Value,
					Unit:       reading.Unit,
					Location: models.Location{
						Latitude:  sensor.Latitude,
						Longitude: sensor.Longitude,
					},
					Timestamp: reading.Timestamp.Format(time.RFC3339),
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

			if messageCount%100 == 0 {
				log.Debug().Int("messages_sent", messageCount).Msg("Kafka messages sent")
			}

		case <-s.stopChan:
			log.Info().Int("total_messages", messageCount).Msg("Simulation stopped")
			return
		}
	}
}

// Stop stops the simulation
func (s *Simulator) Stop() {
	close(s.stopChan)
}

// generateReading generates a realistic sensor reading
func (s *Simulator) generateReading(sensor models.Sensor) models.SensorReading {
	now := time.Now()
	hour := now.Hour()

	var value float64
	var unit string

	switch sensor.Type {
	case models.SensorTypeTemperature:
		// Temperature: 15-35°C, peaks in afternoon
		baseTemp := 20.0
		dailyVariation := 8.0 * math.Sin(2*math.Pi*(float64(hour)-6)/24.0) // Peak at 2 PM
		randomNoise := s.rand.Float64()*2.0 - 1.0                          // ±1°C
		value = baseTemp + dailyVariation + randomNoise
		unit = "celsius"

	case models.SensorTypePollution:
		// PM2.5: 10-150 µg/m³, higher during rush hours
		basePollution := 40.0
		rushHourFactor := 0.0
		if (hour >= 7 && hour <= 9) || (hour >= 17 && hour <= 19) {
			rushHourFactor = 40.0 // Rush hour spike
		}
		randomNoise := s.rand.Float64()*20.0 - 10.0 // ±10 µg/m³
		value = basePollution + rushHourFactor + randomNoise
		value = math.Max(10, value) // Minimum 10
		unit = "µg/m³"

	case models.SensorTypeHumidity:
		// Humidity: 30-80%
		baseHumidity := 55.0
		dailyVariation := 15.0 * math.Sin(2*math.Pi*(float64(hour)-3)/24.0) // Higher in morning
		randomNoise := s.rand.Float64()*10.0 - 5.0                          // ±5%
		value = baseHumidity + dailyVariation + randomNoise
		value = math.Max(30, math.Min(80, value)) // Clamp to 30-80%
		unit = "%"

	case models.SensorTypeNoise:
		// Noise: 40-90 dB, higher during day
		baseNoise := 55.0
		dayFactor := 0.0
		if hour >= 6 && hour <= 22 {
			dayFactor = 15.0 // Daytime is louder
		}
		rushHourFactor := 0.0
		if (hour >= 7 && hour <= 9) || (hour >= 17 && hour <= 19) {
			rushHourFactor = 10.0 // Rush hour spike
		}
		randomNoise := s.rand.Float64()*8.0 - 4.0 // ±4 dB
		value = baseNoise + dayFactor + rushHourFactor + randomNoise
		value = math.Max(40, math.Min(90, value)) // Clamp to 40-90 dB
		unit = "dB"

	default:
		value = 0
		unit = "unknown"
	}

	return models.SensorReading{
		SensorID:   sensor.ID,
		SensorType: sensor.Type,
		Value:      math.Round(value*100) / 100, // Round to 2 decimal places
		Unit:       unit,
		Latitude:   sensor.Latitude,
		Longitude:  sensor.Longitude,
		Timestamp:  now,
	}
}
