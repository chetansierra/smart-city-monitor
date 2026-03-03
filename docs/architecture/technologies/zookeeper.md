# Zookeeper Architecture Notes

## Role

Zookeeper coordinates Kafka broker metadata and cluster state in current Docker Compose setup.

## Runtime Notes

- Single-node development setup.
- Kafka service depends on Zookeeper health in compose.

If migrating to KRaft mode in future, this component can be removed.
