# Docker Deployment Guide

Quick reference for deploying and managing the Smart City Monitor with Docker.

## 🚀 Quick Start

```bash
# Start everything
./scripts/start-all.sh

# Test the stack
./scripts/test-stack.sh

# Stop everything
./scripts/stop-all.sh
```

## 📦 Services Overview

| Service          | Container Name    | Port  | Purpose                          |
|------------------|-------------------|-------|----------------------------------|
| API Gateway      | api-gateway       | 8080  | REST API + WebSocket server      |
| Alert Monitor    | alert-monitor     | -     | Alert detection and generation   |
| PostgreSQL       | postgres          | 5433  | Primary database                 |
| Redis            | redis             | 6379  | Cache and pub/sub                |
| Kafka            | kafka             | 9092  | Message broker                   |
| Zookeeper        | zookeeper         | 2181  | Kafka coordination               |
| Kafka UI         | kafka-ui          | 8081  | Kafka web interface              |
| Redis Commander  | redis-commander   | 8082  | Redis web interface              |

## 🔨 Common Commands

### Start Services

```bash
# Start all services
./scripts/start-all.sh

# Start only Docker infrastructure (no Go apps)
docker compose up -d

# Start specific service
docker compose up -d api-gateway
```

### Stop Services

```bash
# Stop everything (including Go apps)
./scripts/stop-all.sh

# Stop only Docker services
docker compose down

# Stop and remove volumes (⚠️ deletes all data)
docker compose down -v
```

### View Logs

```bash
# Follow API Gateway logs
docker logs -f api-gateway

# Follow Alert Monitor logs
docker logs -f alert-monitor

# View all Docker service logs
docker compose logs -f

# View specific service logs
docker compose logs -f postgres redis kafka
```

### Rebuild Services

```bash
# Rebuild all services
docker compose build

# Rebuild specific service
docker compose build api-gateway

# Force rebuild (no cache)
docker compose build --no-cache api-gateway

# Rebuild and restart
docker compose up -d --build api-gateway
```

### Service Status

```bash
# Check running containers
docker compose ps

# Check all containers (including stopped)
docker compose ps -a

# Detailed status
docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
```

## 🔍 Debugging

### Check Service Health

```bash
# API Gateway health
curl http://localhost:8080/health

# PostgreSQL health
docker exec postgres pg_isready -U admin

# Redis health
docker exec redis redis-cli ping

# Kafka health
docker exec kafka kafka-broker-api-versions --bootstrap-server localhost:9093
```

### Enter Container Shell

```bash
# API Gateway container
docker exec -it api-gateway sh

# PostgreSQL container
docker exec -it postgres psql -U admin -d smart_city

# Redis container
docker exec -it redis redis-cli

# Kafka container
docker exec -it kafka bash
```

### Common Issues

#### Port Already in Use

```bash
# Find process using port 8080
lsof -i :8080

# Kill the process
kill -9 <PID>

# Or change port in docker-compose.yml
```

#### Service Won't Start

```bash
# Check logs for errors
docker logs api-gateway

# Restart service
docker compose restart api-gateway

# Rebuild and restart
docker compose up -d --build --force-recreate api-gateway
```

#### Database Connection Issues

```bash
# Check if PostgreSQL is ready
docker exec postgres pg_isready -U admin

# Check connection from API Gateway
docker exec api-gateway wget -qO- http://localhost:8080/health
```

#### Clean Slate

```bash
# Stop everything
docker compose down

# Remove all volumes (⚠️ deletes data)
docker compose down -v

# Remove all containers and images
docker compose down --rmi all

# Start fresh
docker compose up -d --build
```

## 📊 Monitoring

### Resource Usage

```bash
# Real-time resource usage
docker stats

# Resource usage for Smart City services only
docker stats api-gateway alert-monitor postgres redis kafka
```

### Kafka Topics

```bash
# List all topics
docker exec kafka kafka-topics --bootstrap-server localhost:9093 --list

# Describe topic
docker exec kafka kafka-topics --bootstrap-server localhost:9093 --describe --topic sensor-readings

# View messages
docker exec kafka kafka-console-consumer --bootstrap-server localhost:9093 --topic sensor-readings --from-beginning --max-messages 10
```

### Database Queries

```bash
# Connect to database
docker exec -it postgres psql -U admin -d smart_city

# Count sensors
echo "SELECT COUNT(*) FROM sensors;" | docker exec -i postgres psql -U admin -d smart_city

# Recent readings
echo "SELECT * FROM sensor_readings ORDER BY timestamp DESC LIMIT 10;" | docker exec -i postgres psql -U admin -d smart_city
```

