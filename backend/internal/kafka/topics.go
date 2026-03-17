package kafka

import (
	"fmt"

	"github.com/IBM/sarama"
	"github.com/rs/zerolog/log"
)

// TopicSpec defines the desired configuration for a Kafka topic.
type TopicSpec struct {
	Name              string
	NumPartitions     int32
	ReplicationFactor int16
}

// EnsureTopics creates topics if they don't exist. Idempotent: no-op if topic exists.
func EnsureTopics(brokers []string, topics []TopicSpec) error {
	config := sarama.NewConfig()
	config.Version = sarama.V2_6_0_0

	admin, err := sarama.NewClusterAdmin(brokers, config)
	if err != nil {
		return fmt.Errorf("failed to create cluster admin: %w", err)
	}
	defer admin.Close()

	existing, err := admin.ListTopics()
	if err != nil {
		return fmt.Errorf("failed to list topics: %w", err)
	}

	for _, spec := range topics {
		if detail, ok := existing[spec.Name]; ok {
			if int32(detail.NumPartitions) != spec.NumPartitions {
				log.Warn().
					Str("topic", spec.Name).
					Int32("existing_partitions", int32(detail.NumPartitions)).
					Int32("desired_partitions", spec.NumPartitions).
					Msg("Topic exists with different partition count")
			}
			log.Debug().Str("topic", spec.Name).Msg("Topic already exists")
			continue
		}

		topicDetail := &sarama.TopicDetail{
			NumPartitions:     spec.NumPartitions,
			ReplicationFactor: spec.ReplicationFactor,
		}

		if err := admin.CreateTopic(spec.Name, topicDetail, false); err != nil {
			// Ignore "topic already exists" errors (race condition safe)
			if terr, ok := err.(*sarama.TopicError); ok && terr.Err == sarama.ErrTopicAlreadyExists {
				log.Debug().Str("topic", spec.Name).Msg("Topic already exists (race)")
				continue
			}
			return fmt.Errorf("failed to create topic %s: %w", spec.Name, err)
		}

		log.Info().
			Str("topic", spec.Name).
			Int32("partitions", spec.NumPartitions).
			Int16("replication_factor", spec.ReplicationFactor).
			Msg("Created Kafka topic")
	}

	return nil
}
