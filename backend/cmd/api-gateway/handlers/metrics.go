package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/chetansierra/smart-city-monitor/internal/postgres"
	"github.com/chetansierra/smart-city-monitor/internal/redis"
	"github.com/chetansierra/smart-city-monitor/internal/sse"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// MetricsHandler handles metrics endpoints for Kafka, Redis, and system health
type MetricsHandler struct {
	redisClient  *redis.Client
	db           *postgres.DB
	broadcaster  *sse.Broadcaster
	kafkaBrokers []string
	startedAt    time.Time
	cacheMu      sync.RWMutex
	cachedStats  NerdStatsResponse
	cacheReady   bool
	lastStatsSig string
	sampleMu     sync.Mutex
	lastSampleAt time.Time
	lastKafkaMsg int64
	lastRedisCmd int64
	kafkaMu      sync.Mutex
	kafkaAdmin   sarama.ClusterAdmin
	kafkaClient  sarama.Client
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(db *postgres.DB, redisClient *redis.Client, broadcaster *sse.Broadcaster, kafkaBrokers []string) *MetricsHandler {
	return &MetricsHandler{
		redisClient:  redisClient,
		db:           db,
		broadcaster:  broadcaster,
		kafkaBrokers: kafkaBrokers,
		startedAt:    time.Now().UTC(),
	}
}

// KafkaMetricsResponse represents Kafka metrics
type KafkaMetricsResponse struct {
	Topics          []TopicMetrics         `json:"topics"`
	ConsumerGroups  []ConsumerGroupMetrics `json:"consumer_groups"`
	BrokerCount     int                    `json:"broker_count"`
	TotalPartitions int                    `json:"total_partitions"`
	Timestamp       string                 `json:"timestamp"`
}

// TopicMetrics represents metrics for a Kafka topic
type TopicMetrics struct {
	Name              string  `json:"name"`
	Partitions        int     `json:"partitions"`
	ReplicationFactor int     `json:"replication_factor"`
	MessagesPerSec    float64 `json:"messages_per_second"`
}

// ConsumerGroupMetrics represents metrics for a consumer group
type ConsumerGroupMetrics struct {
	GroupID   string     `json:"group_id"`
	State     string     `json:"state"`
	Members   int        `json:"members"`
	TopicLags []TopicLag `json:"topic_lags"`
	TotalLag  int64      `json:"total_lag"`
}

// TopicLag represents lag information for a topic
type TopicLag struct {
	Topic      string         `json:"topic"`
	Partitions []PartitionLag `json:"partitions"`
	TotalLag   int64          `json:"total_lag"`
}

// PartitionLag represents lag for a partition
type PartitionLag struct {
	Partition     int32 `json:"partition"`
	CurrentOffset int64 `json:"current_offset"`
	LogEndOffset  int64 `json:"log_end_offset"`
	Lag           int64 `json:"lag"`
}

// RedisMetricsResponse represents Redis metrics
type RedisMetricsResponse struct {
	Memory            RedisMemoryStats `json:"memory"`
	Clients           RedisClientStats `json:"clients"`
	Stats             RedisStats       `json:"stats"`
	Persistence       RedisPersistence `json:"persistence"`
	KeyspaceStats     map[string]int64 `json:"keyspace_stats"`
	CommandsPerSecond float64          `json:"commands_per_second"`
	HitRate           float64          `json:"hit_rate"`
	Timestamp         string           `json:"timestamp"`
}

// RedisMemoryStats represents Redis memory statistics
type RedisMemoryStats struct {
	UsedMemory          int64   `json:"used_memory_bytes"`
	UsedMemoryHuman     string  `json:"used_memory_human"`
	UsedMemoryPeak      int64   `json:"used_memory_peak_bytes"`
	UsedMemoryPeakHuman string  `json:"used_memory_peak_human"`
	TotalSystemMemory   int64   `json:"total_system_memory_bytes"`
	MemoryFragmentation float64 `json:"memory_fragmentation_ratio"`
}

// RedisClientStats represents Redis client statistics
type RedisClientStats struct {
	ConnectedClients int `json:"connected_clients"`
	BlockedClients   int `json:"blocked_clients"`
	MaxClients       int `json:"max_clients"`
}

// RedisStats represents Redis operation statistics
type RedisStats struct {
	TotalConnections int64 `json:"total_connections_received"`
	TotalCommands    int64 `json:"total_commands_processed"`
	KeyspaceHits     int64 `json:"keyspace_hits"`
	KeyspaceMisses   int64 `json:"keyspace_misses"`
	ExpiredKeys      int64 `json:"expired_keys"`
	EvictedKeys      int64 `json:"evicted_keys"`
}

// RedisPersistence represents Redis persistence information
type RedisPersistence struct {
	RDBLastSaveTime         int64 `json:"rdb_last_save_time"`
	RDBChangesSinceLastSave int64 `json:"rdb_changes_since_last_save"`
}

// SystemHealthResponse represents overall system health
type SystemHealthResponse struct {
	Services      map[string]ServiceHealth `json:"services"`
	DatabasePool  DatabasePoolStats        `json:"database_pool"`
	SSEStats      SSEStats                 `json:"sse_stats"`
	OverallStatus string                   `json:"overall_status"`
	Timestamp     string                   `json:"timestamp"`
}

// ServiceHealth represents health status of a service
type ServiceHealth struct {
	Status  string  `json:"status"`
	Latency float64 `json:"latency_ms"`
	Message string  `json:"message,omitempty"`
}

// DatabasePoolStats represents database connection pool statistics
type DatabasePoolStats struct {
	MaxConnections  int `json:"max_connections"`
	OpenConnections int `json:"open_connections"`
	InUse           int `json:"in_use"`
	Idle            int `json:"idle"`
}

// SSEStats represents SSE stream statistics
type SSEStats struct {
	ActiveConnections int   `json:"active_connections"`
	TotalConnections  int64 `json:"total_connections"`
	EventsSent        int64 `json:"events_sent"`
}

// NerdStatsResponse represents deep operational stats for advanced users.
type NerdStatsResponse struct {
	System struct {
		UptimeSec     int64   `json:"uptime_sec"`
		Goroutines    int     `json:"goroutines"`
		HeapAllocMB   float64 `json:"heap_alloc_mb"`
		HeapSysMB     float64 `json:"heap_sys_mb"`
		GCCyclesTotal uint32  `json:"gc_cycles_total"`
	} `json:"system"`
	Database struct {
		Sensors             int64 `json:"sensors"`
		TotalSensorsCreated int64 `json:"total_sensors_created"`
		Readings            int64 `json:"readings"`
	} `json:"database"`
	Redis struct {
		KeyCount         int64   `json:"key_count"`
		PingLatencyMS    float64 `json:"ping_latency_ms"`
		ConnectedClients int64   `json:"connected_clients"`
		TotalCommands    int64   `json:"total_commands_processed"`
	} `json:"redis"`
	Kafka struct {
		BrokerCount     int   `json:"broker_count"`
		TopicCount      int   `json:"topic_count"`
		TotalPartitions int   `json:"total_partitions"`
		TotalMessages   int64 `json:"total_messages"`
	} `json:"kafka"`
	SSE struct {
		ActiveClients int64 `json:"active_clients"`
	} `json:"sse"`
	Sessions struct {
		TotalVisitors int64 `json:"total_visitors"`
		ActiveNow     int64 `json:"active_now"`
	} `json:"sessions"`
	Performance struct {
		EstimatedThroughput float64 `json:"estimated_throughput_msg_sec"`
	} `json:"performance"`
	Platform struct {
		SessionsWithSensors  int64   `json:"sessions_with_sensors"`
		AvgReadingsPerSensor float64 `json:"avg_readings_per_sensor"`
	} `json:"platform"`
	Timestamp string `json:"timestamp"`
}

type SessionNerdStatsResponse struct {
	Session struct {
		SessionID       string  `json:"session_id"`
		Sensors         int64   `json:"sensors"`
		Readings        int64   `json:"readings"`
		ReadingsLastMin int64   `json:"readings_last_min"`
		AvgSensorValue  float64 `json:"avg_sensor_value"`
		LastReadingAt   string  `json:"last_reading_at,omitempty"`
	} `json:"session"`
	Realtime struct {
		EstimatedThroughput float64 `json:"estimated_throughput_msg_sec"`
		SSEConnected        bool    `json:"sse_connected"`
	} `json:"realtime"`
	Timestamp string `json:"timestamp"`
}

// getOrCreateKafkaAdmin returns a cached or new Kafka ClusterAdmin.
func (h *MetricsHandler) getOrCreateKafkaAdmin() (sarama.ClusterAdmin, error) {
	h.kafkaMu.Lock()
	defer h.kafkaMu.Unlock()

	if h.kafkaAdmin != nil {
		// Quick health check — if broken, recreate
		if _, _, err := h.kafkaAdmin.DescribeCluster(); err == nil {
			return h.kafkaAdmin, nil
		}
		h.kafkaAdmin.Close()
		h.kafkaAdmin = nil
	}

	cfg := sarama.NewConfig()
	cfg.Version = sarama.V2_6_0_0
	cfg.Admin.Timeout = 5 * time.Second

	admin, err := sarama.NewClusterAdmin(h.kafkaBrokers, cfg)
	if err != nil {
		return nil, err
	}
	h.kafkaAdmin = admin
	return admin, nil
}

// getOrCreateKafkaClient returns a cached or new Kafka Client.
func (h *MetricsHandler) getOrCreateKafkaClient() (sarama.Client, error) {
	h.kafkaMu.Lock()
	defer h.kafkaMu.Unlock()

	if h.kafkaClient != nil && !h.kafkaClient.Closed() {
		return h.kafkaClient, nil
	}

	cfg := sarama.NewConfig()
	cfg.Version = sarama.V2_6_0_0

	client, err := sarama.NewClient(h.kafkaBrokers, cfg)
	if err != nil {
		return nil, err
	}
	h.kafkaClient = client
	return client, nil
}

// GetKafkaMetrics returns Kafka metrics
func (h *MetricsHandler) GetKafkaMetrics(c *fiber.Ctx) error {
	ctx := context.Background()

	admin, err := h.getOrCreateKafkaAdmin()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Kafka admin client")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    "KAFKA_CONNECTION_ERROR",
				"message": "Failed to connect to Kafka",
			},
		})
	}

	// Get broker information
	brokers, _, err := admin.DescribeCluster()
	if err != nil {
		log.Error().Err(err).Msg("Failed to describe Kafka cluster")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    "KAFKA_DESCRIBE_ERROR",
				"message": "Failed to describe Kafka cluster",
			},
		})
	}

	// Get topic metadata - first list all topics
	topicsMap, err := admin.ListTopics()
	if err != nil {
		log.Error().Err(err).Msg("Failed to list Kafka topics")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    "KAFKA_TOPICS_ERROR",
				"message": "Failed to list Kafka topics",
			},
		})
	}

	// Get detailed metadata for topics
	topicNames := make([]string, 0, len(topicsMap))
	for topicName := range topicsMap {
		topicNames = append(topicNames, topicName)
	}

	// Use cached client for partition metadata
	kafkaClient, clientErr := h.getOrCreateKafkaClient()

	// Build topic metrics
	var topicMetrics []TopicMetrics
	totalPartitions := 0

	for _, topicName := range topicNames {
		topicDetail := topicsMap[topicName]
		replicationFactor := int(topicDetail.ReplicationFactor)
		if replicationFactor == -1 {
			replicationFactor = 1
		}

		partitionCount := len(topicDetail.ReplicaAssignment)
		if kafkaClient != nil && clientErr == nil {
			if partitions, pErr := kafkaClient.Partitions(topicName); pErr == nil {
				partitionCount = len(partitions)
			}
		}

		totalPartitions += partitionCount
		topicMetrics = append(topicMetrics, TopicMetrics{
			Name:              topicName,
			Partitions:        partitionCount,
			ReplicationFactor: replicationFactor,
			MessagesPerSec:    0,
		})
	}

	// Get consumer group information
	consumerGroups, err := admin.ListConsumerGroups()
	if err != nil {
		log.Error().Err(err).Msg("Failed to list consumer groups")
	}

	var consumerGroupMetrics []ConsumerGroupMetrics

	if consumerGroups != nil {
		for groupID := range consumerGroups {
			groupMetrics, err := h.getConsumerGroupMetrics(admin, groupID, ctx)
			if err != nil {
				log.Error().Err(err).Str("group_id", groupID).Msg("Failed to get consumer group metrics")
				continue
			}
			consumerGroupMetrics = append(consumerGroupMetrics, groupMetrics)
		}
	}

	response := KafkaMetricsResponse{
		Topics:          topicMetrics,
		ConsumerGroups:  consumerGroupMetrics,
		BrokerCount:     len(brokers),
		TotalPartitions: totalPartitions,
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    response,
	})
}

