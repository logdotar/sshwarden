// Package banmanager 提供 IP 封禁管理功能
package banmanager

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"regexp"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/logdotar/sshwarden/internal/firewall"
)

// BlockedIP 表示被封禁的 IP 信息
//
// 字段说明：
// - IP: IP 地址
// - BannedAt: 封禁时间
// - ExpiresAt: 过期时间
// - Permanent: 是否为永久封禁

type BlockedIP struct {
	IP        string    `json:"ip"`
	BannedAt  time.Time `json:"banned_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Permanent bool      `json:"permanent"` // 标记是否为永久封禁
}

// BlockedIPs 表示被封禁的 IP 列表
//
// 字段说明：
// - IPs: 被封禁的 IP 列表

type BlockedIPs struct {
	IPs []BlockedIP `json:"ips"`
}

// WhitelistManager 白名单管理器接口
//
// 方法说明：
// - Contains: 检查 IP 是否在白名单中

type WhitelistManager interface {
	Contains(ip string) bool
}

// Failure 表示登录失败信息
//
// 字段说明：
// - Count: 失败次数
// - FirstSeen: 第一次失败时间
// - LastSeen: 最近一次失败时间

type Failure struct {
	Count     int       `json:"count"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

// Manager 封禁管理器
//
// 字段说明：
// - ipFailures: IP 失败记录
// - ipBlocked: 被封禁的 IP
// - regexPatterns: 正则表达式模式
// - blockedIPsFile: 封禁记录文件
// - findTime: 查找时间窗口
// - banTime: 封禁时间
// - mu: 互斥锁
// - logger: 日志器
// - firewall: 防火墙管理器

type Manager struct {
	ipFailures     map[string]Failure
	ipBlocked      map[string]BlockedIP
	regexPatterns  []*regexp.Regexp
	blockedIPsFile string
	findTime       time.Duration
	banTime        time.Duration
	mu             sync.RWMutex
	logger         *zap.Logger
	firewall       firewall.Manager
}

// NewManager 创建封禁管理器实例
//
// 参数:
// - blockedIPsFile: 封禁记录文件路径
// - findTime: 查找时间窗口
// - banTime: 封禁时间
// - firewall: 防火墙管理器
// - logger: 日志器
//
// 返回值:
// - *Manager: 封禁管理器实例
func NewManager(blockedIPsFile string, findTime, banTime time.Duration, firewall firewall.Manager, logger *zap.Logger) *Manager {
	return &Manager{
		ipFailures:     make(map[string]Failure),
		ipBlocked:      make(map[string]BlockedIP),
		regexPatterns:  make([]*regexp.Regexp, 0),
		blockedIPsFile: blockedIPsFile,
		findTime:       findTime,
		banTime:        banTime,
		logger:         logger,
		firewall:       firewall,
	}
}

// LoadRegexPatterns 加载正则表达式模式
//
// 参数:
// - patterns: 正则表达式模式列表
//
// 返回值:
// - error: 加载过程中的错误
func (m *Manager) LoadRegexPatterns(patterns []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.regexPatterns = make([]*regexp.Regexp, 0)
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			m.logger.Error("编译正则表达式失败", zap.String("pattern", pattern), zap.Error(err))
			continue
		}
		m.regexPatterns = append(m.regexPatterns, re)
	}

	return nil
}

// ExtractIP 从日志行中提取 IP 地址
//
// 参数:
// - logLine: 日志行
//
// 返回值:
// - string: 提取的 IP 地址，空字符串表示未找到
func (m *Manager) ExtractIP(logLine string) string {
	m.mu.RLock()
	regexPatterns := m.regexPatterns
	m.mu.RUnlock()

	for _, re := range regexPatterns {
		matches := re.FindStringSubmatch(logLine)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	return ""
}

// IncrementFailure 增加 IP 的失败计数
//
// 参数:
// - ip: IP 地址
//
// 返回值:
// - int: 增加后的失败计数
func (m *Manager) IncrementFailure(ip string) int {
	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	failure, exists := m.ipFailures[ip]

	if !exists {
		// 第一次失败
		m.ipFailures[ip] = Failure{
			Count:     1,
			FirstSeen: now,
			LastSeen:  now,
		}
		return 1
	}

	// 检查是否在时间窗口内
	if now.Sub(failure.FirstSeen) > m.findTime {
		// 超出时间窗口，重置计数
		m.ipFailures[ip] = Failure{
			Count:     1,
			FirstSeen: now,
			LastSeen:  now,
		}
		return 1
	}

	// 在时间窗口内，增加计数
	failure.Count++
	failure.LastSeen = now
	m.ipFailures[ip] = failure

	return failure.Count
}

// ClearFailure 清除 IP 的失败记录
//
// 参数:
// - ip: IP 地址
func (m *Manager) ClearFailure(ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.ipFailures, ip)
}

