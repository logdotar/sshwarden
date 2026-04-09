// Package config 提供配置管理功能
package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	// FirewallTypeIptables 表示使用 iptables 防火墙
	FirewallTypeIptables = "iptables"
	// FirewallTypeFirewallCmd 表示使用 firewall-cmd 防火墙
	FirewallTypeFirewallCmd = "firewall-cmd"
)

// Config 配置结构体
//
// 字段说明：
// - SSH: SSH 相关配置
// - Log: 日志相关配置
// - Firewall: 防火墙相关配置
// - IP: IP 相关配置
// - Alert: 告警相关配置

type Config struct {
	SSH      SSHConfig      `mapstructure:"ssh"`
	Log      LogConfig      `mapstructure:"log"`
	Firewall FirewallConfig `mapstructure:"firewall"`
	IP       IPConfig       `mapstructure:"ip"`
	Alert    AlertConfig    `mapstructure:"alert"`
}

// SSHConfig SSH 相关配置
//
// 字段说明：
// - LogPath: SSH 日志文件路径
// - MaxFailures: 最大失败次数
// - BlockedIPsFile: 被封禁 IP 记录文件
// - RegexPatterns: 正则表达式模式
// - IgnoreIPs: 忽略的 IP 列表
// - BanTime: 封禁时间
// - FindTime: 查找时间窗口
// - PermanentBlockIPs: 永久封禁的 IP 列表

type SSHConfig struct {
	LogPath           string   `mapstructure:"logpath"`
	MaxFailures       int      `mapstructure:"maxfailures"`
	BlockedIPsFile    string   `mapstructure:"blockedipsfile"`
	RegexPatterns     []string `mapstructure:"regex"`
	IgnoreIPs         []string `mapstructure:"ignoreip"`
	PermanentBlockIPs []string `mapstructure:"permanentblockip"`
	BanTime           string   `mapstructure:"bantime"`
	FindTime          string   `mapstructure:"findtime"`
}

// LogConfig 日志相关配置
//
// 字段说明：
// - Level: 日志级别
// - Filename: 日志文件名称
// - MaxSize: 日志文件最大大小
// - MaxBackups: 日志文件最大备份数
// - MaxAge: 日志文件最大保存天数
// - LocalTime: 是否使用本地时间
// - Compress: 是否压缩日志文件

type LogConfig struct {
	Level      string `mapstructure:"level"`
	Filename   string `mapstructure:"filename"`
	MaxSize    int    `mapstructure:"maxsize"`
	MaxBackups int    `mapstructure:"maxbackups"`
	MaxAge     int    `mapstructure:"maxage"`
	LocalTime  bool   `mapstructure:"localtime"`
	Compress   bool   `mapstructure:"compress"`
}

// FirewallConfig 防火墙相关配置
//
// 字段说明：
// - Type: 防火墙类型
// - ExportIptables: 是否导出 iptables 规则
// - LoadIptables: 是否加载 iptables 规则
// - UseIPSet: 是否使用 ipset 来提高性能（仅对 iptables 有效）
// - IPSetName: ipset 集合名称（仅对 iptables 有效）

type FirewallConfig struct {
	Type           string `mapstructure:"type"`
	ExportIptables bool   `mapstructure:"export_iptables_rules"`
	LoadIptables   bool   `mapstructure:"load_iptables_rules"`
	UseIPSet       bool   `mapstructure:"use_ipset"`
	IPSetName      string `mapstructure:"ipset_name"`
}

// IPConfig IP 相关配置
//
// 字段说明：
// - RegionDBPath: IP 归属地数据库路径

type IPConfig struct {
	RegionDBPath string `mapstructure:"region_db_path"`
}

// AlertConfig 告警相关配置
//
// 字段说明：
// - Email: 邮件告警配置

type AlertConfig struct {
	Email struct {
		Enabled  bool     `mapstructure:"enabled"`
		SMTPHost string   `mapstructure:"smtp_host"`
		SMTPPort string   `mapstructure:"smtp_port"`
		Username string   `mapstructure:"username"`
		Password string   `mapstructure:"password"`
		From     string   `mapstructure:"from"`
		To       []string `mapstructure:"to"`
		Subject  string   `mapstructure:"subject"`
	} `mapstructure:"email"`
}

// Manager 配置管理器
//
// 字段说明：
// - config: 配置信息
// - logger: 日志器

type Manager struct {
	config *Config
	logger *zap.Logger
}

// NewManager 创建配置管理器实例
//
// 参数:
// - logger: 日志器
//
// 返回值:
// - *Manager: 配置管理器实例
func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		config: &Config{},
		logger: logger,
	}
}

