# SSE Architecture Notes

## Role

SSE is the only real-time transport to frontend clients.

## Stream Endpoint

- `/stream?session_id=<uuid>`

## Event Types

- `sensor_update`
- `stats_update`

## Behavior

- API gateway broadcasts from Redis pub/sub and internal metrics aggregator.
- Frontend uses SSE-first updates; polling remains fallback only where needed.
