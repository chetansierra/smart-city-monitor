# Smart City Monitor — Resilience Patterns

## Overview

Smart City Monitor implements three resilience patterns to handle infrastructure failures gracefully: circuit breakers, retry with exponential backoff, and a dead letter queue (DLQ). These prevent cascading failures and ensure data is not silently lost.

## Circuit Breakers

Implementation: `backend/internal/resilience/circuitbreaker.go`

### State Machine

```
Closed (normal) → Open (after threshold failures) → Half-Open (after timeout) → Closed (on success)
                                                   → Open (on failure in half-open)
```

- **Closed:** All calls pass through. Failures increment a counter.
- **Open:** All calls are rejected immediately (fail-fast). No external calls are made.
- **Half-Open:** After `CB_TIMEOUT_SECONDS`, a single call is allowed through. Success closes the breaker. Failure re-opens it.

### Where Circuit Breakers Are Used

| Location | Wraps | Behavior When Open |
|----------|-------|--------------------|
| Sensor Simulator | Kafka producer | Messages buffered in-memory (up to 1000). Generation loop continues. |
| Data Ingestion | Redis cache operations | Redis updates skipped. Kafka consumption and anomaly detection continue. |

### Configuration

| Variable | Default | Purpose |
|----------|---------|---------|
| `CB_THRESHOLD` | 5 | Number of consecutive failures before opening |
| `CB_TIMEOUT_SECONDS` | 30 | Seconds to wait before transitioning from open to half-open |

## Retry with Exponential Backoff

Implementation: `backend/internal/resilience/retry.go`

### Behavior

When a Kafka message fails to process in data ingestion:

1. Retry up to `MAX_RETRIES` times (default: 3).
2. Each retry waits with exponential backoff: `RETRY_INITIAL_BACKOFF_MS * 2^attempt`.
3. Jitter is added to prevent thundering herd.
4. If all retries are exhausted, the message is sent to the dead letter queue.

### Configuration

| Variable | Default | Purpose |
|----------|---------|---------|
| `MAX_RETRIES` | 3 | Maximum retry attempts |
| `RETRY_INITIAL_BACKOFF_MS` | 100 | Initial backoff in milliseconds |

### Backoff Schedule (with defaults)

| Attempt | Base Delay |
|---------|------------|
| 1 | 100ms |
| 2 | 200ms |
| 3 | 400ms |
| Exhausted | → DLQ |

## Dead Letter Queue (DLQ)

Implementation: `backend/internal/kafka/dlq.go`

### Behavior

Messages that fail all retry attempts are published to the `sensor-readings-dlq` Kafka topic with metadata headers:

| Header | Content |
|--------|---------|
| `error` | Error message from the last failure |
| `original-topic` | The source topic (`sensor-readings`) |
| `retry-count` | Number of retries attempted |
| `failed-at` | Timestamp of final failure |

### Browsing the DLQ

DLQ messages can be browsed via the REST API:

```
GET /api/v1/pipeline/dlq?limit=10
```

### Recovery

DLQ messages are not automatically reprocessed. Manual investigation is required:

1. Browse DLQ messages via the API.
2. Identify root cause from `error` header.
3. Fix the issue.
4. Replay messages if needed (manual process).

## Failed Batch Fallback

If PostgreSQL batch writes fail after all retries in data ingestion, the batch data is written to a JSON file inside the container:

```
/tmp/smart-city-failed-batch-{timestamp}.json
```

This provides a last-resort recovery mechanism for data that could not be persisted.

## Observability

- Circuit breaker state changes are logged (`circuit breaker opened`, `circuit breaker closed`).
- Retry attempts are logged with attempt number and delay.
- DLQ writes are logged with message ID and error.
- Kafka consumer lag is monitored every 30 seconds and cached in Redis (`kafka:consumer-lag:{groupID}`).