// getConsumerGroupMetrics fetches metrics for a specific consumer group
func (h *MetricsHandler) getConsumerGroupMetrics(admin sarama.ClusterAdmin, groupID string, ctx context.Context) (ConsumerGroupMetrics, error) {
	// Describe consumer group
	groupDescriptions, err := admin.DescribeConsumerGroups([]string{groupID})
	if err != nil || len(groupDescriptions) == 0 {
		return ConsumerGroupMetrics{}, fmt.Errorf("failed to describe consumer group: %w", err)
	}

	groupDesc := groupDescriptions[0]

	// Get topic offsets
	topicLags := []TopicLag{}
	totalLag := int64(0)

	// List offsets for each topic this consumer group is subscribed to
	groupOffsets, err := admin.ListConsumerGroupOffsets(groupID, nil)
	if err == nil {
		kafkaClient, clientErr := h.getOrCreateKafkaClient()
		for topic, partitions := range groupOffsets.Blocks {
			topicLag := TopicLag{
				Topic:      topic,
				Partitions: []PartitionLag{},
				TotalLag:   0,
			}

			for partition, offsetBlock := range partitions {
				if kafkaClient == nil || clientErr != nil {
					continue
				}
				hwm, hwmErr := kafkaClient.GetOffset(topic, partition, sarama.OffsetNewest)
				if hwmErr != nil {
					continue
				}
				lag := hwm - offsetBlock.Offset
				if lag < 0 {
					lag = 0
				}

				topicLag.Partitions = append(topicLag.Partitions, PartitionLag{
					Partition:     partition,
					CurrentOffset: offsetBlock.Offset,
					LogEndOffset:  hwm,
					Lag:           lag,
				})
				topicLag.TotalLag += lag
				totalLag += lag
			}

			if len(topicLag.Partitions) > 0 {
				topicLags = append(topicLags, topicLag)
			}
		}
	}

	return ConsumerGroupMetrics{
		GroupID:   groupID,
		State:     groupDesc.State,
		Members:   len(groupDesc.Members),
		TopicLags: topicLags,
		TotalLag:  totalLag,
	}, nil
}

