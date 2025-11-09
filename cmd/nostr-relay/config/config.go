package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for the relay
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	ClickHouse ClickHouseConfig `yaml:"clickhouse"`
	Monitoring MonitoringConfig `yaml:"monitoring"`
	Limits     LimitsConfig     `yaml:"limits"`
}

// ServerConfig holds relay server configuration
type ServerConfig struct {
	Listen              string `yaml:"listen"`
	Domain              string `yaml:"domain"`
	QueueCapacity       int    `yaml:"queue_capacity"`
	MaxProcessors       int    `yaml:"max_processors"`
	ClientResponseLimit int    `yaml:"client_response_limit"`
}

// ClickHouseConfig holds ClickHouse database configuration
type ClickHouseConfig struct {
	DSN           string        `yaml:"dsn"`
	BatchSize     int           `yaml:"batch_size"`
	FlushInterval time.Duration `yaml:"flush_interval"`
	MaxOpenConns  int           `yaml:"max_open_conns"`
	MaxIdleConns  int           `yaml:"max_idle_conns"`
}

// MonitoringConfig holds monitoring and observability configuration
type MonitoringConfig struct {
	StatsInterval   time.Duration `yaml:"stats_interval"`
	HealthCheckPort int           `yaml:"health_check_port"`
	EnableMetrics   bool          `yaml:"enable_metrics"`
}

// LimitsConfig holds rate limiting and resource limits
type LimitsConfig struct {
	MaxEventSize      int `yaml:"max_event_size"`
	MaxSubscriptions  int `yaml:"max_subscriptions"`
	MaxFiltersPerSub  int `yaml:"max_filters_per_sub"`
	ConnectionTimeout int `yaml:"connection_timeout"`
}

// Default returns a Config with sensible defaults
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Listen:              "0.0.0.0:3334",
			Domain:              "localhost",
			QueueCapacity:       2048,
			MaxProcessors:       8,
			ClientResponseLimit: 500,
		},
		ClickHouse: ClickHouseConfig{
			DSN:           "clickhouse://localhost:9000/nostr",
			BatchSize:     1000,
			FlushInterval: 1 * time.Second,
			MaxOpenConns:  10,
			MaxIdleConns:  5,
		},
		Monitoring: MonitoringConfig{
			StatsInterval:   30 * time.Second,
			HealthCheckPort: 8080,
			EnableMetrics:   true,
		},
		Limits: LimitsConfig{
			MaxEventSize:      64 * 1024, // 64KB
			MaxSubscriptions:  20,
			MaxFiltersPerSub:  10,
			ConnectionTimeout: 300, // 5 minutes
		},
	}
}

// Load loads configuration from file or environment variables
func Load() (*Config, error) {
	// Start with defaults
	cfg := Default()

	// Check for config file
	configPath := os.Getenv("CONFIG_FILE")
	if configPath == "" {
		configPath = "config.yaml"
	}

	// Try to load config file if it exists
	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	// Override with environment variables
	cfg.applyEnvOverrides()

	return cfg, nil
}

// applyEnvOverrides applies environment variable overrides
func (c *Config) applyEnvOverrides() {
	if listen := os.Getenv("LISTEN"); listen != "" {
		c.Server.Listen = listen
	}
	if domain := os.Getenv("DOMAIN"); domain != "" {
		c.Server.Domain = domain
	}
	if dsn := os.Getenv("CLICKHOUSE_DSN"); dsn != "" {
		c.ClickHouse.DSN = dsn
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Server.Listen == "" {
		return fmt.Errorf("server.listen is required")
	}
	if c.Server.Domain == "" {
		return fmt.Errorf("server.domain is required")
	}
	if c.ClickHouse.DSN == "" {
		return fmt.Errorf("clickhouse.dsn is required")
	}
	if c.ClickHouse.BatchSize <= 0 {
		return fmt.Errorf("clickhouse.batch_size must be positive")
	}
	if c.ClickHouse.FlushInterval <= 0 {
		return fmt.Errorf("clickhouse.flush_interval must be positive")
	}
	if c.Server.QueueCapacity <= 0 {
		return fmt.Errorf("server.queue_capacity must be positive")
	}
	if c.Server.MaxProcessors <= 0 {
		return fmt.Errorf("server.max_processors must be positive")
	}
	return nil
}
