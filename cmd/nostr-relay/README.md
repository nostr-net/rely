# Nostr Relay with ClickHouse Storage

A production-ready, high-performance Nostr relay implementation using [rely](https://github.com/pippellia-btc/rely) as a library and ClickHouse for storage.

## Features

- ✅ **High Performance**: ClickHouse-backed storage for millions of events
- ✅ **Efficient Batching**: Configurable batch insertion (50K-200K events/sec)
- ✅ **Full NIP Support**: NIP-01, NIP-11, NIP-42, and more
- ✅ **Advanced Analytics**: Built-in analytics tables for insights
- ✅ **Production Ready**: Graceful shutdown, health checks, monitoring
- ✅ **Easy Deployment**: Docker, systemd, and binary deployment options
- ✅ **Configurable**: YAML config + environment variable overrides

## Quick Start

### Prerequisites

- Go 1.21+ (for building from source)
- ClickHouse 23.0+ (for storage)
- Linux/macOS/Windows

### 1. Install ClickHouse

```bash
# Docker (recommended for testing)
docker run -d \
  --name clickhouse \
  -p 9000:9000 \
  -p 8123:8123 \
  clickhouse/clickhouse-server

# Or install natively (Ubuntu/Debian)
sudo apt-get install clickhouse-server clickhouse-client
sudo systemctl start clickhouse-server
```

### 2. Initialize Database Schema

```bash
cd internal/storage/clickhouse/migrations
clickhouse-client < 001_consolidated_schema.sql
```

This creates:
- Main events table with automatic deduplication
- Materialized views for optimized queries
- Analytics tables for reporting
- Performance indexes

### 3. Configure the Relay

```bash
# Copy example config
cp config.yaml.example config.yaml

# Edit config.yaml with your settings
nano config.yaml
```

Key configuration options:
- `server.listen`: Address to bind (e.g., `0.0.0.0:3334`)
- `server.domain`: Your relay's domain name
- `clickhouse.dsn`: ClickHouse connection string

### 4. Run the Relay

```bash
# From source
go run main.go

# Or build and run
go build -o nostr-relay
./nostr-relay
```

The relay will:
1. Load configuration from `config.yaml`
2. Connect to ClickHouse and verify schema
3. Start accepting WebSocket connections
4. Log statistics periodically

## Configuration

### Configuration File (config.yaml)

See [`config.yaml.example`](./config.yaml.example) for all available options.

### Environment Variables

Override config with environment variables:

```bash
export LISTEN="0.0.0.0:7777"
export DOMAIN="relay.mysite.com"
export CLICKHOUSE_DSN="clickhouse://remote-host:9000/nostr"

./nostr-relay
```

Supported variables:
- `LISTEN` - Server listen address
- `DOMAIN` - Relay domain name
- `CLICKHOUSE_DSN` - Database connection string
- `CONFIG_FILE` - Path to config file (default: `config.yaml`)

## Deployment

### Docker Compose

```yaml
version: '3.8'

services:
  clickhouse:
    image: clickhouse/clickhouse-server:latest
    ports:
      - "9000:9000"
      - "8123:8123"
    volumes:
      - clickhouse-data:/var/lib/clickhouse
    environment:
      CLICKHOUSE_DB: nostr

  relay:
    build: .
    ports:
      - "3334:3334"
      - "8080:8080"  # Health check
    depends_on:
      - clickhouse
    environment:
      CLICKHOUSE_DSN: "clickhouse://clickhouse:9000/nostr"
      DOMAIN: "relay.example.com"
    volumes:
      - ./config.yaml:/app/config.yaml
    restart: unless-stopped

volumes:
  clickhouse-data:
```

Run with:
```bash
docker-compose up -d
```

### Systemd Service

Create `/etc/systemd/system/nostr-relay.service`:

```ini
[Unit]
Description=Nostr Relay with ClickHouse
After=network.target clickhouse-server.service

[Service]
Type=simple
User=nostr
Group=nostr
WorkingDirectory=/opt/nostr-relay
ExecStart=/opt/nostr-relay/nostr-relay
Restart=on-failure
RestartSec=10

# Environment
Environment="CLICKHOUSE_DSN=clickhouse://localhost:9000/nostr"
Environment="DOMAIN=relay.example.com"

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/nostr-relay

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl daemon-reload
sudo systemctl enable nostr-relay
sudo systemctl start nostr-relay
sudo systemctl status nostr-relay
```

### Building for Production

Build optimized binary:
```bash
go build -ldflags="-s -w -X main.version=1.0.0 -X main.gitCommit=$(git rev-parse HEAD) -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o nostr-relay
```

Cross-compile for Linux:
```bash
GOOS=linux GOARCH=amd64 go build -o nostr-relay-linux-amd64
```

## Monitoring

### Health Check

The relay exposes a health check endpoint:

```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "healthy",
  "storage": "connected",
  "uptime": "2h15m30s"
}
```

### Statistics

The relay logs statistics periodically:

```
Relay Statistics:
  Connected clients: 42
  Active subscriptions: 128
  Queue load: 15.3%
  Storage events: 1,234,567 (2.45 GB)
```

### ClickHouse Monitoring

Query storage directly:

```sql
-- Recent events
SELECT count()
FROM nostr.events
WHERE relay_received_at >= now() - INTERVAL 1 MINUTE;

-- Top authors
SELECT pubkey, count() as events
FROM nostr.events FINAL
WHERE created_at >= now() - INTERVAL 1 DAY
GROUP BY pubkey
ORDER BY events DESC
LIMIT 10;

-- Storage size by table
SELECT
    table,
    formatReadableSize(sum(bytes)) as size,
    sum(rows) as rows
FROM system.parts
WHERE database = 'nostr' AND active = 1
GROUP BY table;
```

## Performance

Expected performance on modern hardware (32 cores, 128GB RAM, NVMe SSD):

### Write Performance
- **Event ingestion**: 50,000-200,000 events/second
- **Batch insertion**: 50-200ms per 1,000 events
- **Single event latency**: 0.5-2ms

### Read Performance
- **Query by ID**: 1-5ms
- **Query by author**: 10-50ms
- **Complex multi-filter**: 50-500ms
- **Query throughput**: 1,000-10,000 queries/sec

### Storage Efficiency
- **Compression ratio**: 70-85% (ClickHouse compression)
- **Index overhead**: ~5-10% of data size

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                         Nostr Clients                         │
│                   (WebSocket Connections)                     │
└────────────────────────────┬─────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────┐
│                    Nostr Relay (main.go)                      │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  rely Library (WebSocket, NIP handling, routing)       │  │
│  └────────────────────────────────────────────────────────┘  │
│                             │                                 │
│                             ▼                                 │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  ClickHouse Storage (internal/storage/clickhouse)     │  │
│  │  - Batch insertion                                     │  │
│  │  - Query optimization                                  │  │
│  │  - Analytics                                           │  │
│  └────────────────────────────────────────────────────────┘  │
└────────────────────────────┬─────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────┐
│                       ClickHouse Database                     │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────────┐  │
│  │   Events    │  │ Materialized │  │   Analytics        │  │
│  │   Table     │  │    Views     │  │    Tables          │  │
│  └─────────────┘  └──────────────┘  └────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

## Troubleshooting

### Relay won't start

**Error:** `Failed to initialize ClickHouse storage`

**Solution:** Verify ClickHouse is running and DSN is correct:
```bash
clickhouse-client --query "SELECT 1"
```

### Events not being saved

**Error:** `batch insert error`

**Solution:** Check ClickHouse logs and ensure schema is initialized:
```bash
clickhouse-client --query "SHOW TABLES FROM nostr"
```

### High memory usage

**Solution:** Reduce batch size in config:
```yaml
clickhouse:
  batch_size: 500  # Reduced from 1000
```

### Slow queries

**Solution:** Check table sizes and optimize:
```sql
-- Force merge to optimize storage
OPTIMIZE TABLE nostr.events FINAL;
OPTIMIZE TABLE nostr.events_by_author FINAL;
```

## Development

### Running Tests

```bash
# Unit tests
go test ./...

# Integration tests (requires ClickHouse)
CLICKHOUSE_DSN="clickhouse://localhost:9000/nostr_test" go test -v ./internal/storage/clickhouse

# Functional tests
go test -v -tags=functional ./internal/storage/clickhouse
```

### Code Structure

```
cmd/nostr-relay/
├── main.go                           # Entry point
├── config/
│   └── config.go                     # Configuration management
├── internal/
│   └── storage/
│       └── clickhouse/               # ClickHouse storage implementation
│           ├── storage.go           # Main storage interface
│           ├── insert.go            # Event insertion
│           ├── query.go             # Event queries
│           ├── count.go             # Event counting
│           ├── analytics.go         # Analytics queries
│           └── migrations/          # Database schemas
└── README.md
```

## Security Considerations

1. **Network Security**: Use TLS/SSL for WebSocket connections in production
2. **Database Access**: Restrict ClickHouse access with firewall rules
3. **Rate Limiting**: Consider adding rate limiting for public relays
4. **Event Validation**: All events are validated before storage
5. **SQL Injection**: All queries use parameterized statements (no SQL injection possible)

## Maintenance

### Backup

```bash
# Backup ClickHouse data
clickhouse-client --query "ALTER TABLE nostr.events FREEZE WITH NAME 'backup_$(date +%Y%m%d)'"

# Copy to backup location
rsync -av /var/lib/clickhouse/shadow/ /backup/clickhouse/
```

### Restore

```bash
# Stop relay
sudo systemctl stop nostr-relay

# Restore from backup
rsync -av /backup/clickhouse/ /var/lib/clickhouse/shadow/

# Attach from backup
clickhouse-client --query "ALTER TABLE nostr.events ATTACH PARTITION FROM 'backup_20240101'"

# Start relay
sudo systemctl start nostr-relay
```

### Cleanup Old Data

```sql
-- Drop partitions older than 1 year
ALTER TABLE nostr.events DROP PARTITION '202301';
```

## License

Same as rely - see [LICENSE](../../LICENSE)

## Support

- **Issues**: [GitHub Issues](https://github.com/nostr-net/rely/issues)
- **Rely Documentation**: [docs.rely.io](https://docs.rely.io)
- **ClickHouse Docs**: [clickhouse.com/docs](https://clickhouse.com/docs)
- **Nostr NIPs**: [github.com/nostr-protocol/nips](https://github.com/nostr-protocol/nips)

## Contributing

Contributions welcome! Please read [CONTRIBUTING.md](../../CONTRIBUTING.md) first.
