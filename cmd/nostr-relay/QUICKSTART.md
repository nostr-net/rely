# Quick Start Guide

Get your Nostr relay running in 5 minutes!

## Option 1: Docker Compose (Recommended)

The easiest way to get started:

```bash
# 1. Navigate to relay directory
cd cmd/nostr-relay

# 2. Start everything (ClickHouse + Relay)
docker-compose up -d

# 3. Check logs
docker-compose logs -f relay

# 4. Test connection
websocat ws://localhost:3334
```

Your relay is now running on `ws://localhost:3334`!

## Option 2: Local Development

For development with hot reload:

```bash
# 1. Install ClickHouse
docker run -d -p 9000:9000 -p 8123:8123 clickhouse/clickhouse-server

# 2. Initialize database
cd cmd/nostr-relay
make init-db

# 3. Create config
cp config.yaml.example config.yaml

# 4. Run relay
make run
```

## Option 3: Production Binary

For production deployment:

```bash
# 1. Build optimized binary
cd cmd/nostr-relay
make build

# 2. Install (optional)
sudo make install

# 3. Setup systemd service
sudo make systemd-install

# 4. Start service
sudo systemctl start nostr-relay
sudo systemctl status nostr-relay
```

## Testing Your Relay

### Using websocat

```bash
# Install websocat
cargo install websocat

# Connect and subscribe
websocat ws://localhost:3334

# Send a REQ (paste this):
["REQ","sub1",{"kinds":[1],"limit":10}]
```

### Using nostr-tool

```bash
# Install nostr-tool
go install github.com/fiatjaf/nostr-tools/nostcat@latest

# Subscribe to events
nostcat -relay ws://localhost:3334 -kinds 1 -limit 10
```

### Using a Nostr Client

Configure your Nostr client (e.g., Damus, Amethyst, Snort) to use:
```
ws://localhost:3334
```

## Monitoring

### Check Relay Status

```bash
# View logs
docker-compose logs -f relay

# Check health
curl http://localhost:8080/health

# View stats in logs (every 30 seconds)
docker-compose logs relay | grep "Relay Statistics"
```

### Check ClickHouse

```bash
# Connect to ClickHouse
docker exec -it nostr-clickhouse clickhouse-client

# Query stats
SELECT count() FROM nostr.events;
SELECT kind, count() FROM nostr.events GROUP BY kind;
```

## Configuration

Edit `config.yaml` to customize:

```yaml
server:
  listen: "0.0.0.0:3334"
  domain: "your-relay.com"

clickhouse:
  dsn: "clickhouse://clickhouse:9000/nostr"
  batch_size: 1000
  flush_interval: 1s
```

Or use environment variables:
```bash
export LISTEN="0.0.0.0:7777"
export DOMAIN="relay.example.com"
./nostr-relay
```

## Next Steps

- Read the [full README](README.md) for detailed documentation
- Configure [NIP-11](https://github.com/nostr-protocol/nips/blob/master/11.md) relay information
- Set up TLS/SSL for production
- Configure rate limiting and anti-spam
- Monitor with Prometheus/Grafana

## Troubleshooting

### Can't connect to relay

```bash
# Check if relay is running
docker-compose ps

# Check relay logs
docker-compose logs relay

# Check if port is open
nc -zv localhost 3334
```

### Database errors

```bash
# Check ClickHouse is running
docker-compose ps clickhouse

# Reinitialize schema
make init-db
```

### High memory usage

Reduce batch size in `config.yaml`:
```yaml
clickhouse:
  batch_size: 500  # Lower value
```

## Getting Help

- 📖 [Full Documentation](README.md)
- 🐛 [Report Issues](https://github.com/nostr-net/rely/issues)
- 💬 [Community Support](https://github.com/nostr-net/rely/discussions)
