# Smart City Monitor — Kafka Architecture

## Role

Kafka is the event backbone of Smart City Monitor. All sensor data, anomaly detections, and pattern events flow through Kafka topics. The system uses Kafka in KRaft mode (no Zookeeper required).

## Deployment

- **Image:** `confluentinc/cp-kafka:7.5.0`
- **Mode:** KRaft (controller + broker in single node)
- **Ports:** 9092 (external/localhost), 9093 (internal/Docker network)
- **Heap:** 512MB max, 256MB initial
- **Persistence:** Docker volume `kafka-data`

## Topics

| Topic | Partitions | Purpose | Producer | Consumer(s) |
|-------|------------|---------|----------|-------------|
| `sensor-readings` | 6 | Main sensor data pipeline | Sensor Simulator | Data Ingestion (`data-ingestion-group`), API Gateway (`gateway-group`) |
| `sensor-anomalies` | 3 | Anomaly detection events | Data Ingestion | API Gateway (`gateway-group`) |
| `sensor-events` | 3 | Zone-level pattern detections | Data Ingestion | API Gateway (`gateway-group`) |
| `sensor-readings-dlq` | 1 | Dead letter queue for failed messages | Data Ingestion | Browsable via API (`/api/v1/pipeline/dlq`) |

Topics are auto-created by services on startup via `backend/internal/kafka/topics.go`.

## Consumer Groups

| Group ID | Service | Topics Consumed |
|----------|---------|-----------------|
| `data-ingestion-group` | Data Ingestion | `sensor-readings` |
| `gateway-group` | API Gateway | `sensor-readings`, `sensor-anomalies`, `sensor-events` |

The two consumer groups operate independently. Data ingestion processes readings for storage and intelligence. The API gateway processes readings for real-time SSE broadcasting.

## Producer Configuration

The Kafka producer in the sensor simulator uses:

- **Idempotent production:** Ensures exactly-once delivery semantics.
- **LZ4 compression:** Reduces message size on the wire.
- **Circuit breaker:** Wraps the producer. Opens after `CB_THRESHOLD` failures, auto-recovers after `CB_TIMEOUT_SECONDS`. During open state, messages are buffered in-memory (up to 1000).

Implementation: `backend/internal/kafka/producer.go`

## Consumer Configuration

The Kafka consumer in data ingestion uses:

- **Retry wrapper:** 3 retries with exponential backoff and jitter.
- **Dead letter queue:** After retry exhaustion, messages are sent to `sensor-readings-dlq` with error metadata headers (`error`, `original-topic`, `retry-count`).

Implementation: `backend/internal/kafka/consumer.go`, `backend/internal/kafka/dlq.go`

## Zone-Based Partitioning

The `sensor-readings` topic uses a custom zone-based partitioner that routes messages to partitions based on the sensor's geographic zone. This ensures all readings from the same zone land on the same partition, enabling efficient zone-level aggregation.

Implementation: `backend/internal/kafka/partitioner.go`

## Consumer Lag Monitoring

Data ingestion runs a background Kafka consumer lag monitor every 30 seconds. Lag data is cached in Redis (`kafka:consumer-lag:{groupID}`) and served via the metrics API.

Implementation: `backend/internal/kafka/metrics.go`

## Message Format

All topics use JSON-encoded messages. See individual service docs for payload schemas.

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `KAFKA_BROKERS` | localhost:9092 | Broker addresses |
| `KAFKA_TOPIC_SENSOR_READINGS` | sensor-readings | Main readings topic |
| `KAFKA_TOPIC_DLQ` | sensor-readings-dlq | Dead letter queue |
| `KAFKA_TOPIC_ANOMALIES` | sensor-anomalies | Anomaly events topic |
| `KAFKA_TOPIC_EVENTS` | sensor-events | Pattern events topic |
| `KAFKA_CONSUMER_GROUP_GATEWAY` | gateway-group | API gateway consumer group |
| `KAFKA_CONSUMER_GROUP_INGESTION` | data-ingestion-group | Data ingestion consumer group |

## Troubleshooting

- **Kafka not ready after compose up:** Wait 15–20 seconds. Topics are auto-created by services.
- **Consumer lag growing:** Check data-ingestion logs for processing errors. Browse DLQ via `/api/v1/pipeline/dlq`.
- **Circuit breaker open on producer:** Check simulator logs. Breaker auto-recovers after timeout. Buffered messages are replayed.