### Redis Keys

```bash
# List all keys
docker exec redis redis-cli KEYS "*"

# Get key value
docker exec redis redis-cli GET "sensor:latest:SENSOR_ID"

# Monitor Redis commands
docker exec redis redis-cli MONITOR
```

## 🔧 Advanced Operations

### Backup Database

```bash
# Backup PostgreSQL
docker exec postgres pg_dump -U admin smart_city > backup.sql

# Restore PostgreSQL
cat backup.sql | docker exec -i postgres psql -U admin smart_city
```

### Export Kafka Messages

```bash
# Export topic to file
docker exec kafka kafka-console-consumer \
  --bootstrap-server localhost:9093 \
  --topic sensor-readings \
  --from-beginning \
  --max-messages 1000 > messages.json
```

### Update Environment Variables

```bash
# Edit docker-compose.yml
vim docker-compose.yml

# Restart service to apply changes
docker compose up -d --force-recreate api-gateway
```

### Scale Services

```bash
# Note: Currently not configured for horizontal scaling
# To enable, update docker-compose.yml with:
# - Load balancer configuration
# - Shared volumes for stateful services
# - Service discovery

# Example (requires additional setup):
docker compose up -d --scale api-gateway=3
```

## 🌐 Access Services

### API Gateway

```bash
# Health check
curl http://localhost:8080/health

# List sensors
curl http://localhost:8080/api/v1/sensors

# Get readings
curl http://localhost:8080/api/v1/readings?limit=10

# City stats
curl http://localhost:8080/api/v1/analytics/city-stats
```

### Web Interfaces

- **API Gateway**: http://localhost:8080/health
- **Kafka UI**: http://localhost:8081
- **Redis Commander**: http://localhost:8082

### WebSocket

```bash
# Using websocat
websocat ws://localhost:8080/ws

# Send subscription message
{"action": "subscribe", "sensor_id": "all"}
```

## 📝 Configuration

### Environment Variables

All services are configured via environment variables in `docker-compose.yml`.

Key configurations:
- **Database**: `POSTGRES_*` variables
- **Redis**: `REDIS_*` variables
- **Kafka**: `KAFKA_*` variables
- **Application**: `APP_*` variables

### Volumes

Persistent data is stored in Docker volumes:
- `postgres-data` - Database files
- `redis-data` - Redis persistence
- `kafka-data` - Kafka logs
- `zookeeper-data` - Zookeeper data
- `zookeeper-logs` - Zookeeper logs

List volumes:
```bash
docker volume ls | grep smart-city
```

Remove volumes (⚠️ deletes data):
```bash
docker volume rm smart-city-monitor_postgres-data
```

## 🧪 Testing

### Integration Tests

```bash
# Ensure stack is running
./scripts/start-all.sh

# Run integration tests
cd backend
go test -v ./tests/integration/...
```

### Performance Tests

```bash
# Ensure stack is running
./scripts/start-all.sh

# Run load tests
cd backend
go run tests/performance/load_test_main.go
```

### Stack Validation

```bash
# Comprehensive stack test
./scripts/test-stack.sh
```

## 🚨 Production Considerations

When deploying to production:

1. **Security**
   - [ ] Change default passwords
   - [ ] Use secrets management (e.g., Docker secrets)
   - [ ] Enable SSL/TLS
   - [ ] Configure firewall rules
   - [ ] Use private networks

2. **Performance**
   - [ ] Add resource limits (CPU, memory)
   - [ ] Configure connection pools
   - [ ] Enable Redis persistence
   - [ ] Tune Kafka settings

3. **Reliability**
   - [ ] Set up log aggregation
   - [ ] Configure monitoring (Prometheus, Grafana)
   - [ ] Set up alerting
   - [ ] Implement backups
   - [ ] Configure restart policies

4. **Scaling**
   - [ ] Use orchestration (Docker Swarm or Kubernetes)
   - [ ] Add load balancer
   - [ ] Configure session affinity
   - [ ] Use managed services for PostgreSQL, Redis, Kafka

Example resource limits:
```yaml
api-gateway:
  deploy:
    resources:
      limits:
        cpus: '1.0'
        memory: 512M
      reservations:
        cpus: '0.5'
        memory: 256M
```

---

For more information:
- API Documentation: [docs/API.md](docs/API.md)
- Postman Guide: [docs/POSTMAN_GUIDE.md](docs/POSTMAN_GUIDE.md)
- Testing Guide: [backend/tests/README.md](backend/tests/README.md)
