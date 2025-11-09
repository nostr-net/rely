# Architecture Overview

## Project Structure

The project is now cleanly separated into **library** and **application**:

```
rely/
├── [LIBRARY CODE]
│   ├── relay.go              # Core rely library
│   ├── client.go             # Client management
│   ├── subscription.go       # Subscription handling
│   └── ...                   # Other library code
│
└── cmd/nostr-relay/          # PRODUCTION RELAY APPLICATION
    ├── main.go               # Application entry point
    ├── config/               # Configuration management
    │   └── config.go
    ├── internal/             # Internal packages (not exported)
    │   └── storage/          # Storage implementations
    │       └── clickhouse/   # ClickHouse storage backend
    │           ├── storage.go
    │           ├── insert.go
    │           ├── query.go
    │           ├── count.go
    │           ├── analytics.go
    │           └── migrations/
    ├── config.yaml.example   # Example configuration
    ├── Dockerfile            # Container image
    ├── docker-compose.yml    # Full stack deployment
    ├── Makefile              # Build automation
    └── systemd/              # Service management
        └── nostr-relay.service
```

## Separation of Concerns

### Rely Library (`github.com/nostr-net/rely`)

**Purpose**: Generic Nostr relay framework

**Responsibilities**:
- WebSocket connection management
- Nostr protocol (NIP-01) implementation
- Event validation and routing
- Subscription management
- Client authentication (NIP-42)

**Does NOT include**:
- Storage implementations
- Configuration management
- Deployment tooling
- Monitoring

**Usage**:
```go
import "github.com/nostr-net/rely"

relay := rely.NewRelay(opts...)
relay.On.Event = yourStorageImplementation
relay.StartAndServe(ctx, addr)
```

### Nostr Relay Application (`cmd/nostr-relay`)

**Purpose**: Production-ready relay using rely library

**Responsibilities**:
- Application lifecycle (startup, shutdown)
- Configuration (files, env vars)
- Storage implementation (ClickHouse)
- Monitoring and health checks
- Deployment artifacts (Docker, systemd)

**Key Components**:

#### 1. Main Application (`main.go`)
- Initializes configuration
- Sets up ClickHouse storage
- Creates and configures rely instance
- Handles graceful shutdown
- Periodic statistics reporting

#### 2. Configuration (`config/`)
- YAML-based configuration
- Environment variable overrides
- Validation
- Defaults

#### 3. Storage Layer (`internal/storage/clickhouse/`)
- ClickHouse-specific implementation
- Event insertion (batched)
- Event querying (optimized)
- Analytics queries
- Database migrations

## Data Flow

```
┌─────────────────────────────────────────────────────────┐
│                    Nostr Client                         │
│                  (WebSocket Connection)                 │
└────────────────────────┬────────────────────────────────┘
                         │
                         │ EVENT/REQ/COUNT
                         ▼
┌─────────────────────────────────────────────────────────┐
│                  Rely Library                            │
│  ┌────────────────────────────────────────────────┐     │
│  │  • WebSocket Handler                           │     │
│  │  • Protocol Parser                             │     │
│  │  • Event Validation                            │     │
│  │  • Subscription Manager                        │     │
│  └────────────────────┬───────────────────────────┘     │
└─────────────────────────────────────────────────────────┘
                         │
                         │ Hooks: On.Event, On.Req, On.Count
                         ▼
┌─────────────────────────────────────────────────────────┐
│               Relay Application (main.go)                │
│  ┌────────────────────────────────────────────────┐     │
│  │  Routing Logic                                 │     │
│  │  • relay.On.Event = storage.SaveEvent         │     │
│  │  • relay.On.Req = storage.QueryEvents         │     │
│  │  • relay.On.Count = storage.CountEvents       │     │
│  └────────────────────┬───────────────────────────┘     │
└─────────────────────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────┐
│          ClickHouse Storage (internal/storage)           │
│  ┌────────────────────────────────────────────────┐     │
│  │  • SaveEvent: Batch insertion                 │     │
│  │  • QueryEvents: Optimized queries             │     │
│  │  • CountEvents: Fast counting                 │     │
│  │  • Analytics: Pre-aggregated queries          │     │
│  └────────────────────┬───────────────────────────┘     │
└─────────────────────────────────────────────────────────┘
                         │
                         │ SQL Queries
                         ▼
┌─────────────────────────────────────────────────────────┐
│                  ClickHouse Database                     │
│  ┌──────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │  Events  │  │ Materialized │  │    Analytics     │  │
│  │  Table   │  │    Views     │  │     Tables       │  │
│  └──────────┘  └──────────────┘  └──────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

## Configuration Flow

```
Priority (high to low):

1. Environment Variables
   ↓
2. config.yaml
   ↓
