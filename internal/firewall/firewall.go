// Package firewall 提供防火墙管理功能
package firewall

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/logdotar/sshwarden/internal/config"

	"go.uber.org/zap"
)

// Manager 防火墙管理器接口
//
// 方法说明：
// - BlockIP: 封禁 IP
// - UnblockIP: 解除封禁 IP
// - RestoreRules: 恢复防火墙规则
// - Close: 关闭防火墙管理器

type Manager interface {
	BlockIP(ip string) error
	UnblockIP(ip string) error
	RestoreRules() error
	Close() error
}

// iptablesManager iptables 防火墙管理器
//
// 字段说明：
// - logger: 日志器
// - exportRules: 是否导出规则
// - rulesFilePath: 规则文件路径
// - useIPSet: 是否使用 ipset
// - ipSetName: ipset 集合名称

type iptablesManager struct {
	logger        *zap.Logger
	exportRules   bool
	rulesFilePath string
	useIPSet      bool
	ipSetName     string
}

// firewallCmdManager firewall-cmd 防火墙管理器
//
// 字段说明：
// - logger: 日志器
// - useZone: 是否使用 Zone 方式来提高性能
// - blockZone: 用于封禁的 Zone 名称

type firewallCmdManager struct {
	logger    *zap.Logger
	useZone   bool
	blockZone string
}

// NewManager 创建防火墙管理器实例
//
// 参数:
// - cfg: 防火墙配置
// - logger: 日志器
//
// 返回值:
// - Manager: 防火墙管理器实例
// - error: 创建过程中的错误
func NewManager(cfg *config.FirewallConfig, logger *zap.Logger) (Manager, error) {
	switch cfg.Type {
	case config.FirewallTypeIptables:
		manager := &iptablesManager{
			logger:        logger,
			exportRules:   cfg.ExportIptables,
			rulesFilePath: "/etc/iptables/rules.v4",
			useIPSet:      cfg.UseIPSet,
			ipSetName:     cfg.IPSetName,
		}

		// 如果启用了 ipset，初始化 ipset 集合
		if cfg.UseIPSet {
			if err := manager.initIPSet(); err != nil {
				logger.Warn("初始化 ipset 失败，将回退到普通 iptables 模式", zap.Error(err))
				manager.useIPSet = false
			}
		}

		return manager, nil
	case config.FirewallTypeFirewallCmd:
		manager := &firewallCmdManager{
			logger:    logger,
			useZone:   cfg.UseIPSet, // 复用 use_ipset 配置项来控制是否使用 Zone 方式
			blockZone: "block",      // 使用默认的 block zone
		}

		// 如果启用了 Zone 方式，初始化 block zone
		if cfg.UseIPSet {
			if err := manager.initBlockZone(); err != nil {
				logger.Warn("初始化 block zone 失败，将回退到富规则模式", zap.Error(err))
				manager.useZone = false
			}
		}

		return manager, nil
	default:
		return nil, fmt.Errorf("不支持的防火墙类型: %s", cfg.Type)
	}
}

// BlockIP 封禁 IP
//
// 参数:
// - ip: IP 地址或 CIDR 网段
//
// 返回值:
// - error: 封禁过程中的错误
func (m *iptablesManager) BlockIP(ip string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if m.useIPSet {
		// 使用 ipset 添加 IP 或网段
		cmd := exec.CommandContext(ctx, "ipset", "add", m.ipSetName, ip)
		if err := cmd.Run(); err != nil {
			m.logger.Error("使用 ipset 封禁 IP 失败", zap.String("ip", ip), zap.Error(err))
			return err
		}
		m.logger.Info("已使用 ipset 封禁 IP", zap.String("ip", ip))
	} else {
		// 使用普通 iptables 命令
		cmd := exec.CommandContext(ctx, "iptables", "-A", "INPUT", "-s", ip, "-j", "DROP")
		if err := cmd.Run(); err != nil {
			m.logger.Error("封禁IP失败", zap.String("ip", ip), zap.Error(err))
			return err
		}
		m.logger.Info("已封禁IP", zap.String("ip", ip))
	}

	if m.exportRules {
		if err := m.saveRules(); err != nil {
			m.logger.Error("保存iptables规则失败", zap.Error(err))
		}
	}

	return nil
}

