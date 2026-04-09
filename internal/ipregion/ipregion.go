// Package ipregion 提供 IP 归属地查询功能
package ipregion

import (
	"fmt"
	"net"

	"github.com/logdotar/sshwarden/internal/config"

	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
	"go.uber.org/zap"
)

// Manager IP 归属地管理器
//
// 字段说明：
// - searcher: IP 归属地搜索器
// - logger: 日志器
// - dbPath: 数据库文件路径

type Manager struct {
	searcher *xdb.Searcher
	logger   *zap.Logger
	dbPath   string
}

// NewManager 创建 IP 归属地管理器实例
//
// 参数:
// - cfg: IP 配置
// - logger: 日志器
//
// 返回值:
// - *Manager: IP 归属地管理器实例
func NewManager(cfg *config.IPConfig, logger *zap.Logger) *Manager {
	return &Manager{
		logger: logger,
		dbPath: cfg.RegionDBPath,
	}
}

// Init 初始化 IP 归属地管理器
//
// 返回值:
// - error: 初始化过程中的错误
func (m *Manager) Init() error {

	// 验证数据库文件的适应性
	err := xdb.VerifyFromFile(m.dbPath)
	if err != nil {
		m.logger.Error("ip2region数据库文件校验失败", zap.String("path", m.dbPath), zap.Error(err))
		return fmt.Errorf("ip2region数据库文件校验失败: %w", err)
	}

	m.logger.Debug("ip2region数据库文件校验成功，正在加载IP归属地数据库", zap.String("path", m.dbPath))

	file, err := xdb.LoadContentFromFile(m.dbPath)
	if err != nil {
		m.logger.Error("加载ip2region数据库文件失败", zap.String("path", m.dbPath), zap.Error(err))
		return fmt.Errorf("加载ip2region数据库失败: %w", err)
	}

	version := xdb.IPv4
	m.searcher, err = xdb.NewWithBuffer(version, file)
	if err != nil {
		m.logger.Error("初始化ip2region搜索器失败", zap.Error(err))
		return fmt.Errorf("创建ip2region searcher失败: %w", err)
	}

	m.logger.Info("IP归属地数据库加载成功", zap.String("path", m.dbPath))
	return nil
}

// GetRegion 获取 IP 归属地
//
// 参数:
// - ip: IP 地址
//
// 返回值:
// - string: IP 归属地
// - error: 查询过程中的错误
func (m *Manager) GetRegion(ip string) (string, error) {
	if m.searcher == nil {
		return "未知", fmt.Errorf("IP归属地查询器未初始化")
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		m.logger.Warn("非法IP地址", zap.String("ip", ip))
		return "unknown", fmt.Errorf("非法IP地址: %s", ip)
	}

	region, err := m.searcher.Search(ip)
	if err != nil {
		m.logger.Warn("查询IP归属地失败", zap.String("ip", ip), zap.Error(err))
		return "unknown", fmt.Errorf("查询IP归属地失败: %w", err)
	}

	return region, nil
}

// Close 关闭 IP 归属地管理器
func (m *Manager) Close() {
	if m.searcher != nil {
		m.searcher.Close()
		m.searcher = nil
		m.logger.Info("已释放IP归属地数据库资源")
	}
}
