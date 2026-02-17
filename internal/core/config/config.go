package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Auth      AuthConfig      `yaml:"auth"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Resilience ResilienceConfig `yaml:"resilience"`
	Observability ObservabilityConfig `yaml:"observability"`
	Plugins   []PluginConfig  `yaml:"plugins"`
}

type ObservabilityConfig struct {
	Logging struct {
		Level  string `yaml:"level"`  // debug, info, warn, error
		Format string `yaml:"format"` // json, text
	} `yaml:"logging"`
	Tracing struct {
		Enabled      bool    `yaml:"enabled"`
		SamplingRate float64 `yaml:"sampling_rate"` // 0.0 - 1.0
		Exporter     string  `yaml:"exporter"`      // stdout, otlp
	} `yaml:"tracing"`
	Metrics struct {
		Enabled bool `yaml:"enabled"`
		Port    int  `yaml:"port"`
	} `yaml:"metrics"`
}

type ResilienceConfig struct {
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
	Watchdog       WatchdogConfig       `yaml:"watchdog"`
}

type CircuitBreakerConfig struct {
	Enabled          bool    `yaml:"enabled"`
	MaxRequests      uint32  `yaml:"max_requests"`        // Half-open state max requests
	Interval         string  `yaml:"interval"`            // Cyclic period of the closed state
	Timeout          string  `yaml:"timeout"`             // Open state timeout
	ReadyToTripRatio float64 `yaml:"ready_to_trip_ratio"` // Error ratio to trip (0.0 - 1.0)
}

type WatchdogConfig struct {
	Enabled    bool   `yaml:"enabled"`
	Interval   string `yaml:"interval"`    // Check interval
	MaxRetries int    `yaml:"max_retries"` // Max restart retries
	BaseDelay  string `yaml:"base_delay"`  // Base delay for backoff
	MaxDelay   string `yaml:"max_delay"`   // Max delay for backoff
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

type AuthConfig struct {
	Enabled bool   `yaml:"enabled"`
	APIKey  string `yaml:"api_key"`
}

type RateLimitConfig struct {
	Enabled bool    `yaml:"enabled"`
	QPS     float64 `yaml:"qps"`
	Burst   int     `yaml:"burst"`
}

type PluginConfig struct {
	Name       string            `yaml:"name"`
	Version    string            `yaml:"version"`
	Runtime    string            `yaml:"runtime"`
	Entrypoint string            `yaml:"entrypoint"`
	Params     map[string]string `yaml:"params"`
	Routes     []RouteConfig     `yaml:"routes"`
}

type RouteConfig struct {
	Path    string `yaml:"path"`
	Method  string `yaml:"method"`
	Handler string `yaml:"handler"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