// IsBlocked 检查 IP 是否已被封禁
//
// 参数:
// - ip: IP 地址
//
// 返回值:
// - bool: 是否已被封禁
func (m *Manager) IsBlocked(ip string) bool {
	// 先检查 IP 是否在内存的封禁记录中
	m.mu.RLock()
	blocked, exists := m.ipBlocked[ip]
	m.mu.RUnlock()

	// 检查封禁是否过期
	if exists {
		// 检查是否为永久封禁
		if blocked.Permanent {
			// 对于永久封禁，验证文件中的记录是否仍然存在
			if err := m.LoadBlockedIPs(); err != nil {
				m.logger.Debug("重新加载封禁记录失败", zap.Error(err))
			}

			// 重新检查 IP 是否在封禁记录中
			m.mu.RLock()
			blocked, exists = m.ipBlocked[ip]
			m.mu.RUnlock()

			return exists && blocked.Permanent
		}

		// 检查封禁是否过期
		if time.Now().After(blocked.ExpiresAt) {
			// 封禁已过期，自动解除封禁
			m.unblockExpiredIP(ip)
			return false
		}

		// 对于临时封禁，验证文件中的记录是否仍然存在
		if err := m.LoadBlockedIPs(); err != nil {
			m.logger.Debug("重新加载封禁记录失败", zap.Error(err))
		}

		// 重新检查 IP 是否在封禁记录中
		m.mu.RLock()
		blocked, exists = m.ipBlocked[ip]
		m.mu.RUnlock()

		if !exists {
			return false
		}

		// 再次检查封禁是否过期
		if time.Now().After(blocked.ExpiresAt) {
			// 封禁已过期，自动解除封禁
			m.unblockExpiredIP(ip)
			return false
		}

		return true
	}

	// IP 不在内存的封禁记录中，尝试重新加载封禁记录
	// 这样当用户通过命令行解除封禁后，系统会自动更新内存中的记录
	if err := m.LoadBlockedIPs(); err != nil {
		m.logger.Debug("重新加载封禁记录失败", zap.Error(err))
	}

	// 再次检查 IP 是否在封禁记录中
	m.mu.RLock()
	blocked, exists = m.ipBlocked[ip]
	m.mu.RUnlock()

	if !exists {
		return false
	}

	// 检查是否为永久封禁
	if blocked.Permanent {
		return true
	}

	// 检查封禁是否过期
	if time.Now().After(blocked.ExpiresAt) {
		// 封禁已过期，自动解除封禁
		m.unblockExpiredIP(ip)
		return false
	}

	return true
}

// BlockIP 封禁 IP 或 CIDR 网段
//
// 参数:
// - ip: IP 地址或 CIDR 网段
//
// 返回值:
// - error: 封禁过程中的错误
func (m *Manager) BlockIP(ip string) error {
	// 验证 IP 地址格式（支持单个 IP 和 CIDR 网段）
	if net.ParseIP(ip) == nil {
		// 尝试解析为 CIDR 网段
		_, _, err := net.ParseCIDR(ip)
		if err != nil {
			return fmt.Errorf("无效的 IP 地址或 CIDR 网段: %s", ip)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	var expiresAt time.Time

	if m.banTime == -1 {
		// 永久封禁
		expiresAt = time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)
	} else {
		expiresAt = now.Add(m.banTime)
	}

	blockedIP := BlockedIP{
		IP:        ip,
		BannedAt:  now,
		ExpiresAt: expiresAt,
		Permanent: m.banTime == -1, // 根据 banTime 决定是否为永久封禁
	}

	m.ipBlocked[ip] = blockedIP
	delete(m.ipFailures, ip)

	if err := m.saveBlockedIP(blockedIP); err != nil {
		m.logger.Error("保存封禁IP记录失败", zap.String("ip", ip), zap.Error(err))
		return err
	}

	return nil
}

// GetBlockedIPs 获取所有被封禁的 IP 地址
//
// 返回值:
// - []string: 被封禁的 IP 地址列表
func (m *Manager) GetBlockedIPs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ips := make([]string, 0, len(m.ipBlocked))
	for ip := range m.ipBlocked {
		ips = append(ips, ip)
	}
	return ips
}