// RestoreRules 恢复防火墙规则
//
// 返回值:
// - error: 恢复过程中的错误
func (m *iptablesManager) RestoreRules() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "iptables-restore", m.rulesFilePath)
	if err := cmd.Run(); err != nil {
		m.logger.Error("恢复iptables规则失败", zap.Error(err))
		return err
	}
	m.logger.Info("成功恢复iptables规则")
	return nil
}

// UnblockIP 解除封禁 IP
//
// 参数:
// - ip: IP 地址或 CIDR 网段
//
// 返回值:
// - error: 解除封禁过程中的错误
func (m *iptablesManager) UnblockIP(ip string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if m.useIPSet {
		// 使用 ipset 删除 IP 或网段
		cmd := exec.CommandContext(ctx, "ipset", "del", m.ipSetName, ip)
		if err := cmd.Run(); err != nil {
			m.logger.Error("使用 ipset 解除封禁 IP 失败", zap.String("ip", ip), zap.Error(err))
			return err
		}
		m.logger.Info("已使用 ipset 解除 IP 封禁", zap.String("ip", ip))
	} else {
		// 使用普通 iptables 命令
		cmd := exec.CommandContext(ctx, "iptables", "-D", "INPUT", "-s", ip, "-j", "DROP")
		if err := cmd.Run(); err != nil {
			m.logger.Error("解除封禁IP失败", zap.String("ip", ip), zap.Error(err))
			return err
		}
		m.logger.Info("已解除IP封禁", zap.String("ip", ip))
	}

	if m.exportRules {
		if err := m.saveRules(); err != nil {
			m.logger.Error("保存iptables规则失败", zap.Error(err))
		}
	}

	return nil
}

// Close 关闭防火墙管理器
//
// 返回值:
// - error: 关闭过程中的错误
func (m *iptablesManager) Close() error {
	return nil
}

// initIPSet 初始化 ipset 集合
//
// 返回值:
// - error: 初始化过程中的错误
func (m *iptablesManager) initIPSet() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 检查 ipset 是否存在
	checkCmd := exec.CommandContext(ctx, "ipset", "list", m.ipSetName)
	if err := checkCmd.Run(); err == nil {
		// ipset 集合已存在，直接返回
		m.logger.Debug("ipset 集合已存在", zap.String("name", m.ipSetName))
		return nil
	}

	// 创建 ipset 集合
	createCmd := exec.CommandContext(ctx, "ipset", "create", m.ipSetName, "hash:net")
	if err := createCmd.Run(); err != nil {
		return fmt.Errorf("创建 ipset 集合失败: %w", err)
	}

	// 添加 iptables 规则来使用 ipset
	addRuleCmd := exec.CommandContext(ctx, "iptables", "-A", "INPUT", "-m", "set", "--match-set", m.ipSetName, "src", "-j", "DROP")
	if err := addRuleCmd.Run(); err != nil {
		// 如果添加规则失败，删除创建的 ipset 集合
		deleteCmd := exec.CommandContext(ctx, "ipset", "destroy", m.ipSetName)
		_ = deleteCmd.Run()
		return fmt.Errorf("添加 iptables 规则失败: %w", err)
	}

	m.logger.Info("ipset 初始化成功", zap.String("name", m.ipSetName))
	return nil
}

// saveRules 保存 iptables 规则
//
// 返回值:
// - error: 保存过程中的错误
func (m *iptablesManager) saveRules() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "iptables-save")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("保存iptables规则失败: %w", err)
	}

	if err := os.WriteFile(m.rulesFilePath, output, 0644); err != nil {
		return fmt.Errorf("写入iptables规则文件失败: %w", err)
	}

	// 如果使用了 ipset，保存 ipset 规则
	if m.useIPSet {
		saveCmd := exec.CommandContext(ctx, "ipset", "save", m.ipSetName, "-f", "/etc/ipset/rules")
		if err := saveCmd.Run(); err != nil {
			m.logger.Warn("保存 ipset 规则失败", zap.Error(err))
		}
	}

	m.logger.Debug("iptables规则已保存", zap.String("path", m.rulesFilePath))
	return nil
}

