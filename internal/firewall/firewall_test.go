package firewall

import (
	"testing"

	"github.com/logdotar/sshwarden/internal/config"

	"go.uber.org/zap"
)

func TestFirewallManager(t *testing.T) {
	logger, err := zap.NewProduction()
	if err != nil {
		t.Fatalf("创建日志器失败: %v", err)
	}

	// 测试 iptables 防火墙
	t.Run("iptables", func(t *testing.T) {
		cfg := &config.FirewallConfig{
			Type: "iptables",
		}
		mgr, err := NewManager(cfg, logger)
		if err != nil {
			t.Skipf("创建 iptables 防火墙管理器失败: %v", err)
		}

		// 测试添加规则（这些操作可能需要特权）
		// 注意：在实际测试中，这些操作可能会失败，因为需要 root 权限
		ip := "192.168.1.100"

		// 尝试添加规则（可能会失败，因为需要特权）
		err = mgr.BlockIP(ip)
		if err != nil {
			t.Logf("添加 iptables 规则失败（可能需要特权）: %v", err)
		}
	})

	// 测试 firewall-cmd 防火墙
	t.Run("firewall-cmd", func(t *testing.T) {
		cfg := &config.FirewallConfig{
			Type: "firewall-cmd",
		}
		mgr, err := NewManager(cfg, logger)
		if err != nil {
			t.Skipf("创建 firewall-cmd 防火墙管理器失败: %v", err)
		}

		// 测试添加规则（这些操作可能需要特权）
		ip := "192.168.1.100"

		// 尝试添加规则（可能会失败，因为需要特权）
		err = mgr.BlockIP(ip)
		if err != nil {
			t.Logf("添加 firewall-cmd 规则失败（可能需要特权）: %v", err)
		}
	})

	// 测试未知防火墙类型
	t.Run("unknown", func(t *testing.T) {
		cfg := &config.FirewallConfig{
			Type: "unknown",
		}
		_, err := NewManager(cfg, logger)
		if err == nil {
			t.Errorf("期望创建未知防火墙类型失败, 实际成功")
		}
	})

	// 测试 iptables 使用 ipset 方式
	t.Run("iptables_with_ipset", func(t *testing.T) {
		cfg := &config.FirewallConfig{
			Type:      "iptables",
			UseIPSet:  true,
			IPSetName: "test_ssh_warden",
		}
		mgr, err := NewManager(cfg, logger)
		if err != nil {
			t.Skipf("创建 iptables 防火墙管理器失败: %v", err)
		}

		// 测试封禁单个 IP
		ip := "192.168.2.100"
		err = mgr.BlockIP(ip)
		if err != nil {
			t.Logf("使用 ipset 封禁 IP 失败（可能需要特权）: %v", err)
		}

		// 测试封禁 CIDR 网段
		cidr := "192.168.3.0/24"
		err = mgr.BlockIP(cidr)
		if err != nil {
			t.Logf("使用 ipset 封禁 CIDR 网段失败（可能需要特权）: %v", err)
		}

		// 测试解除封禁
		err = mgr.UnblockIP(ip)
		if err != nil {
			t.Logf("使用 ipset 解除封禁 IP 失败（可能需要特权）: %v", err)
		}

		err = mgr.UnblockIP(cidr)
		if err != nil {
			t.Logf("使用 ipset 解除封禁 CIDR 网段失败（可能需要特权）: %v", err)
		}
	})

	// 测试 firewall-cmd 使用 Zone 方式
	t.Run("firewall-cmd_with_zone", func(t *testing.T) {
		cfg := &config.FirewallConfig{
			Type:     "firewall-cmd",
			UseIPSet: true, // 复用此配置项来控制是否使用 Zone 方式
		}
		mgr, err := NewManager(cfg, logger)
		if err != nil {
			t.Skipf("创建 firewall-cmd 防火墙管理器失败: %v", err)
		}

		// 测试封禁单个 IP
		ip := "192.168.4.100"
		err = mgr.BlockIP(ip)
		if err != nil {
			t.Logf("使用 Zone 方式封禁 IP 失败（可能需要特权）: %v", err)
		}

		// 测试封禁 CIDR 网段
		cidr := "192.168.5.0/24"
		err = mgr.BlockIP(cidr)
		if err != nil {
			t.Logf("使用 Zone 方式封禁 CIDR 网段失败（可能需要特权）: %v", err)
		}

		// 测试解除封禁
		err = mgr.UnblockIP(ip)
		if err != nil {
			t.Logf("使用 Zone 方式解除封禁 IP 失败（可能需要特权）: %v", err)
		}

		err = mgr.UnblockIP(cidr)
		if err != nil {
			t.Logf("使用 Zone 方式解除封禁 CIDR 网段失败（可能需要特权）: %v", err)
		}
	})
}
