# Smart City Monitor — Zookeeper (DEPRECATED)

## Status: Not Used

Zookeeper is NOT part of the Smart City Monitor deployment. Kafka runs in KRaft mode (self-managed metadata), which eliminates the need for Zookeeper.

This file is retained for historical reference only.

## Historical Context

Early versions of the Docker Compose setup used Zookeeper for Kafka broker coordination. The system was migrated to KRaft mode (`confluentinc/cp-kafka:7.5.0`) with `KAFKA_PROCESS_ROLES: "broker,controller"`, making Zookeeper unnecessary.