// GetBlockedIPDetails 返回所有被封禁IP的详细信息
//
// 返回值:
// - []BlockedIP: 被封禁的 IP 详细信息列表
func (m *Manager) GetBlockedIPDetails() []BlockedIP {
	m.mu.RLock()
	defer m.mu.RUnlock()

	blockedIPs := make([]BlockedIP, 0, len(m.ipBlocked))
	for _, blockedIP := range m.ipBlocked {
		blockedIPs = append(blockedIPs, blockedIP)
	}
	return blockedIPs
}

// CleanupExpired 清理过期的封禁记录
//
// 返回值:
// - int: 清理的过期记录数量
func (m *Manager) CleanupExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	expiredCount := 0

	for ip, blockedIP := range m.ipBlocked {
		// 检查是否过期（即使 banTime == -1，也检查过期时间，以防配置已更改）
		// 但永久封禁的 IP 不应该被清理
		if !blockedIP.Permanent && m.banTime != -1 && now.After(blockedIP.ExpiresAt) {
			// 从防火墙中解除封禁
			if m.firewall != nil {
				if err := m.firewall.UnblockIP(ip); err != nil {
					m.logger.Error("从防火墙解除封禁失败", zap.String("ip", ip), zap.Error(err))
					// 继续处理，不中断清理过程
				}
			}

			delete(m.ipBlocked, ip)
			expiredCount++
		}
	}

	if expiredCount > 0 {
		// 保存更新后的封禁记录
		if err := m.saveAllBlockedIPs(); err != nil {
			m.logger.Error("保存更新后的封禁记录失败", zap.Error(err))
		}
	}

	return expiredCount
}

// LoadBlockedIPs 加载被封禁的 IP 记录
//
// 返回值:
// - error: 加载过程中的错误
func (m *Manager) LoadBlockedIPs() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var blockedIPs BlockedIPs

	data, err := os.ReadFile(m.blockedIPsFile)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，清空封禁记录
			m.ipBlocked = make(map[string]BlockedIP)
			m.logger.Debug("已封禁的IP文件不存在，清空封禁记录")
			return nil
		}
		m.logger.Error("读取已封禁的IP文件失败", zap.Error(err))
		return err
	}

	if err := json.Unmarshal(data, &blockedIPs); err != nil {
		m.logger.Error("解析已封禁的IP文件失败", zap.Error(err))
		return err
	}

	// 清空现有的封禁记录，重新加载
	m.ipBlocked = make(map[string]BlockedIP)

	// 清理过期的封禁记录并更新过期时间
	now := time.Now()
	updated := false
	for _, blockedIP := range blockedIPs.IPs {
		// 检查是否过期
		if m.banTime != -1 && !blockedIP.Permanent && now.After(blockedIP.ExpiresAt) {
			m.logger.Debug("跳过已过期的封禁记录", zap.String("ip", blockedIP.IP))
			continue
		}
		// 兼容旧格式，将过期时间为 9999 年的记录标记为永久封禁
		if !blockedIP.Permanent && blockedIP.ExpiresAt.Year() == 9999 {
			blockedIP.Permanent = true
			updated = true
		}
		// 如果 banTime 从 -1 变为非 -1，更新过期时间
		if m.banTime != -1 && !blockedIP.Permanent && blockedIP.ExpiresAt.Year() == 9999 {
			blockedIP.ExpiresAt = now.Add(m.banTime)
			updated = true
		}
		m.ipBlocked[blockedIP.IP] = blockedIP
	}

	// 如果有更新，保存到文件
	if updated && len(m.ipBlocked) > 0 {
		if err := m.saveAllBlockedIPs(); err != nil {
			m.logger.Error("保存更新后的封禁记录失败", zap.Error(err))
		}
	}

	return nil
}

