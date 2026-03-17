package kafka

import (
	"hash/fnv"

	"github.com/IBM/sarama"
	"github.com/chetansierra/smart-city-monitor/internal/zones"
)

// ZonePartitioner routes messages to partitions based on geographic zone.
type ZonePartitioner struct {
	numPartitions int32
}

// NewZonePartitioner creates a zone-based partitioner.
func NewZonePartitioner(topic string) sarama.Partitioner {
	return &ZonePartitioner{}
}

func (p *ZonePartitioner) Partition(message *sarama.ProducerMessage, numPartitions int32) (int32, error) {
	p.numPartitions = numPartitions

	if message.Key == nil {
		return 0, nil
	}

	keyBytes, err := message.Key.Encode()
	if err != nil {
		return 0, nil
	}

	// Use the key as sensor ID; determine zone from key or fall back to hash
	zone := zones.GetZoneFromKey(string(keyBytes))

	h := fnv.New32a()
	h.Write([]byte(zone))
	partition := int32(h.Sum32()) % numPartitions
	if partition < 0 {
		partition = -partition
	}
	return partition, nil
}

func (p *ZonePartitioner) RequiresConsistency() bool {
	return true
}

func (p *ZonePartitioner) MessageRequiresConsistency(message *sarama.ProducerMessage) bool {
	return true
}