// GetRedisMetrics returns Redis metrics
func (h *MetricsHandler) GetRedisMetrics(c *fiber.Ctx) error {
	ctx := context.Background()

	// Get Redis INFO
	info, err := h.redisClient.Client.Info(ctx, "all").Result()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get Redis INFO")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    "REDIS_INFO_ERROR",
				"message": "Failed to fetch Redis metrics",
			},
		})
	}

	// Parse INFO output
	metrics := h.parseRedisInfo(info)

	// Get keyspace statistics
	keyspaceStats := make(map[string]int64)

	// Count keys by pattern
	patterns := []string{
		"sensor:latest:*",
		"sensor:*",
		"city:stats",
		"sensors:geo",
	}

	for _, pattern := range patterns {
		keys, err := h.redisClient.Client.Keys(ctx, pattern).Result()
		if err == nil {
			keyspaceStats[pattern] = int64(len(keys))
		}
	}

	metrics.KeyspaceStats = keyspaceStats
	metrics.Timestamp = time.Now().UTC().Format(time.RFC3339)

	return c.JSON(fiber.Map{
		"success": true,
		"data":    metrics,
	})
}

// parseRedisInfo parses Redis INFO command output
func (h *MetricsHandler) parseRedisInfo(info string) RedisMetricsResponse {
	metrics := RedisMetricsResponse{
		Memory:        RedisMemoryStats{},
		Clients:       RedisClientStats{},
		Stats:         RedisStats{},
		Persistence:   RedisPersistence{},
		KeyspaceStats: make(map[string]int64),
	}

	// Parse INFO string (simple parsing - can be enhanced)
	// This is a simplified parser - in production, use a more robust solution

	ctx := context.Background()

	// Get memory stats
	usedMemory, _ := h.redisClient.Client.Info(ctx, "memory").Result()
	_ = usedMemory // Parse this properly in production

	// Get client stats
	clientList, err := h.redisClient.Client.ClientList(ctx).Result()
	if err == nil {
		// Count connected clients from CLIENT LIST output
		metrics.Clients.ConnectedClients = len(clientList) / 100 // Rough estimate
	}

	// Get stats
	stats, _ := h.redisClient.Client.Info(ctx, "stats").Result()
	_ = stats // Parse this properly in production

	// Calculate hit rate
	if metrics.Stats.KeyspaceHits > 0 || metrics.Stats.KeyspaceMisses > 0 {
		total := float64(metrics.Stats.KeyspaceHits + metrics.Stats.KeyspaceMisses)
		metrics.HitRate = float64(metrics.Stats.KeyspaceHits) / total * 100
	}

	// Estimate commands per second (would need time-series data in production)
	metrics.CommandsPerSecond = 0

	return metrics
}

