package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/chetansierra/smart-city-monitor/internal/redis"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

// PipelineHandler handles data pipeline visualization endpoints
type PipelineHandler struct {
	redisClient  *redis.Client
	kafkaBrokers []string
}

// NewPipelineHandler creates a new pipeline handler
func NewPipelineHandler(redisClient *redis.Client, kafkaBrokers []string) *PipelineHandler {
	return &PipelineHandler{
		redisClient:  redisClient,
		kafkaBrokers: kafkaBrokers,
	}
}

// KafkaMessage represents a message from Kafka
type KafkaMessage struct {
	Topic     string            `json:"topic"`
	Partition int32             `json:"partition"`
	Offset    int64             `json:"offset"`
	Timestamp time.Time         `json:"timestamp"`
	Key       string            `json:"key,omitempty"`
	Value     json.RawMessage   `json:"value"`
	Headers   map[string]string `json:"headers,omitempty"`
}

// KafkaTopicDetail represents detailed topic information
type KafkaTopicDetail struct {
	Name              string            `json:"name"`
	Partitions        []PartitionDetail `json:"partitions"`
	ReplicationFactor int               `json:"replication_factor"`
	Config            map[string]string `json:"config"`
	TotalMessages     int64             `json:"total_messages"`
}

// PartitionDetail represents partition information
type PartitionDetail struct {
	ID           int32   `json:"id"`
	Leader       int32   `json:"leader"`
	Replicas     []int32 `json:"replicas"`
	ISR          []int32 `json:"isr"`
	OldestOffset int64   `json:"oldest_offset"`
	NewestOffset int64   `json:"newest_offset"`
	MessageCount int64   `json:"message_count"`
}

// RedisKeyInfo represents information about a Redis key
type RedisKeyInfo struct {
	Key      string      `json:"key"`
	Type     string      `json:"type"`
	TTL      int64       `json:"ttl"`
	Size     int64       `json:"size"`
	Value    interface{} `json:"value,omitempty"`
	Encoding string      `json:"encoding,omitempty"`
}

// SSEConnection represents an SSE stream connection
type SSEConnection struct {
	ID           string    `json:"id"`
	RemoteAddr   string    `json:"remote_addr"`
	ConnectedAt  time.Time `json:"connected_at"`
	Duration     string    `json:"duration"`
	EventsSent   int64     `json:"events_sent"`
	LastActivity time.Time `json:"last_activity"`
}

// DataFlowStats represents real-time data flow statistics
type DataFlowStats struct {
	Sensors          int     `json:"sensors"`
	KafkaTopics      int     `json:"kafka_topics"`
	KafkaMessages    int64   `json:"kafka_messages"`
	RedisKeys        int64   `json:"redis_keys"`
	DatabaseRecords  int64   `json:"database_records"`
	SSEClients       int     `json:"sse_clients"`
	ThroughputMsgSec float64 `json:"throughput_msg_sec"`
	Timestamp        string  `json:"timestamp"`
}

// GetKafkaTopics handles GET /api/v1/pipeline/kafka/topics
func (h *PipelineHandler) GetKafkaTopics(c *fiber.Ctx) error {
	config := sarama.NewConfig()
	config.Version = sarama.V2_6_0_0

	admin, err := sarama.NewClusterAdmin(h.kafkaBrokers, config)
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
	defer admin.Close()

	// List all topics
	topicsMap, err := admin.ListTopics()
	if err != nil {
		log.Error().Err(err).Msg("Failed to list topics")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    "KAFKA_LIST_ERROR",
				"message": "Failed to list Kafka topics",
			},
		})
	}

	topics := make([]string, 0, len(topicsMap))
	for topic := range topicsMap {
		topics = append(topics, topic)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"topics":    topics,
			"count":     len(topics),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		},
	})
}

