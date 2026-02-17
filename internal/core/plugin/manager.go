package plugin

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/polyshift/microkernel/internal/core/config"
)

type Manager struct {
	plugins       map[string]*PluginInstance
	resilienceCfg config.ResilienceConfig
	mu            sync.RWMutex
}

func NewManager(resilienceCfg config.ResilienceConfig) *Manager {
	return &Manager{
		plugins:       make(map[string]*PluginInstance),
		resilienceCfg: resilienceCfg,
	}
}

func (m *Manager) LoadPlugins(configs []config.PluginConfig) error {
	for _, cfg := range configs {
		if err := m.StartPlugin(cfg); err != nil {
			slog.Error("Failed to load plugin", "name", cfg.Name, "error", err)
			continue
		}
	}
	return nil
}

func (m *Manager) StartWatchdog(ctx context.Context) {
	if !m.resilienceCfg.Watchdog.Enabled {
		return
	}

	interval := parseDuration(m.resilienceCfg.Watchdog.Interval, 5*time.Second)
	baseDelay := parseDuration(m.resilienceCfg.Watchdog.BaseDelay, 1*time.Second)
	maxDelay := parseDuration(m.resilienceCfg.Watchdog.MaxDelay, 30*time.Second)
	maxRetries := m.resilienceCfg.Watchdog.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}

	backoff := NewBackoffStrategy(baseDelay, maxDelay)
	slog.Info("Watchdog started", "interval", interval, "maxRetries", maxRetries)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				slog.Info("Watchdog stopped")
				return
			case <-ticker.C:
				m.checkPlugins(backoff, maxRetries)
			}
		}
	}()
}

func (m *Manager) checkPlugins(backoff *BackoffStrategy, maxRetries int) {
	// Copy plugins to avoid holding lock during check
	m.mu.RLock()
	pluginsToCheck := make([]*PluginInstance, 0, len(m.plugins))
	for _, p := range m.plugins {
		pluginsToCheck = append(pluginsToCheck, p)
	}
	m.mu.RUnlock()

	for _, p := range pluginsToCheck {
		m.checkSinglePlugin(p, backoff, maxRetries)
	}
}

func (m *Manager) checkSinglePlugin(p *PluginInstance, backoff *BackoffStrategy, maxRetries int) {
	p.mu.Lock()

	// 1. Check if pending restart
	if !p.NextRestartTime.IsZero() {
		if time.Now().Before(p.NextRestartTime) {
			p.mu.Unlock()
			return
		}
		// Ready to restart
		p.NextRestartTime = time.Time{} // Clear

		// Ensure stopped before restart
		_ = p.stopInternal()

		// Unlock to allow Start to acquire lock
		p.mu.Unlock()

		slog.Info("Watchdog: Restarting plugin", "plugin", p.Config.Name, "attempt", p.RestartCount)
		err := p.Start()

		// Re-lock to update state
		p.mu.Lock()
		if err != nil {
			slog.Error("Watchdog: Failed to restart plugin", "plugin", p.Config.Name, "error", err)
			p.RestartCount++
			// Calculate delay based on new count
			delay := backoff.CalculateDelay(p.RestartCount)
			p.NextRestartTime = time.Now().Add(delay)
			slog.Info("Watchdog: Scheduled next restart", "plugin", p.Config.Name, "delay", delay, "attempt", p.RestartCount)
		} else {
			slog.Info("Watchdog: Plugin restarted successfully", "plugin", p.Config.Name)
			// Reset count on success
			p.RestartCount = 0
		}
		p.mu.Unlock()
		return
	}

	// 2. Check Health
	// Unlock to allow CheckHealth to run (it locks p.mu)
	p.mu.Unlock()

	healthy, err := p.CheckHealth()

	if !healthy {
		p.mu.Lock()
		slog.Warn("Watchdog: Plugin unhealthy", "plugin", p.Config.Name, "error", err)

		// Schedule restart
		if p.NextRestartTime.IsZero() { // Only if not already scheduled
			p.RestartCount++
			if p.RestartCount > maxRetries {
				slog.Error("Watchdog: Plugin reached max retries, stopping checks", "plugin", p.Config.Name, "maxRetries", maxRetries)
				// We don't schedule next restart, effectively giving up.
				// But we still stopped it.
				_ = p.stopInternal()
			} else {
				delay := backoff.CalculateDelay(p.RestartCount)
				p.NextRestartTime = time.Now().Add(delay)
				slog.Info("Watchdog: Scheduled restart", "plugin", p.Config.Name, "delay", delay, "attempt", p.RestartCount)
				// Stop the plugin to release resources
				_ = p.stopInternal()
			}
		}
		p.mu.Unlock()
	} else {
		// Healthy
		p.mu.Lock()
		if p.RestartCount > 0 {
			p.RestartCount = 0
		}
		p.mu.Unlock()
	}
}

func (m *Manager) StartPlugin(cfg config.PluginConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.plugins[cfg.Name]; exists {
		return fmt.Errorf("plugin %s already exists", cfg.Name)
	}

	instance := NewPluginInstance(cfg, m.resilienceCfg)
	if err := instance.Start(); err != nil {
		return err
	}

	m.plugins[cfg.Name] = instance
	return nil
}

func (m *Manager) StopPlugin(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, exists := m.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	if err := instance.Stop(); err != nil {
		slog.Error("Error stopping plugin", "plugin", name, "error", err)
	}

	delete(m.plugins, name)
	return nil
}

func (m *Manager) ReloadPlugin(name string) error {
	// 1. 获取现有配置
	m.mu.RLock()
	instance, exists := m.plugins[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	cfg := instance.Config

	// 2. 停止插件
	if err := m.StopPlugin(name); err != nil {
		return err
	}

	// 3. 重新启动
	return m.StartPlugin(cfg)
}

func (m *Manager) ListPlugins() []config.PluginConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	configs := make([]config.PluginConfig, 0, len(m.plugins))
	for _, p := range m.plugins {
		configs = append(configs, p.Config)
	}
	return configs
}

func (m *Manager) GetPlugin(name string) (*PluginInstance, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.plugins[name]
	return p, ok
}

func (m *Manager) CheckPluginHealth(name string) (bool, error) {
	m.mu.RLock()
	p, ok := m.plugins[name]
	m.mu.RUnlock()
	if !ok {
		return false, fmt.Errorf("plugin %s not found", name)
	}
	return p.CheckHealth()
}
