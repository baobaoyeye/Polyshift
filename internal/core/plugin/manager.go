package plugin

import (
	"fmt"
	"log"
	"sync"

	"github.com/polyshift/microkernel/internal/core/config"
)

type Manager struct {
	plugins map[string]*PluginInstance
	mu      sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		plugins: make(map[string]*PluginInstance),
	}
}

func (m *Manager) LoadPlugins(configs []config.PluginConfig) error {
	for _, cfg := range configs {
		if err := m.StartPlugin(cfg); err != nil {
			log.Printf("Failed to load plugin %s: %v", cfg.Name, err)
			continue
		}
	}
	return nil
}

func (m *Manager) StartPlugin(cfg config.PluginConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.plugins[cfg.Name]; exists {
		return fmt.Errorf("plugin %s already exists", cfg.Name)
	}

	instance := NewPluginInstance(cfg)
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
		log.Printf("Error stopping plugin %s: %v", name, err)
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