// BlockIP 封禁 IP
//
// 参数:
// - ip: IP 地址或 CIDR 网段
//
// 返回值:
// - error: 封禁过程中的错误
func (m *firewallCmdManager) BlockIP(ip string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if m.useZone {
		// 使用 Zone 方式封禁 IP 或网段
		addSourceCmd := exec.CommandContext(ctx, "firewall-cmd", "--zone="+m.blockZone, "--add-source", ip, "--permanent")
		if err := m.executeCommand(addSourceCmd, "使用 Zone 封禁 IP", ip); err != nil {
			return err
		}
	} else {
		// 使用富规则方式封禁 IP 或网段
		addRuleCmd := exec.CommandContext(ctx, "firewall-cmd", "--zone=public", "--add-rich-rule",
			fmt.Sprintf(`rule family="ipv4" source address="%s" reject`, ip), "--permanent")
		if err := m.executeCommand(addRuleCmd, "封禁IP", ip); err != nil {
			return err
		}
	}

	reloadCmd := exec.CommandContext(ctx, "firewall-cmd", "--reload")
	if err := m.executeCommand(reloadCmd, "重载防火墙配置", ip); err != nil {
		return err
	}

	return nil
}

// RestoreRules 恢复防火墙规则
//
// 返回值:
// - error: 恢复过程中的错误
func (m *firewallCmdManager) RestoreRules() error {
	return nil
}

// UnblockIP 解除封禁 IP
//
// 参数:
// - ip: IP 地址或 CIDR 网段
//
// 返回值:
// - error: 解除封禁过程中的错误
func (m *firewallCmdManager) UnblockIP(ip string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if m.useZone {
		// 使用 Zone 方式解除 IP 或网段封禁
		removeSourceCmd := exec.CommandContext(ctx, "firewall-cmd", "--zone="+m.blockZone, "--remove-source", ip, "--permanent")
		if err := m.executeCommand(removeSourceCmd, "使用 Zone 解除 IP 封禁", ip); err != nil {
			return err
		}
	} else {
		// 使用富规则方式解除 IP 或网段封禁
		removeRuleCmd := exec.CommandContext(ctx, "firewall-cmd", "--zone=public", "--remove-rich-rule",
			fmt.Sprintf(`rule family="ipv4" source address="%s" reject`, ip), "--permanent")
		if err := m.executeCommand(removeRuleCmd, "解除封禁IP", ip); err != nil {
			return err
		}
	}

	reloadCmd := exec.CommandContext(ctx, "firewall-cmd", "--reload")
	if err := m.executeCommand(reloadCmd, "重载防火墙配置", ip); err != nil {
		return err
	}

	return nil
}

// Close 关闭防火墙管理器
//
// 返回值:
// - error: 关闭过程中的错误
func (m *firewallCmdManager) Close() error {
	return nil
}

// initBlockZone 初始化 block zone
//
// 返回值:
// - error: 初始化过程中的错误
func (m *firewallCmdManager) initBlockZone() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 检查 block zone 是否存在
	checkCmd := exec.CommandContext(ctx, "firewall-cmd", "--get-zones")
	output, err := checkCmd.Output()
	if err != nil {
		return fmt.Errorf("获取 firewall-cmd zones 失败: %w", err)
	}

	zones := strings.Split(strings.TrimSpace(string(output)), " ")
	zoneExists := false
	for _, zone := range zones {
		if zone == m.blockZone {
			zoneExists = true
			break
		}
	}

	// 如果 block zone 不存在，创建它
	if !zoneExists {
		createCmd := exec.CommandContext(ctx, "firewall-cmd", "--new-zone", m.blockZone, "--permanent")
		if err := createCmd.Run(); err != nil {
			return fmt.Errorf("创建 block zone 失败: %w", err)
		}

		// 重载防火墙配置
		reloadCmd := exec.CommandContext(ctx, "firewall-cmd", "--reload")
		if err := reloadCmd.Run(); err != nil {
			return fmt.Errorf("重载防火墙配置失败: %w", err)
		}
	}

	m.logger.Info("block zone 初始化成功", zap.String("zone", m.blockZone))
	return nil
}

// executeCommand 执行命令
//
// 参数:
// - cmd: 命令
// - action: 操作描述
// - ip: IP 地址
//
// 返回值:
// - error: 执行过程中的错误
func (m *firewallCmdManager) executeCommand(cmd *exec.Cmd, action, ip string) error {
	output, err := cmd.CombinedOutput()
	if err != nil {
		m.logger.Error(fmt.Sprintf("%s失败", action),
			zap.String("ip", ip),
			zap.String("command", cmd.String()),
			zap.String("output", strings.TrimSuffix(string(output), "\n")),
			zap.Error(err))
		return err
	}
	m.logger.Info(fmt.Sprintf("成功%s", action),
		zap.String("ip", ip),
		zap.String("command", cmd.String()),
		zap.String("output", strings.TrimSuffix(string(output), "\n")))
	return nil
}