// LoadPermanentBlockIPs 加载配置文件中的永久封禁 IP 列表
//
// 参数:
// - permanentBlockIPs: 永久封禁 IP 列表
// - whitelist: 白名单管理器，用于检查 IP 是否在白名单中
func (m *Manager) LoadPermanentBlockIPs(permanentBlockIPs []string, whitelist WhitelistManager) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, ip := range permanentBlockIPs {
		// 验证 IP 地址格式（支持单个 IP 和 CIDR 网段）
		if net.ParseIP(ip) == nil {
			// 尝试解析为 CIDR 网段
			_, _, err := net.ParseCIDR(ip)
			if err != nil {
				m.logger.Warn("无效的 IP 地址或 CIDR 网段，跳过永久封禁", zap.String("ip", ip))
				continue
			}
		}

		// 检查 IP 是否在白名单中
		if whitelist != nil && whitelist.Contains(ip) {
			m.logger.Warn("IP 在白名单中，跳过永久封禁", zap.String("ip", ip))
			continue
		}

		// 检查 IP 是否已被封禁
		if _, exists := m.ipBlocked[ip]; exists {
			// 如果已被封禁，检查是否为永久封禁
			blockedIP := m.ipBlocked[ip]
			if !blockedIP.Permanent {
				// 如果不是永久封禁，更新为永久封禁
				blockedIP.Permanent = true
				blockedIP.ExpiresAt = time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)
				m.ipBlocked[ip] = blockedIP
				// 从防火墙中封禁 IP
				if m.firewall != nil {
					if err := m.firewall.BlockIP(ip); err != nil {
						m.logger.Error("封禁 IP 失败", zap.String("ip", ip), zap.Error(err))
						// 继续处理，不中断封禁过程
					}
				}
				if err := m.saveBlockedIP(blockedIP); err != nil {
					m.logger.Error("保存封禁 IP 记录失败", zap.String("ip", ip), zap.Error(err))
				}
				m.logger.Info("已将 IP 更新为永久封禁", zap.String("ip", ip))
			}
		} else {
			// 如果未被封禁，直接永久封禁
			now := time.Now()
			expiresAt := time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)

			blockedIP := BlockedIP{
				IP:        ip,
				BannedAt:  now,
				ExpiresAt: expiresAt,
				Permanent: true,
			}

			m.ipBlocked[ip] = blockedIP
			delete(m.ipFailures, ip)

			// 从防火墙中封禁 IP
			if m.firewall != nil {
				if err := m.firewall.BlockIP(ip); err != nil {
					m.logger.Error("封禁 IP 失败", zap.String("ip", ip), zap.Error(err))
					// 继续处理，不中断封禁过程
				}
			}

			if err := m.saveBlockedIP(blockedIP); err != nil {
				m.logger.Error("保存封禁 IP 记录失败", zap.String("ip", ip), zap.Error(err))
			}

			m.logger.Info("已永久封禁 IP", zap.String("ip", ip))
		}
	}
}

// saveBlockedIP 保存单个封禁记录
//
// 参数:
// - blockedIP: 封禁记录
//
// 返回值:
// - error: 保存过程中的错误
func (m *Manager) saveBlockedIP(blockedIP BlockedIP) error {
	var blockedIPs BlockedIPs

	data, err := os.ReadFile(m.blockedIPsFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &blockedIPs); err != nil {
			m.logger.Warn("解析已有的封禁IP文件失败，将创建新文件", zap.Error(err))
			blockedIPs = BlockedIPs{}
		}
	}

	// 检查是否已经存在
	exists := false
	for i, existingIP := range blockedIPs.IPs {
		if existingIP.IP == blockedIP.IP {
			// 更新现有记录
			blockedIPs.IPs[i] = blockedIP
			exists = true
			break
		}
	}

	if !exists {
		blockedIPs.IPs = append(blockedIPs.IPs, blockedIP)
	}

	data, err = json.MarshalIndent(blockedIPs, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(m.blockedIPsFile, data, 0644); err != nil {
		return err
	}

	return nil
}