3. Defaults (code)
```

Example:
```bash
# Environment overrides file and defaults
export CLICKHOUSE_DSN="clickhouse://prod-db:9000/nostr"
export DOMAIN="relay.example.com"

# config.yaml used for other settings
./nostr-relay
```

## Storage Interface

The relay expects a storage implementation that provides these methods:

```go
type Storage interface {
    SaveEvent(c rely.Client, event *nostr.Event) error
    QueryEvents(ctx context.Context, c rely.Client, filters nostr.Filters) ([]nostr.Event, error)
    CountEvents(c rely.Client, filters nostr.Filters) (int64, bool, error)
}
```

**Current Implementation**: ClickHouse
**Future Possibilities**: PostgreSQL, SQLite, S3, etc.

## Deployment Models

### 1. Development
```bash
# Local development with hot reload
make dev
```

### 2. Docker Compose
```bash
# Full stack (ClickHouse + Relay)
docker-compose up -d
```

### 3. Kubernetes
```yaml
# Deploy as pods with health checks
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nostr-relay
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: relay
        image: nostr-relay:latest
        ports:
        - containerPort: 3334
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
```

### 4. Systemd
```bash
# Native Linux service
sudo systemctl start nostr-relay
```

## Scaling Considerations

### Horizontal Scaling

The relay can be horizontally scaled:

```
                    ┌──────────────┐
                    │ Load Balancer│
                    └──────┬───────┘
                           │
          ┌────────────────┼────────────────┐
          │                │                │
    ┌─────▼─────┐    ┌─────▼─────┐   ┌─────▼─────┐
    │  Relay 1  │    │  Relay 2  │   │  Relay 3  │
    └─────┬─────┘    └─────┬─────┘   └─────┬─────┘
          │                │                │
          └────────────────┼────────────────┘
                           │
                    ┌──────▼───────┐
                    │  ClickHouse  │
                    │   Cluster    │
                    └──────────────┘
```

**Requirements**:
- Shared ClickHouse instance/cluster
- WebSocket sticky sessions (load balancer)
- Consistent configuration

### ClickHouse Scaling

For high-write volumes:
- Use ClickHouse cluster (sharding + replication)
- Increase batch size
- Tune flush interval
- Add more relay instances

## Monitoring Stack

Recommended production monitoring:

```
┌────────────────┐
│  Nostr Relay   │───► Logs ───► Loki/ElasticSearch
│                │
│                │───► Metrics ─► Prometheus
│                │
│                │───► Traces ──► Jaeger
└────────────────┘
```

**Metrics to track**:
- Connected clients
- Events per second
- Query latency
- Batch insert performance
- ClickHouse query times
- Queue depth

## Security Considerations

### Network Security
- TLS/SSL for WebSocket (wss://)
- Firewall rules (only 3334, 8080)
- ClickHouse not exposed publicly

### Application Security
- Event signature verification (rely library)
- SQL injection prevention (parameterized queries)
- Resource limits (max connections, subscriptions)
- Rate limiting (future enhancement)

### System Security
- Non-root user (systemd, Docker)
- Read-only filesystem where possible
- Minimal privileges (systemd hardening)
- Regular security updates

## Performance Targets

### Write Performance
- **50K-200K events/second** (depending on hardware)
- **<2ms** latency for event acceptance
- **<100ms** batch insertion to ClickHouse

### Read Performance
- **<5ms** for ID lookup
- **<50ms** for author queries
- **<500ms** for complex filters
- **1K-10K** queries/second

### Resource Usage
- **~100-500MB** RAM per relay instance
- **~1 CPU core** at 10K events/sec
- **Storage**: 70-85% compression vs raw JSON

## Extensibility

The architecture supports easy extension:

### Adding New Storage Backend

1. Implement storage interface in `internal/storage/yourstorage/`
2. Update `main.go` to use new storage
3. Add configuration for new backend

### Adding Monitoring

1. Import metrics library
2. Add metrics in `main.go`
3. Expose `/metrics` endpoint

### Adding Features

1. Implement in rely library (if generic)
2. Or add to relay application (if specific)
3. Configure via `config.yaml`

## Migration Path

From example to production:

1. **Start**: `examples/clickhouse/main.go`
2. **Develop**: `cmd/nostr-relay/` (this)
3. **Customize**: Fork and modify for your needs

## Summary

**Clean Architecture Benefits**:
✅ Rely remains a pure library
✅ Storage implementations are pluggable
✅ Easy to test and maintain
✅ Multiple relay types possible
✅ Production deployment patterns included

**This Structure Enables**:
- Anyone can build a relay with rely
- Storage backends are implementation details
- Configuration is flexible and powerful
- Deployment is straightforward
- Monitoring and operations are built-in