// GetKafkaTopicDetail handles GET /api/v1/pipeline/kafka/topics/:topic
func (h *PipelineHandler) GetKafkaTopicDetail(c *fiber.Ctx) error {
	topicName := c.Params("topic")
	if topicName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    "INVALID_TOPIC",
				"message": "Topic name is required",
			},
		})
	}

	config := sarama.NewConfig()
	config.Version = sarama.V2_6_0_0

	admin, err := sarama.NewClusterAdmin(h.kafkaBrokers, config)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    "KAFKA_CONNECTION_ERROR",
				"message": "Failed to connect to Kafka",
			},
		})
	}
	defer admin.Close()

	// Get topic metadata
	topicsMap, err := admin.ListTopics()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    "KAFKA_LIST_ERROR",
				"message": "Failed to list topics",
			},
		})
	}

	topicDetail, exists := topicsMap[topicName]
	if !exists {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    "TOPIC_NOT_FOUND",
				"message": fmt.Sprintf("Topic '%s' not found", topicName),
			},
		})
	}

	// Create consumer to get partition details
	consumer, err := sarama.NewConsumer(h.kafkaBrokers, config)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    "KAFKA_CONSUMER_ERROR",
				"message": "Failed to create Kafka consumer",
			},
		})
	}
	defer consumer.Close()

	partitionIDs, err := consumer.Partitions(topicName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    "KAFKA_PARTITION_ERROR",
				"message": "Failed to get partitions",
			},
		})
	}

	// Create Kafka client to get offsets
	client, err := sarama.NewClient(h.kafkaBrokers, config)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    "KAFKA_CLIENT_ERROR",
				"message": "Failed to create Kafka client",
			},
		})
	}
	defer client.Close()

	// Get partition details
	partitions := make([]PartitionDetail, 0, len(partitionIDs))
	totalMessages := int64(0)

	for _, partitionID := range partitionIDs {
		oldestOffset, err := client.GetOffset(topicName, partitionID, sarama.OffsetOldest)
		if err != nil {
			log.Error().Err(err).Int32("partition", partitionID).Msg("Failed to get oldest offset")
			continue
		}

		newestOffset, err := client.GetOffset(topicName, partitionID, sarama.OffsetNewest)
		if err != nil {
			log.Error().Err(err).Int32("partition", partitionID).Msg("Failed to get newest offset")
			continue
		}

		messageCount := newestOffset - oldestOffset
		totalMessages += messageCount

		partitions = append(partitions, PartitionDetail{
			ID:           partitionID,
			Leader:       -1, // Would need metadata for this
			Replicas:     []int32{},
			ISR:          []int32{},
			OldestOffset: oldestOffset,
			NewestOffset: newestOffset,
			MessageCount: messageCount,
		})
	}

	replicationFactor := int(topicDetail.ReplicationFactor)
	if replicationFactor == -1 {
		replicationFactor = 1
	}

	response := KafkaTopicDetail{
		Name:              topicName,
		Partitions:        partitions,
		ReplicationFactor: replicationFactor,
		Config:            make(map[string]string),
		TotalMessages:     totalMessages,
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    response,
	})
}

// GetKafkaMessages handles GET /api/v1/pipeline/kafka/topics/:topic/messages
func (h *PipelineHandler) GetKafkaMessages(c *fiber.Ctx) error {
	topicName := c.Params("topic")
	partition := c.QueryInt("partition", 0)
	offset := c.QueryInt("offset", -1) // -1 means newest
	limit := c.QueryInt("limit", 10)

	if limit > 100 {
		limit = 100 // Cap at 100 messages
	}

	config := sarama.NewConfig()
	config.Version = sarama.V2_6_0_0
	config.Consumer.Return.Errors = true

	// Create Kafka client to get offsets
	client, err := sarama.NewClient(h.kafkaBrokers, config)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    "KAFKA_CLIENT_ERROR",
				"message": "Failed to create Kafka client",
			},
		})
	}
	defer client.Close()

	// Determine starting offset
	var startOffset int64
	if offset == -1 {
		// Get newest offset and go back by limit
		newestOffset, err := client.GetOffset(topicName, int32(partition), sarama.OffsetNewest)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error": fiber.Map{
					"code":    "OFFSET_ERROR",
					"message": "Failed to get offset",
				},
			})
		}
		startOffset = newestOffset - int64(limit)
		if startOffset < 0 {
			startOffset = 0
		}
	} else {
		startOffset = int64(offset)
	}

	// Create consumer
	consumer, err := sarama.NewConsumer(h.kafkaBrokers, config)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    "KAFKA_CONSUMER_ERROR",
				"message": "Failed to create Kafka consumer",
			},
		})
	}
	defer consumer.Close()

	// Create partition consumer
	partitionConsumer, err := consumer.ConsumePartition(topicName, int32(partition), startOffset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    "PARTITION_CONSUMER_ERROR",
				"message": fmt.Sprintf("Failed to consume partition: %v", err),
			},
		})
	}
	defer partitionConsumer.Close()

	// Collect messages with timeout
	messages := make([]KafkaMessage, 0, limit)
	timeout := time.After(5 * time.Second)

	for len(messages) < limit {
		select {
		case msg := <-partitionConsumer.Messages():
			headers := make(map[string]string)
			for _, header := range msg.Headers {
				headers[string(header.Key)] = string(header.Value)
			}

			messages = append(messages, KafkaMessage{
				Topic:     msg.Topic,
				Partition: msg.Partition,
				Offset:    msg.Offset,
				Timestamp: msg.Timestamp,
				Key:       string(msg.Key),
				Value:     json.RawMessage(msg.Value),
				Headers:   headers,
			})
		case <-timeout:
			// Timeout reached, return what we have
			goto done
		}
	}

done:
	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"topic":     topicName,
			"partition": partition,
			"offset":    startOffset,
			"messages":  messages,
			"count":     len(messages),
		},
	})
}

// GetRedisKeys handles GET /api/v1/pipeline/redis/keys
func (h *PipelineHandler) GetRedisKeys(c *fiber.Ctx) error {
	ctx := context.Background()
	pattern := c.Query("pattern", "*")
	limit := c.QueryInt("limit", 100)

	if limit > 1000 {
		limit = 1000 // Cap at 1000 keys
	}

	// Use SCAN instead of KEYS for better performance
	var cursor uint64
	var keys []string

	for len(keys) < limit {
		var scanKeys []string
		var err error

		scanKeys, cursor, err = h.redisClient.Client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan Redis keys")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error": fiber.Map{
					"code":    "REDIS_SCAN_ERROR",
					"message": "Failed to scan Redis keys",
				},
			})
		}

		keys = append(keys, scanKeys...)

		if cursor == 0 {
			break
		}
	}

	// Limit results
	if len(keys) > limit {
		keys = keys[:limit]
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"keys":    keys,
			"count":   len(keys),
			"pattern": pattern,
		},
	})
}

