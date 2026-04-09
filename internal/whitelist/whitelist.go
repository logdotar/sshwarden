// Package whitelist 提供白名单管理功能
package whitelist

import (
	"net"
	"strings"
	"sync"

	"go.uber.org/zap"
)

// Manager 白名单管理器
//
// 字段说明：
// - ips: IP 白名单
// - cidrs: CIDR 白名单
// - mu: 读写锁
// - logger: 日志器

type Manager struct {
	ips    map[string]bool
	cidrs  []*net.IPNet
	mu     sync.RWMutex
	logger *zap.Logger
}

// NewManager 创建白名单管理器实例
//
// 参数:
// - logger: 日志器
//
// 返回值:
// - *Manager: 白名单管理器实例
func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		ips:    make(map[string]bool),
		cidrs:  make([]*net.IPNet, 0),
		logger: logger,
	}
}

// Load 加载白名单
//
// 参数:
// - ignoreIPs: 忽略的 IP 列表
func (m *Manager) Load(ignoreIPs []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ips = make(map[string]bool)
	m.cidrs = make([]*net.IPNet, 0)

	for _, ip := range ignoreIPs {
		trimmedIP := strings.TrimSpace(ip)
		if _, network, err := net.ParseCIDR(trimmedIP); err == nil {
			m.cidrs = append(m.cidrs, network)
			m.logger.Debug("添加CIDR到白名单", zap.String("cidr", trimmedIP))
		} else if net.ParseIP(trimmedIP) != nil {
			m.ips[trimmedIP] = true
			m.logger.Debug("添加IP到白名单", zap.String("ip", trimmedIP))
		} else {
			m.logger.Warn("无效的IP或CIDR格式", zap.String("value", trimmedIP))
		}
	}
}

// Contains 检查 IP 是否在白名单中
//
// 参数:
// - ip: IP 地址
//
// 返回值:
// - bool: 是否在白名单中
func (m *Manager) Contains(ip string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	if m.ips[ip] {
		return true
	}

	for _, network := range m.cidrs {
		if network.Contains(parsedIP) {
			return true
		}
	}

	return false
}

// List 获取白名单列表
//
// 返回值:
// - []string: IP 白名单列表
// - []string: CIDR 白名单列表
func (m *Manager) List() ([]string, []string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ips := make([]string, 0, len(m.ips))
	for ip := range m.ips {
		ips = append(ips, ip)
	}

	cidrs := make([]string, 0, len(m.cidrs))
	for _, cidr := range m.cidrs {
		cidrs = append(cidrs, cidr.String())
	}

	return ips, cidrs
}