// GetSystemHealth returns overall system health
func (h *MetricsHandler) GetSystemHealth(c *fiber.Ctx) error {
	ctx := context.Background()

	services := make(map[string]ServiceHealth)

	// Check Redis health
	redisStart := time.Now()
	if err := h.redisClient.Client.Ping(ctx).Err(); err != nil {
		services["redis"] = ServiceHealth{
			Status:  "unhealthy",
			Latency: float64(time.Since(redisStart).Milliseconds()),
			Message: err.Error(),
		}
	} else {
		services["redis"] = ServiceHealth{
			Status:  "healthy",
			Latency: float64(time.Since(redisStart).Milliseconds()),
		}
	}

	// Check Kafka health
	kafkaStart := time.Now()
	config := sarama.NewConfig()
	config.Version = sarama.V2_6_0_0
	config.Admin.Timeout = 5 * time.Second

	admin, err := sarama.NewClusterAdmin(h.kafkaBrokers, config)
	if err != nil {
		services["kafka"] = ServiceHealth{
			Status:  "unhealthy",
			Latency: float64(time.Since(kafkaStart).Milliseconds()),
			Message: err.Error(),
		}
	} else {
		_, _, err := admin.DescribeCluster()
		admin.Close()

		if err != nil {
			services["kafka"] = ServiceHealth{
				Status:  "unhealthy",
				Latency: float64(time.Since(kafkaStart).Milliseconds()),
				Message: err.Error(),
			}
		} else {
			services["kafka"] = ServiceHealth{
				Status:  "healthy",
				Latency: float64(time.Since(kafkaStart).Milliseconds()),
			}
		}
	}

	// Determine overall status
	overallStatus := "healthy"
	for _, service := range services {
		if service.Status != "healthy" {
			overallStatus = "degraded"
			break
		}
	}

	response := SystemHealthResponse{
		Services:      services,
		OverallStatus: overallStatus,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		DatabasePool: DatabasePoolStats{
			MaxConnections:  100, // TODO: Get from actual DB pool
			OpenConnections: 0,
			InUse:           0,
			Idle:            0,
		},
		SSEStats: SSEStats{
			ActiveConnections: 0, // TODO: Get from broadcaster
			TotalConnections:  0,
			EventsSent:        0,
		},
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    response,
	})
}