// GetRedisKeyDetail handles GET /api/v1/pipeline/redis/keys/:key
func (h *PipelineHandler) GetRedisKeyDetail(c *fiber.Ctx) error {
	ctx := context.Background()
	key := c.Params("key")

	if key == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    "INVALID_KEY",
				"message": "Key is required",
			},
		})
	}

	// Check if key exists
	exists, err := h.redisClient.Client.Exists(ctx, key).Result()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    "REDIS_ERROR",
				"message": "Failed to check key existence",
			},
		})
	}

	if exists == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    "KEY_NOT_FOUND",
				"message": fmt.Sprintf("Key '%s' not found", key),
			},
		})
	}

	// Get key type
	keyType, err := h.redisClient.Client.Type(ctx, key).Result()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    "REDIS_TYPE_ERROR",
				"message": "Failed to get key type",
			},
		})
	}

	// Get TTL
	ttl, err := h.redisClient.Client.TTL(ctx, key).Result()
	if err != nil {
		ttl = -1
	}

	// Get value based on type
	var value interface{}
	var size int64

	switch keyType {
	case "string":
		strValue, err := h.redisClient.Client.Get(ctx, key).Result()
		if err == nil {
			value = strValue
			size = int64(len(strValue))
		}
	case "list":
		listValue, err := h.redisClient.Client.LRange(ctx, key, 0, 99).Result()
		if err == nil {
			value = listValue
			size = int64(len(listValue))
		}
	case "set":
		setValue, err := h.redisClient.Client.SMembers(ctx, key).Result()
		if err == nil {
			value = setValue
			size = int64(len(setValue))
		}
	case "zset":
		zsetValue, err := h.redisClient.Client.ZRangeWithScores(ctx, key, 0, 99).Result()
		if err == nil {
			value = zsetValue
			size = int64(len(zsetValue))
		}
	case "hash":
		hashValue, err := h.redisClient.Client.HGetAll(ctx, key).Result()
		if err == nil {
			value = hashValue
			size = int64(len(hashValue))
		}
	}

	keyInfo := RedisKeyInfo{
		Key:   key,
		Type:  keyType,
		TTL:   int64(ttl.Seconds()),
		Size:  size,
		Value: value,
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    keyInfo,
	})
}

// GetSSEConnections handles GET /api/v1/pipeline/stream/connections
func (h *PipelineHandler) GetSSEConnections(c *fiber.Ctx) error {
	// For now, return mock data structure or integrate with broadcaster stats
	connections := []SSEConnection{
		{
			ID:           "conn-1",
			RemoteAddr:   "192.168.1.100:54321",
			ConnectedAt:  time.Now().Add(-10 * time.Minute),
			Duration:     "10m0s",
			EventsSent:   1523,
			LastActivity: time.Now(),
		},
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"connections":  connections,
			"active_count": len(connections),
			"total_events": 1523,
			"timestamp":    time.Now().UTC().Format(time.RFC3339),
		},
	})
}

// GetDataFlowStats handles GET /api/v1/pipeline/flow/stats
func (h *PipelineHandler) GetDataFlowStats(c *fiber.Ctx) error {
	ctx := context.Background()

	// Get sensor count from Redis
	sensorsKey := "sensors:*"
	sensorKeys, _ := h.redisClient.Client.Keys(ctx, sensorsKey).Result()

	// Get Kafka topic count
	config := sarama.NewConfig()
	config.Version = sarama.V2_6_0_0
	admin, err := sarama.NewClusterAdmin(h.kafkaBrokers, config)

	topicCount := 0
	totalMessages := int64(0)

	if err == nil {
		topicsMap, err := admin.ListTopics()
		if err == nil {
			topicCount = len(topicsMap)
		}
		admin.Close()
	}

	// Get Redis key count
	var cursor uint64
	redisKeyCount := int64(0)
	for {
		keys, newCursor, err := h.redisClient.Client.Scan(ctx, cursor, "*", 100).Result()
		if err != nil {
			break
		}
		redisKeyCount += int64(len(keys))
		cursor = newCursor
		if cursor == 0 {
			break
		}
	}

	stats := DataFlowStats{
		Sensors:          len(sensorKeys),
		KafkaTopics:      topicCount,
		KafkaMessages:    totalMessages,
		RedisKeys:        redisKeyCount,
		DatabaseRecords:  0, // TODO: Get from actual database
		SSEClients:       1, // TODO: Get from broadcaster
		ThroughputMsgSec: 0, // TODO: Calculate from metrics
		Timestamp:        time.Now().UTC().Format(time.RFC3339),
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    stats,
	})
}
