package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// LagMonitor tracks consumer group lag and publishes it to Redis.
type LagMonitor struct {
	brokers     []string
	groupID     string
	topics      []string
	redisClient *redis.Client
	interval    time.Duration

	mu  sync.RWMutex
	lag map[string]int64 // partition -> lag
}

// NewLagMonitor creates a new consumer lag monitor.
func NewLagMonitor(brokers []string, groupID string, topics []string, redisClient *redis.Client) *LagMonitor {
	return &LagMonitor{
		brokers:     brokers,
		groupID:     groupID,
		topics:      topics,
		redisClient: redisClient,
		interval:    30 * time.Second,
		lag:         make(map[string]int64),
	}
}

// Start begins the background lag monitoring loop.
func (m *LagMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	log.Info().Str("group_id", m.groupID).Msg("Starting Kafka lag monitor")

	for {
		select {
		case <-ticker.C:
			m.fetchLag(ctx)
		case <-ctx.Done():
			log.Info().Msg("Stopping Kafka lag monitor")
			return
		}
	}
}

// GetLag returns the current per-partition lag snapshot.
func (m *LagMonitor) GetLag() map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]int64, len(m.lag))
	for k, v := range m.lag {
		result[k] = v
	}
	return result
}

func (m *LagMonitor) fetchLag(ctx context.Context) {
	config := sarama.NewConfig()
	config.Version = sarama.V2_6_0_0

	admin, err := sarama.NewClusterAdmin(m.brokers, config)
	if err != nil {
		log.Debug().Err(err).Msg("Lag monitor: failed to create admin client")
		return
	}
	defer admin.Close()

	client, err := sarama.NewClient(m.brokers, config)
	if err != nil {
		log.Debug().Err(err).Msg("Lag monitor: failed to create client")
		return
	}
	defer client.Close()

	offsets, err := admin.ListConsumerGroupOffsets(m.groupID, nil)
	if err != nil {
		log.Debug().Err(err).Str("group_id", m.groupID).Msg("Lag monitor: failed to get consumer offsets")
		return
	}

	newLag := make(map[string]int64)
	totalLag := int64(0)

	for topic, partitions := range offsets.Blocks {
		for partition, block := range partitions {
			if block.Offset < 0 {
				continue
			}
			newestOffset, err := client.GetOffset(topic, partition, sarama.OffsetNewest)
			if err != nil {
				continue
			}
			lag := newestOffset - block.Offset
			if lag < 0 {
				lag = 0
			}
			key := fmt.Sprintf("%s:%d", topic, partition)
			newLag[key] = lag
			totalLag += lag
		}
	}

	m.mu.Lock()
	m.lag = newLag
	m.mu.Unlock()

	// Publish to Redis
	if m.redisClient != nil {
		lagJSON, err := json.Marshal(newLag)
		if err == nil {
			redisKey := fmt.Sprintf("kafka:consumer-lag:%s", m.groupID)
			m.redisClient.Set(ctx, redisKey, lagJSON, 60*time.Second)
		}
	}

	if totalLag > 0 {
		log.Debug().Int64("total_lag", totalLag).Str("group_id", m.groupID).Msg("Consumer lag updated")
	}
}
