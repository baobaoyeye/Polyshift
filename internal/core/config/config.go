package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Auth      AuthConfig      `yaml:"auth"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Plugins   []PluginConfig  `yaml:"plugins"`
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