// GetNerdStats returns consolidated low-level service stats.
func (h *MetricsHandler) GetNerdStats(c *fiber.Ctx) error {
	h.cacheMu.RLock()
	cached := h.cachedStats
	ready := h.cacheReady
	h.cacheMu.RUnlock()

	if !ready {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		h.refreshRuntimeAndSessions(ctx)
		h.refreshRedisStats(ctx)
		h.refreshDeepStats(ctx)
		h.broadcastCachedStats(false)

		h.cacheMu.RLock()
		cached = h.cachedStats
		h.cacheMu.RUnlock()
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    cached,
	})
}

// GetSessionNerdStats returns stats scoped to the current session ID.
func (h *MetricsHandler) GetSessionNerdStats(c *fiber.Ctx) error {
	sessionID := strings.TrimSpace(c.Get("X-Session-ID"))
	if sessionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    "MISSING_SESSION_ID",
				"message": "X-Session-ID header is required",
			},
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sessionUUID, parseErr := uuid.Parse(sessionID)
	if parseErr != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    "INVALID_SESSION_ID",
				"message": "Invalid session ID format",
			},
		})
	}

	if hbErr := h.redisClient.SetSessionHeartbeat(ctx, sessionUUID); hbErr != nil {
		log.Warn().Err(hbErr).Str("session_id", sessionID).Msg("Failed to renew session heartbeat from stats endpoint")
	}

	resp := SessionNerdStatsResponse{}
	resp.Session.SessionID = sessionID
	resp.Timestamp = time.Now().UTC().Format(time.RFC3339)

	if err := h.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM sensors
		WHERE session_id = $1
	`, sessionUUID).Scan(&resp.Session.Sensors); err != nil {
		log.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to read session sensors count")
	}

	// Session reading stats from Redis counters (no sensor_readings table)
	if totalReadings, avgValue, lastReadingAt, err := h.redisClient.GetSessionReadingStats(ctx, sessionID); err == nil {
		resp.Session.Readings = totalReadings
		resp.Session.AvgSensorValue = avgValue
		if lastReadingAt != "" {
			resp.Session.LastReadingAt = lastReadingAt
		}
	} else {
		log.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to read session reading stats from Redis")
	}

	// Actual readings in the last 60 seconds from Redis sliding window
	if readingsLastMin, err := h.redisClient.GetSessionReadingsLastMinute(ctx, sessionID); err == nil {
		resp.Session.ReadingsLastMin = readingsLastMin
	} else {
		log.Warn().Err(err).Str("session_id", sessionID).Msg("Failed to get session readings rate")
	}

	resp.Realtime.EstimatedThroughput = float64(resp.Session.ReadingsLastMin) / 60.0

	if h.broadcaster != nil {
		resp.Realtime.SSEConnected = h.broadcaster.HasSession(sessionID)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    resp,
	})
}

// StartNerdStatsAggregator maintains a cached stats snapshot and pushes live deltas over SSE.
func (h *MetricsHandler) StartNerdStatsAggregator(ctx context.Context) {
	fastTicker := time.NewTicker(10 * time.Second)
	redisTicker := time.NewTicker(10 * time.Second)
	deepTicker := time.NewTicker(15 * time.Second)
	heartbeatTicker := time.NewTicker(15 * time.Second)
	defer fastTicker.Stop()
	defer redisTicker.Stop()
	defer deepTicker.Stop()
	defer heartbeatTicker.Stop()

	// Warm first snapshot so first UI load is fast.
	h.refreshRuntimeAndSessions(ctx)
	h.refreshRedisStats(ctx)
	h.refreshDeepStats(ctx)
	h.broadcastCachedStats(true)

	for {
		select {
		case <-ctx.Done():
			return
		case <-fastTicker.C:
			h.refreshRuntimeAndSessions(ctx)
			h.broadcastCachedStats(false)
		case <-redisTicker.C:
			h.refreshRedisStats(ctx)
			h.broadcastCachedStats(false)
		case <-deepTicker.C:
			h.refreshDeepStats(ctx)
			h.broadcastCachedStats(false)
		case <-heartbeatTicker.C:
			h.broadcastCachedStats(true)
		}
	}
}

func (h *MetricsHandler) refreshRuntimeAndSessions(ctx context.Context) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	var totalVisitors int64
	if visitors, err := h.redisClient.GetTotalVisitors(ctx); err == nil {
		totalVisitors = visitors
	}

	var activeNow int64
	if activeSessions, err := h.redisClient.GetActiveSessions(ctx); err == nil {
		activeNow = int64(len(activeSessions))
	}

	var activeClients int64
	if h.broadcaster != nil {
		stats := h.broadcaster.GetStats()
		if totalClients, ok := stats["total_clients"].(int); ok {
			activeClients = int64(totalClients)
		}
	}

	h.cacheMu.Lock()
	s := h.cachedStats
	s.System.UptimeSec = int64(time.Since(h.startedAt).Seconds())
	s.System.Goroutines = runtime.NumGoroutine()
	s.System.HeapAllocMB = float64(mem.HeapAlloc) / (1024 * 1024)
	s.System.HeapSysMB = float64(mem.HeapSys) / (1024 * 1024)
	s.System.GCCyclesTotal = mem.NumGC
	s.Sessions.TotalVisitors = totalVisitors
	s.Sessions.ActiveNow = activeNow
	s.SSE.ActiveClients = activeClients
	s.Timestamp = time.Now().UTC().Format(time.RFC3339)
	h.cachedStats = s
	h.cacheReady = true
	h.cacheMu.Unlock()
}

func (h *MetricsHandler) refreshRedisStats(ctx context.Context) {
	var keyCount int64
	if dbSize, err := h.redisClient.Client.DBSize(ctx).Result(); err == nil {
		keyCount = dbSize
	}

	var pingLatencyMS float64
	redisStart := time.Now()
	if err := h.redisClient.Client.Ping(ctx).Err(); err == nil {
		pingLatencyMS = float64(time.Since(redisStart).Microseconds()) / 1000.0
	}

	var connectedClients int64
	if infoClients, err := h.redisClient.Client.Info(ctx, "clients").Result(); err == nil {
		connectedClients = parseRedisInfoInt64(infoClients, "connected_clients")
	}

	var totalCommands int64
	if infoStats, err := h.redisClient.Client.Info(ctx, "stats").Result(); err == nil {
		totalCommands = parseRedisInfoInt64(infoStats, "total_commands_processed")
	}

	h.cacheMu.Lock()
	s := h.cachedStats
	s.Redis.KeyCount = keyCount
	s.Redis.PingLatencyMS = pingLatencyMS
	s.Redis.ConnectedClients = connectedClients
	s.Redis.TotalCommands = totalCommands
	s.Timestamp = time.Now().UTC().Format(time.RFC3339)
	// Don't call recomputeThroughput here — Kafka.TotalMessages only updates
	// in refreshDeepStats, so calling it here would zero out throughput.
	h.cachedStats = s
	h.cacheReady = true
	h.cacheMu.Unlock()
}

func (h *MetricsHandler) refreshDeepStats(ctx context.Context) {
	var sensors int64
	if err := h.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sensors").Scan(&sensors); err != nil {
		log.Warn().Err(err).Msg("Failed to read sensors count for nerd stats")
	}

	// Total readings from Redis counter (no sensor_readings table)
	var readings int64
	if count, err := h.redisClient.GetTotalReadings(ctx); err == nil {
		readings = count
	} else {
		log.Warn().Err(err).Msg("Failed to read total readings count from Redis")
	}

	// Total distinct sensors ever created from Redis HyperLogLog
	var totalSensorsCreated int64
	if count, err := h.redisClient.GetDistinctSensorCount(ctx); err == nil {
		totalSensorsCreated = count
	} else {
		log.Warn().Err(err).Msg("Failed to count total sensors ever created from Redis")
	}
	// Active sensors is always >= total ever created (sensors that still exist)
	if totalSensorsCreated < sensors {
		totalSensorsCreated = sensors
	}

	var sessionsWithSensors int64
	if err := h.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT session_id)
		FROM sensors
		WHERE session_id IS NOT NULL
	`).Scan(&sessionsWithSensors); err != nil {
		log.Warn().Err(err).Msg("Failed to read sessions_with_sensors for nerd stats")
	}

	avgReadingsPerSensor := 0.0
	if sensors > 0 {
		avgReadingsPerSensor = float64(readings) / float64(sensors)
	}

	kafkaBrokerCount := 0
	kafkaTopicCount := 0
	kafkaTotalPartitions := 0
	kafkaTotalMessages := int64(0)

	if admin, adminErr := h.getOrCreateKafkaAdmin(); adminErr == nil {
		if brokers, _, describeErr := admin.DescribeCluster(); describeErr == nil {
			kafkaBrokerCount = len(brokers)
		}
		if topicsMap, listErr := admin.ListTopics(); listErr == nil {
			for name, topic := range topicsMap {
				if strings.HasPrefix(name, "__") {
					continue
				}
				kafkaTopicCount++
				kafkaTotalPartitions += len(topic.ReplicaAssignment)
			}
		}
	}

	if kafkaClient, clientErr := h.getOrCreateKafkaClient(); clientErr == nil {
		if topics, topicErr := kafkaClient.Topics(); topicErr == nil {
			for _, topic := range topics {
				if strings.HasPrefix(topic, "__") {
					continue
				}
				partitions, partErr := kafkaClient.Partitions(topic)
				if partErr != nil {
					continue
				}
				for _, partition := range partitions {
					oldest, oldestErr := kafkaClient.GetOffset(topic, partition, sarama.OffsetOldest)
					newest, newestErr := kafkaClient.GetOffset(topic, partition, sarama.OffsetNewest)
					if oldestErr == nil && newestErr == nil && newest >= oldest {
						kafkaTotalMessages += (newest - oldest)
					}
				}
			}
		}
	}

	h.cacheMu.Lock()
	s := h.cachedStats
	s.Database.Sensors = sensors
	s.Database.TotalSensorsCreated = totalSensorsCreated
	s.Database.Readings = readings
	s.Platform.SessionsWithSensors = sessionsWithSensors
	s.Platform.AvgReadingsPerSensor = avgReadingsPerSensor
	s.Kafka.BrokerCount = kafkaBrokerCount
	s.Kafka.TopicCount = kafkaTopicCount
	s.Kafka.TotalPartitions = kafkaTotalPartitions
	s.Kafka.TotalMessages = kafkaTotalMessages
	s.Timestamp = time.Now().UTC().Format(time.RFC3339)
	h.recomputeThroughput(&s)
	h.cachedStats = s
	h.cacheReady = true
	h.cacheMu.Unlock()
}

