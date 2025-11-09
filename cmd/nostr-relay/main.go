package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pippellia-btc/rely"
	"github.com/pippellia-btc/rely/cmd/nostr-relay/config"
	"github.com/pippellia-btc/rely/cmd/nostr-relay/internal/storage/clickhouse"
)

const banner = `
╔═══════════════════════════════════════════════════════════════╗
║                                                               ║
║   ███╗   ██╗ ██████╗ ███████╗████████╗██████╗               ║
║   ████╗  ██║██╔═══██╗██╔════╝╚══██╔══╝██╔══██╗              ║
║   ██╔██╗ ██║██║   ██║███████╗   ██║   ██████╔╝              ║
║   ██║╚██╗██║██║   ██║╚════██║   ██║   ██╔══██╗              ║
║   ██║ ╚████║╚██████╔╝███████║   ██║   ██║  ██║              ║
║   ╚═╝  ╚═══╝ ╚═════╝ ╚══════╝   ╚═╝   ╚═╝  ╚═╝              ║
║                                                               ║
║          High-Performance Relay with ClickHouse              ║
║                                                               ║
╚═══════════════════════════════════════════════════════════════╝
`

var (
	version   = "1.0.0"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	// Print banner
	fmt.Println(banner)
	log.Printf("Version: %s | Build: %s | Commit: %s\n", version, buildTime, gitCommit)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	log.Printf("Configuration loaded successfully")
	log.Printf("  Listen: %s", cfg.Server.Listen)
	log.Printf("  Domain: %s", cfg.Server.Domain)
	log.Printf("  ClickHouse: %s", cfg.ClickHouse.DSN)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Printf("Received signal %s, initiating graceful shutdown...", sig)
		cancel()
	}()

	// Initialize ClickHouse storage
	log.Println("Initializing ClickHouse storage...")
	storage, err := clickhouse.NewStorage(clickhouse.Config{
		DSN:           cfg.ClickHouse.DSN,
		BatchSize:     cfg.ClickHouse.BatchSize,
		FlushInterval: cfg.ClickHouse.FlushInterval,
		MaxOpenConns:  cfg.ClickHouse.MaxOpenConns,
		MaxIdleConns:  cfg.ClickHouse.MaxIdleConns,
	})
	if err != nil {
		log.Fatalf("Failed to initialize ClickHouse storage: %v", err)
	}
	defer func() {
		log.Println("Closing storage...")
		storage.Close()
	}()

	// Verify storage connection
	if err := storage.Ping(ctx); err != nil {
		log.Fatalf("Failed to ping ClickHouse: %v", err)
	}
	log.Println("✓ ClickHouse connection verified")

	// Display storage statistics
	if stats, err := storage.Stats(); err == nil {
		log.Printf("Storage stats:")
		log.Printf("  Total events: %d", stats.TotalEvents)
		log.Printf("  Storage size: %.2f GB", float64(stats.TotalBytes)/(1<<30))
		if stats.OldestEvent > 0 && stats.NewestEvent > 0 {
			log.Printf("  Time range: %s - %s",
				time.Unix(int64(stats.OldestEvent), 0).Format(time.RFC3339),
				time.Unix(int64(stats.NewestEvent), 0).Format(time.RFC3339),
			)
		}
	}

	// Create relay with configuration
	log.Println("Initializing Nostr relay...")
	relay := rely.NewRelay(
		rely.WithDomain(cfg.Server.Domain),
		rely.WithQueueCapacity(cfg.Server.QueueCapacity),
		rely.WithMaxProcessors(cfg.Server.MaxProcessors),
		rely.WithClientResponseLimit(cfg.Server.ClientResponseLimit),
	)

	// Hook up storage
	relay.On.Event = storage.SaveEvent
	relay.On.Req = storage.QueryEvents
	relay.On.Count = storage.CountEvents

	// Connection lifecycle hooks
	relay.On.Connect = func(c rely.Client) {
		log.Printf("Client connected: %s", c.IP())
	}

	relay.On.Disconnect = func(c rely.Client) {
		duration := time.Since(c.ConnectedAt())
		log.Printf("Client disconnected: %s (duration: %s)", c.IP(), duration)
	}

	// Authentication hook (NIP-42)
	relay.On.Auth = func(c rely.Client) {
		log.Printf("Client authenticated: %s (pubkey: %s)", c.IP(), c.Pubkey())
	}

	// Start periodic statistics reporting
	if cfg.Monitoring.StatsInterval > 0 {
		go periodicStats(ctx, relay, storage, cfg.Monitoring.StatsInterval)
	}

	// Start HTTP health check endpoint if configured
	if cfg.Monitoring.HealthCheckPort > 0 {
		go startHealthCheck(ctx, cfg.Monitoring.HealthCheckPort, storage)
	}

	// Start relay server
	log.Printf("Starting Nostr relay on %s...", cfg.Server.Listen)
	log.Println("Ready to accept connections ✓")

	if err := relay.StartAndServe(ctx, cfg.Server.Listen); err != nil {
		log.Fatalf("Relay error: %v", err)
	}

	log.Println("Relay stopped gracefully")
}

// periodicStats reports relay statistics at regular intervals
func periodicStats(ctx context.Context, relay *rely.Relay, storage *clickhouse.Storage, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stats, err := storage.Stats()
			if err != nil {
				log.Printf("Failed to get storage stats: %v", err)
				continue
			}

			log.Printf("Relay Statistics:")
			log.Printf("  Connected clients: %d", relay.Clients())
			log.Printf("  Active subscriptions: %d", relay.Subscriptions())
			log.Printf("  Queue load: %.1f%%", relay.QueueLoad()*100)
			log.Printf("  Storage events: %d (%.2f GB)",
				stats.TotalEvents,
				float64(stats.TotalBytes)/(1<<30),
			)
		}
	}
}

// startHealthCheck starts a simple HTTP health check endpoint
func startHealthCheck(ctx context.Context, port int, storage *clickhouse.Storage) {
	// TODO: Implement HTTP health check endpoint
	// This would expose /health, /metrics endpoints
	log.Printf("Health check endpoint would start on port %d (not yet implemented)", port)
}