// saveAllBlockedIPs 保存所有封禁记录
//
// 返回值:
// - error: 保存过程中的错误
func (m *Manager) saveAllBlockedIPs() error {
	blockedIPs := BlockedIPs{
		IPs: make([]BlockedIP, 0, len(m.ipBlocked)),
	}

	for _, blockedIP := range m.ipBlocked {
		blockedIPs.IPs = append(blockedIPs.IPs, blockedIP)
	}

	data, err := json.MarshalIndent(blockedIPs, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(m.blockedIPsFile, data, 0644); err != nil {
		return err
	}

	return nil
}

// unblockExpiredIP 解除过期的 IP 封禁（内部方法，不验证 IP 格式）
//
// 参数:
// - ip: IP 地址或 CIDR 网段
func (m *Manager) unblockExpiredIP(ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查 IP 是否仍在封禁记录中
	if _, exists := m.ipBlocked[ip]; !exists {
		return
	}

	// 从防火墙中解除封禁
	if m.firewall != nil {
		if err := m.firewall.UnblockIP(ip); err != nil {
			m.logger.Error("从防火墙解除过期封禁失败", zap.String("ip", ip), zap.Error(err))
			// 继续处理，不中断解除封禁过程
		}
	}

	// 从封禁记录中删除
	delete(m.ipBlocked, ip)

	// 保存更新后的封禁记录
	if err := m.saveAllBlockedIPs(); err != nil {
		m.logger.Error("保存更新后的封禁记录失败", zap.Error(err))
	}

	m.logger.Info("已自动解除过期 IP 封禁", zap.String("ip", ip))
}

// UnblockIP 主动解除 IP 封禁
//
// 参数:
// - ip: IP 地址或 CIDR 网段
//
// 返回值:
// - error: 解除封禁过程中的错误
func (m *Manager) UnblockIP(ip string) error {
	// 验证 IP 地址格式（支持单个 IP 和 CIDR 网段）
	if net.ParseIP(ip) == nil {
		// 尝试解析为 CIDR 网段
		_, _, err := net.ParseCIDR(ip)
		if err != nil {
			return fmt.Errorf("无效的 IP 地址或 CIDR 网段: %s", ip)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查 IP 是否已被封禁
	if _, exists := m.ipBlocked[ip]; !exists {
		return fmt.Errorf("IP %s 未被封禁", ip)
	}

	// 从防火墙中解除封禁
	if m.firewall != nil {
		if err := m.firewall.UnblockIP(ip); err != nil {
			m.logger.Error("从防火墙解除封禁失败", zap.String("ip", ip), zap.Error(err))
			// 继续处理，不中断解除封禁过程
		}
	}

	// 从封禁记录中删除
	delete(m.ipBlocked, ip)

	// 保存更新后的封禁记录
	if err := m.saveAllBlockedIPs(); err != nil {
		m.logger.Error("保存更新后的封禁记录失败", zap.Error(err))
		return err
	}

	m.logger.Info("已主动解除 IP 封禁", zap.String("ip", ip))
	return nil
}

// BlockIPPermanently 永久封禁 IP
//
// 参数:
// - ip: IP 地址或 CIDR 网段
// - whitelist: 白名单管理器，用于检查 IP 是否在白名单中
//
// 返回值:
// - error: 封禁过程中的错误
func (m *Manager) BlockIPPermanently(ip string, whitelist WhitelistManager) error {
	// 验证 IP 地址格式（支持单个 IP 和 CIDR 网段）
	if net.ParseIP(ip) == nil {
		// 尝试解析为 CIDR 网段
		_, _, err := net.ParseCIDR(ip)
		if err != nil {
			return fmt.Errorf("无效的 IP 地址或 CIDR 网段: %s", ip)
		}
	}

	// 检查 IP 是否在白名单中
	if whitelist != nil && whitelist.Contains(ip) {
		return fmt.Errorf("IP 在白名单中，无法永久封禁: %s", ip)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	// 永久封禁，设置过期时间为 9999 年
	expiresAt := time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)

	blockedIP := BlockedIP{
		IP:        ip,
		BannedAt:  now,
		ExpiresAt: expiresAt,
		Permanent: true, // 标记为永久封禁
	}

	m.ipBlocked[ip] = blockedIP
	delete(m.ipFailures, ip)

	// 从防火墙中封禁 IP
	if m.firewall != nil {
		if err := m.firewall.BlockIP(ip); err != nil {
			m.logger.Error("封禁 IP 失败", zap.String("ip", ip), zap.Error(err))
			// 继续处理，不中断封禁过程
		}
	}

	if err := m.saveBlockedIP(blockedIP); err != nil {
		m.logger.Error("保存封禁 IP 记录失败", zap.String("ip", ip), zap.Error(err))
		return err
	}

	m.logger.Info("已永久封禁 IP", zap.String("ip", ip))
	return nil
}

// UpdateTimeSettings 更新时间设置
//
// 参数:
// - findTime: 查找时间窗口
// - banTime: 封禁时间
func (m *Manager) UpdateTimeSettings(findTime, banTime time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldBanTime := m.banTime
	m.findTime = findTime
	m.banTime = banTime

	// 当 banTime 从非 -1 变为 -1 时，更新所有被封禁 IP 的过期时间
	if oldBanTime != -1 && banTime == -1 {
		for ip, blockedIP := range m.ipBlocked {
			blockedIP.ExpiresAt = time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)
			blockedIP.Permanent = true // 标记为永久封禁
			m.ipBlocked[ip] = blockedIP
		}
		// 保存更新后的封禁记录
		if err := m.saveAllBlockedIPs(); err != nil {
			m.logger.Error("保存更新后的封禁记录失败", zap.Error(err))
		}
	} else if oldBanTime == -1 && banTime != -1 {
		// 当 banTime 从 -1 变为非 -1 时，更新所有被封禁 IP 的过期时间
		// 但保持永久封禁的 IP 不变
		now := time.Now()
		updated := false
		for ip, blockedIP := range m.ipBlocked {
			// 检查是否为永久封禁
			if !blockedIP.Permanent {
				blockedIP.ExpiresAt = now.Add(banTime)
				m.ipBlocked[ip] = blockedIP
				updated = true
			}
		}
		// 如果有更新，保存封禁记录
		if updated {
			if err := m.saveAllBlockedIPs(); err != nil {
				m.logger.Error("保存更新后的封禁记录失败", zap.Error(err))
			}
		}
	} else if oldBanTime != -1 && banTime != -1 && oldBanTime != banTime {
		// 当 banTime 从一个非 -1 值变为另一个非 -1 值时，更新所有非永久封禁 IP 的过期时间
		now := time.Now()
		updated := false
		for ip, blockedIP := range m.ipBlocked {
			// 检查是否为永久封禁
			if !blockedIP.Permanent {
				// 计算剩余的封禁时间
				remainingTime := blockedIP.ExpiresAt.Sub(now)
				if remainingTime > 0 {
					// 如果还有剩余封禁时间，按比例调整
					// 例如：原来剩余 3 分钟，banTime 从 5 分钟变为 10 分钟，则新的剩余时间为 6 分钟
					ratio := float64(banTime) / float64(oldBanTime)
					newRemainingTime := time.Duration(float64(remainingTime) * ratio)
					blockedIP.ExpiresAt = now.Add(newRemainingTime)
				} else {
					// 如果已经过期，设置为新的 banTime
					blockedIP.ExpiresAt = now.Add(banTime)
				}
				m.ipBlocked[ip] = blockedIP
				updated = true
			}
		}
		// 如果有更新，保存封禁记录
		if updated {
			if err := m.saveAllBlockedIPs(); err != nil {
				m.logger.Error("保存更新后的封禁记录失败", zap.Error(err))
			}
		}
	}

	m.logger.Info("时间设置已更新", zap.Duration("findTime", findTime), zap.Duration("banTime", banTime))
}