func (h *MetricsHandler) GetCachedStats() (NerdStatsResponse, bool) {
	h.cacheMu.RLock()
	defer h.cacheMu.RUnlock()
	return h.cachedStats, h.cacheReady
}

func (h *MetricsHandler) recomputeThroughput(s *NerdStatsResponse) {
	now := time.Now()
	h.sampleMu.Lock()
	defer h.sampleMu.Unlock()

	if h.lastSampleAt.IsZero() {
		h.lastSampleAt = now
		h.lastKafkaMsg = s.Kafka.TotalMessages
		h.lastRedisCmd = s.Redis.TotalCommands
		return
	}

	elapsed := now.Sub(h.lastSampleAt).Seconds()
	// Ignore tiny windows to avoid zeroing throughput when refresh cycles overlap.
	if elapsed < 0.5 {
		return
	}

	kafkaDelta := float64(s.Kafka.TotalMessages - h.lastKafkaMsg)
	if kafkaDelta < 0 {
		kafkaDelta = 0
	}
	// Only use Kafka message delta — Redis commands include internal housekeeping
	// and don't reflect actual sensor data throughput.
	s.Performance.EstimatedThroughput = kafkaDelta / elapsed

	h.lastSampleAt = now
	h.lastKafkaMsg = s.Kafka.TotalMessages
	h.lastRedisCmd = s.Redis.TotalCommands
}

func (h *MetricsHandler) broadcastCachedStats(force bool) {
	if h.broadcaster == nil {
		return
	}
	h.cacheMu.RLock()
	if !h.cacheReady {
		h.cacheMu.RUnlock()
		return
	}
	snapshot := h.cachedStats
	signature := nerdStatsSignature(snapshot)
	lastSig := h.lastStatsSig
	h.cacheMu.RUnlock()

	if !force && signature == lastSig {
		return
	}

	h.cacheMu.Lock()
	h.lastStatsSig = signature
	h.cacheMu.Unlock()
	h.broadcaster.BroadcastStats(snapshot)
}

func parseRedisInfoInt64(info string, field string) int64 {
	prefix := field + ":"
	for _, line := range strings.Split(info, "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		raw := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return 0
		}
		return parsed
	}
	return 0
}

func nerdStatsSignature(stats NerdStatsResponse) string {
	stats.Timestamp = ""
	raw, err := json.Marshal(stats)
	if err != nil {
		return ""
	}
	return string(raw)
}