// Load 加载配置文件
//
// 参数:
// - configPath: 配置文件路径
//
// 返回值:
// - error: 加载过程中的错误
func (m *Manager) Load(configPath string) error {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("toml")

	if err := viper.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			return fmt.Errorf("配置文件未找到: %s", configPath)
		}
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	if err := viper.Unmarshal(m.config); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	m.setDefaults()

	if err := m.validate(); err != nil {
		return err
	}

	return nil
}

// Watch 监视配置文件变化
//
// 参数:
// - onChange: 配置变化时的回调函数
func (m *Manager) Watch(onChange func()) {
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		m.logger.Debug("配置文件已更改", zap.String("path", e.Name))
		if err := viper.Unmarshal(m.config); err != nil {
			m.logger.Error("重新加载配置失败", zap.Error(err))
			return
		}
		m.setDefaults()
		if onChange != nil {
			onChange()
		}
	})
}

// Get 获取配置信息
//
// 返回值:
// - *Config: 配置信息
func (m *Manager) Get() *Config {
	return m.config
}

// setDefaults 设置默认配置
func (m *Manager) setDefaults() {
	if m.config.SSH.BlockedIPsFile == "" {
		m.config.SSH.BlockedIPsFile = "blockedips.json"
	}
	if len(m.config.SSH.RegexPatterns) == 0 {
		m.config.SSH.RegexPatterns = []string{"authentication failure;.*rhost=(\\S+)"}
	}
	if m.config.Firewall.Type == "" {
		m.config.Firewall.Type = FirewallTypeIptables
	}
	if m.config.Firewall.IPSetName == "" {
		m.config.Firewall.IPSetName = "ssh_guardian_blacklist"
	}
	if m.config.Log.Level == "" {
		m.config.Log.Level = "info"
	}
	if m.config.Log.Filename == "" {
		m.config.Log.Filename = "sshwarden.log"
	}
	if m.config.Log.MaxSize == 0 {
		m.config.Log.MaxSize = 100
	}
	if m.config.Log.MaxBackups == 0 {
		m.config.Log.MaxBackups = 3
	}
	if m.config.Log.MaxAge == 0 {
		m.config.Log.MaxAge = 7
	}
}

// validate 验证配置
//
// 返回值:
// - error: 验证过程中的错误
func (m *Manager) validate() error {
	if m.config.SSH.LogPath == "" {
		return errors.New("ssh.logpath 配置项不能为空")
	}
	if m.config.SSH.MaxFailures <= 0 {
		return errors.New("ssh.maxfailures 配置项必须大于 0")
	}

	if _, err := zapcore.ParseLevel(m.config.Log.Level); err != nil {
		return fmt.Errorf("无效的日志级别配置: %s", m.config.Log.Level)
	}

	if m.config.Firewall.Type != FirewallTypeIptables && m.config.Firewall.Type != FirewallTypeFirewallCmd {
		return errors.New("firewall.type 配置项必须是 'iptables' 或 'firewall-cmd'")
	}

	return nil
}

// ParseBanTime 解析封禁时间
//
// 返回值:
// - time.Duration: 封禁时间
// - error: 解析过程中的错误
func (c *SSHConfig) ParseBanTime() (time.Duration, error) {
	return parseDuration(c.BanTime, -1)
}

// ParseFindTime 解析查找时间窗口
//
// 返回值:
// - time.Duration: 查找时间窗口
// - error: 解析过程中的错误
func (c *SSHConfig) ParseFindTime() (time.Duration, error) {
	return parseDuration(c.FindTime, 10*time.Minute)
}

// parseDuration 解析时间字符串
//
// 参数:
// - s: 时间字符串
// - defaultDuration: 默认时间
//
// 返回值:
// - time.Duration: 解析后的时间
// - error: 解析过程中的错误
func parseDuration(s string, defaultDuration time.Duration) (time.Duration, error) {
	if s == "" {
		return defaultDuration, nil
	}
	if s == "-1" {
		return -1, nil
	}

	s = strings.TrimSpace(s)
	if !strings.HasSuffix(s, "s") && !strings.HasSuffix(s, "m") &&
		!strings.HasSuffix(s, "h") && !strings.HasSuffix(s, "d") {
		s += "s"
	}

	if strings.HasSuffix(s, "d") {
		days := s[:len(s)-1]
		d, err := time.ParseDuration(days + "h")
		if err != nil {
			return 0, err
		}
		return d * 24, nil
	}

	return time.ParseDuration(s)
}
