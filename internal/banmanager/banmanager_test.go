package banmanager

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"
)

// mockFirewall 是一个 mock 的 firewall.Manager 实现，用于测试
//
// 它实现了 firewall.Manager 接口的所有方法，但不做任何实际操作
type mockFirewall struct{}

// BlockIP 封禁 IP (空操作)
func (m *mockFirewall) BlockIP(string) error {
	return nil
}

// UnblockIP 解除封禁 IP (空操作)
func (m *mockFirewall) UnblockIP(string) error {
	return nil
}

// RestoreRules 恢复防火墙规则 (空操作)
func (m *mockFirewall) RestoreRules() error {
	return nil
}

// Close 关闭防火墙管理器 (空操作)
func (m *mockFirewall) Close() error {
	return nil
}

// mockWhitelistManager 是一个 mock 的 WhitelistManager 实现，用于测试
type mockWhitelistManager struct{}

// Contains 检查 IP 是否在白名单中
func (m *mockWhitelistManager) Contains(ip string) bool {
	// 模拟白名单，包含 192.168.1.1
	return ip == "192.168.1.1"
}

func TestBanManager(t *testing.T) {
	logger, err := zap.NewProduction()
	if err != nil {
		t.Fatalf("创建日志器失败: %v", err)
	}

	tempFile, err := os.CreateTemp("", "blockedips-*.json")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	defer func() {
		if err := os.Remove(tempFile.Name()); err != nil {
			t.Logf("清理临时文件失败: %v", err)
		}
	}()

	findTime := 10 * time.Minute
	banTime := 10 * time.Minute
	mockFirewall := &mockFirewall{}
	mockWhitelist := &mockWhitelistManager{}

	mgr := NewManager(tempFile.Name(), findTime, banTime, mockFirewall, logger)

	t.Run("ExtractIP", func(t *testing.T) {
		patterns := []string{
			"authentication failure;.*rhost=(\\S+)",
			"Failed password for .* from (\\S+)",
		}
		if err := mgr.LoadRegexPatterns(patterns); err != nil {
			t.Fatalf("加载正则表达式失败: %v", err)
		}

		testCases := []struct {
			logLine  string
			expected string
		}{
			{
				logLine:  "authentication failure; rhost=192.168.1.1",
				expected: "192.168.1.1",
			},
			{
				logLine:  "Failed password for root from 10.0.0.1 port 22 ssh2",
				expected: "10.0.0.1",
			},
			{
				logLine:  "normal log line",
				expected: "",
			},
		}

		for _, tc := range testCases {
			result := mgr.ExtractIP(tc.logLine)
			if result != tc.expected {
				t.Errorf("期望 %q, 实际 %q", tc.expected, result)
			}
		}
	})

	t.Run("IncrementFailure", func(t *testing.T) {
		ip := "192.168.1.100"
		for i := 1; i <= 3; i++ {
			failures := mgr.IncrementFailure(ip)
			if failures != i {
				t.Errorf("期望 %d 次失败, 实际 %d", i, failures)
			}
		}
	})

	t.Run("BlockIP", func(t *testing.T) {
		ip := "10.0.0.100"
		if mgr.IsBlocked(ip) {
			t.Errorf("IP 不应该被封禁")
		}

		if err := mgr.BlockIP(ip); err != nil {
			t.Fatalf("封禁 IP 失败: %v", err)
		}

		if !mgr.IsBlocked(ip) {
			t.Errorf("IP 应该被封禁")
		}
	})

	t.Run("BlockIP_InvalidIP", func(t *testing.T) {
		// 测试无效 IP 地址
		invalidIP := "999.999.999.999"
		err := mgr.BlockIP(invalidIP)
		if err == nil {
			t.Errorf("封禁无效 IP 应该失败")
		}
	})

	t.Run("UnblockIP", func(t *testing.T) {
		ip := "10.0.0.200"

		// 先封禁 IP
		if err := mgr.BlockIP(ip); err != nil {
			t.Fatalf("封禁 IP 失败: %v", err)
		}

		if !mgr.IsBlocked(ip) {
			t.Errorf("IP 应该被封禁")
		}

		// 解除封禁
		if err := mgr.UnblockIP(ip); err != nil {
			t.Fatalf("解除封禁 IP 失败: %v", err)
		}

		if mgr.IsBlocked(ip) {
			t.Errorf("IP 应该被解除封禁")
		}
	})

	t.Run("UnblockIP_InvalidIP", func(t *testing.T) {
		// 测试无效 IP 地址
		invalidIP := "999.999.999.999"
		err := mgr.UnblockIP(invalidIP)
		if err == nil {
			t.Errorf("解除封禁无效 IP 应该失败")
		}
	})

	t.Run("UnblockIP_NotBlocked", func(t *testing.T) {
		// 测试解除未封禁的 IP
		ip := "10.0.0.300"
		err := mgr.UnblockIP(ip)
		if err == nil {
			t.Errorf("解除未封禁的 IP 应该失败")
		}
	})

	t.Run("CleanupExpired", func(t *testing.T) {
		// 测试清理过期的封禁记录
		ip := "10.0.0.102"

		// 先封禁 IP
		if err := mgr.BlockIP(ip); err != nil {
			t.Fatalf("封禁 IP 失败: %v", err)
		}

		// 清理过期记录（应该没有过期记录）
		expiredCount := mgr.CleanupExpired()
		if expiredCount != 0 {
			t.Errorf("期望清理 0 个过期记录，实际清理了 %d 个", expiredCount)
		}
	})

	t.Run("BlockIPPermanently", func(t *testing.T) {
		ip := "10.0.0.150"
		if mgr.IsBlocked(ip) {
			t.Errorf("IP 不应该被封禁")
		}

		if err := mgr.BlockIPPermanently(ip, mockWhitelist); err != nil {
			t.Fatalf("永久封禁 IP 失败: %v", err)
		}

		if !mgr.IsBlocked(ip) {
			t.Errorf("IP 应该被永久封禁")
		}

		// 验证永久封禁的 IP 不会被清理
		expiredCount := mgr.CleanupExpired()
		if expiredCount != 0 {
			t.Errorf("永久封禁的 IP 不应该被清理，实际清理了 %d 个", expiredCount)
		}

		if !mgr.IsBlocked(ip) {
			t.Errorf("永久封禁的 IP 应该仍然被封禁")
		}
	})

	t.Run("BlockIPPermanently_InvalidIP", func(t *testing.T) {
		// 测试无效 IP 地址
		invalidIP := "999.999.999.999"
		err := mgr.BlockIPPermanently(invalidIP, mockWhitelist)
		if err == nil {
			t.Errorf("永久封禁无效 IP 应该失败")
		}
	})

	t.Run("UpdateTimeSettings_PermanentBlock", func(t *testing.T) {
		// 测试配置热重载时，永久封禁的 IP 不会被修改
		ip := "10.0.0.160"

		// 先永久封禁 IP
		if err := mgr.BlockIPPermanently(ip, mockWhitelist); err != nil {
			t.Fatalf("永久封禁 IP 失败: %v", err)
		}

		// 检查 IP 是否被永久封禁
		if !mgr.IsBlocked(ip) {
			t.Errorf("IP 应该被永久封禁")
		}

		// 获取封禁详情
		blockedIPs := mgr.GetBlockedIPDetails()
		var targetIP BlockedIP
		for _, ipDetail := range blockedIPs {
			if ipDetail.IP == ip {
				targetIP = ipDetail
				break
			}
		}

		if !targetIP.Permanent {
			t.Errorf("IP 应该被标记为永久封禁")
		}

		// 模拟配置热重载，将 banTime 从 -1 改为 10 分钟
		mgr.UpdateTimeSettings(10*time.Minute, 10*time.Minute)

		// 检查 IP 是否仍然被封禁
		if !mgr.IsBlocked(ip) {
			t.Errorf("永久封禁的 IP 应该仍然被封禁")
		}

		// 再次获取封禁详情，检查 Permanent 字段是否仍然为 true
		blockedIPs = mgr.GetBlockedIPDetails()
		for _, ipDetail := range blockedIPs {
			if ipDetail.IP == ip {
				targetIP = ipDetail
				break
			}
		}

		if !targetIP.Permanent {
			t.Errorf("永久封禁的 IP 应该仍然被标记为永久封禁")
		}
	})

	t.Run("UpdateTimeSettings_TemporaryBlock", func(t *testing.T) {
		// 测试配置热重载时，临时封禁的 IP 过期时间会根据新的 banTime 调整
		ip := "10.0.0.180"

		// 先设置 banTime 为 5 分钟
		mgr.UpdateTimeSettings(10*time.Minute, 5*time.Minute)

		// 封禁 IP
		if err := mgr.BlockIP(ip); err != nil {
			t.Fatalf("封禁 IP 失败: %v", err)
		}

		// 检查 IP 是否被封禁
		if !mgr.IsBlocked(ip) {
			t.Errorf("IP 应该被封禁")
		}

		// 获取封禁详情，记录当前过期时间
		blockedIPs := mgr.GetBlockedIPDetails()
		var targetIP BlockedIP
		for _, ipDetail := range blockedIPs {
			if ipDetail.IP == ip {
				targetIP = ipDetail
				break
			}
		}

		if targetIP.Permanent {
			t.Errorf("IP 不应该被标记为永久封禁")
		}

		// 模拟配置热重载，将 banTime 从 5 分钟改为 10 分钟
		mgr.UpdateTimeSettings(10*time.Minute, 10*time.Minute)

		// 检查 IP 是否仍然被封禁
		if !mgr.IsBlocked(ip) {
			t.Errorf("IP 应该仍然被封禁")
		}

		// 再次获取封禁详情，检查过期时间是否更新
		blockedIPs = mgr.GetBlockedIPDetails()
		var updatedIP BlockedIP
		for _, ipDetail := range blockedIPs {
			if ipDetail.IP == ip {
				updatedIP = ipDetail
				break
			}
		}

		if updatedIP.Permanent {
			t.Errorf("IP 不应该被标记为永久封禁")
		}

		// 检查过期时间是否延长了
		if !updatedIP.ExpiresAt.After(targetIP.ExpiresAt) {
			t.Errorf("过期时间应该被延长，旧过期时间: %v, 新过期时间: %v", targetIP.ExpiresAt, updatedIP.ExpiresAt)
		}
	})

	t.Run("CleanupExpired_PermanentBlock", func(t *testing.T) {
		// 测试清理过期记录时，永久封禁的 IP 不会被清理
		ip := "10.0.0.170"

		// 先永久封禁 IP
		if err := mgr.BlockIPPermanently(ip, mockWhitelist); err != nil {
			t.Fatalf("永久封禁 IP 失败: %v", err)
		}

		// 清理过期记录
		expiredCount := mgr.CleanupExpired()
		if expiredCount != 0 {
			t.Errorf("永久封禁的 IP 不应该被清理，实际清理了 %d 个", expiredCount)
		}

		// 检查 IP 是否仍然被封禁
		if !mgr.IsBlocked(ip) {
			t.Errorf("永久封禁的 IP 应该仍然被封禁")
		}
	})

	t.Run("LoadPermanentBlockIPs", func(t *testing.T) {
		// 测试加载永久封禁 IP 列表
		permanentIPs := []string{"192.168.1.100", "10.0.0.50", "invalid-ip"}

		// 加载永久封禁 IP 列表
		mgr.LoadPermanentBlockIPs(permanentIPs, mockWhitelist)

		// 检查有效的 IP 是否被永久封禁
		if !mgr.IsBlocked("192.168.1.100") {
			t.Errorf("192.168.1.100 应该被永久封禁")
		}

		if !mgr.IsBlocked("10.0.0.50") {
			t.Errorf("10.0.0.50 应该被永久封禁")
		}

		// 获取封禁详情，检查 Permanent 字段
		blockedIPs := mgr.GetBlockedIPDetails()
		for _, ipDetail := range blockedIPs {
			if ipDetail.IP == "192.168.1.100" && !ipDetail.Permanent {
				t.Errorf("192.168.1.100 应该被标记为永久封禁")
			}
			if ipDetail.IP == "10.0.0.50" && !ipDetail.Permanent {
				t.Errorf("10.0.0.50 应该被标记为永久封禁")
			}
		}
	})

	t.Run("BlockIP_CIDR", func(t *testing.T) {
		// 测试封禁 CIDR 网段
		cidr := "192.168.1.0/24"
		if mgr.IsBlocked(cidr) {
			t.Errorf("网段不应该被封禁")
		}

		if err := mgr.BlockIP(cidr); err != nil {
			t.Fatalf("封禁网段失败: %v", err)
		}

		if !mgr.IsBlocked(cidr) {
			t.Errorf("网段应该被封禁")
		}
	})

	t.Run("UnblockIP_CIDR", func(t *testing.T) {
		// 测试解除 CIDR 网段的封禁
		cidr := "192.168.2.0/24"

		// 先封禁网段
		if err := mgr.BlockIP(cidr); err != nil {
			t.Fatalf("封禁网段失败: %v", err)
		}

		if !mgr.IsBlocked(cidr) {
			t.Errorf("网段应该被封禁")
		}

		// 解除封禁
		if err := mgr.UnblockIP(cidr); err != nil {
			t.Fatalf("解除封禁网段失败: %v", err)
		}

		if mgr.IsBlocked(cidr) {
			t.Errorf("网段应该被解除封禁")
		}
	})

	t.Run("BlockIPPermanently_CIDR", func(t *testing.T) {
		// 测试永久封禁 CIDR 网段
		cidr := "10.0.0.0/8"
		if mgr.IsBlocked(cidr) {
			t.Errorf("网段不应该被封禁")
		}

		if err := mgr.BlockIPPermanently(cidr, mockWhitelist); err != nil {
			t.Fatalf("永久封禁网段失败: %v", err)
		}

		if !mgr.IsBlocked(cidr) {
			t.Errorf("网段应该被永久封禁")
		}

		// 验证永久封禁的网段不会被清理
		expiredCount := mgr.CleanupExpired()
		if expiredCount != 0 {
			t.Errorf("永久封禁的网段不应该被清理，实际清理了 %d 个", expiredCount)
		}

		if !mgr.IsBlocked(cidr) {
			t.Errorf("永久封禁的网段应该仍然被封禁")
		}
	})

	t.Run("LoadPermanentBlockIPs_CIDR", func(t *testing.T) {
		// 测试加载包含 CIDR 网段的永久封禁列表
		permanentIPs := []string{"192.168.3.0/24", "10.1.0.0/16"}

		// 加载永久封禁 IP 列表
		mgr.LoadPermanentBlockIPs(permanentIPs, mockWhitelist)

		// 检查有效的网段是否被永久封禁
		if !mgr.IsBlocked("192.168.3.0/24") {
			t.Errorf("192.168.3.0/24 应该被永久封禁")
		}

		if !mgr.IsBlocked("10.1.0.0/16") {
			t.Errorf("10.1.0.0/16 应该被永久封禁")
		}

		// 获取封禁详情，检查 Permanent 字段
		blockedIPs := mgr.GetBlockedIPDetails()
		for _, ipDetail := range blockedIPs {
			if ipDetail.IP == "192.168.3.0/24" && !ipDetail.Permanent {
				t.Errorf("192.168.3.0/24 应该被标记为永久封禁")
			}
			if ipDetail.IP == "10.1.0.0/16" && !ipDetail.Permanent {
				t.Errorf("10.1.0.0/16 应该被标记为永久封禁")
			}
		}
	})

	t.Run("BlockIPPermanently_WhitelistConflict", func(t *testing.T) {
		// 测试永久封禁白名单中的 IP
		whitelistIP := "192.168.1.1" // 此 IP 在 mockWhitelist 中

		// 尝试永久封禁白名单中的 IP，应该失败
		err := mgr.BlockIPPermanently(whitelistIP, mockWhitelist)
		if err == nil {
			t.Errorf("永久封禁白名单中的 IP 应该失败")
		}

		// 检查 IP 是否未被封禁
		if mgr.IsBlocked(whitelistIP) {
			t.Errorf("白名单中的 IP 不应该被封禁")
		}
	})

	t.Run("IsBlocked_FileSync", func(t *testing.T) {
		// 测试 IsBlocked 方法的文件同步功能
		ip := "10.0.0.190"

		// 先封禁 IP
		if err := mgr.BlockIP(ip); err != nil {
			t.Fatalf("封禁 IP 失败: %v", err)
		}

		// 检查 IP 是否被封禁
		if !mgr.IsBlocked(ip) {
			t.Errorf("IP 应该被封禁")
		}

		// 手动修改 blockedips.json 文件，移除该 IP
		blockedIPs := mgr.GetBlockedIPDetails()
		var newBlockedIPs []BlockedIP
		for _, blockedIP := range blockedIPs {
			if blockedIP.IP != ip {
				newBlockedIPs = append(newBlockedIPs, blockedIP)
			}
		}

		// 保存修改后的封禁记录
		blockedIPsFile := BlockedIPs{
			IPs: newBlockedIPs,
		}

		file, err := os.Create(mgr.blockedIPsFile)
		if err != nil {
			t.Fatalf("创建文件失败: %v", err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				t.Logf("关闭文件失败: %v", err)
			}
		}()

		if err := json.NewEncoder(file).Encode(blockedIPsFile); err != nil {
			t.Fatalf("写入文件失败: %v", err)
		}

		// 再次检查 IP 是否被封禁（应该解除封禁）
		if mgr.IsBlocked(ip) {
			t.Errorf("IP 应该被解除封禁")
		}
	})

	t.Run("LoadBlockedIPs_FileError", func(t *testing.T) {
		// 测试 LoadBlockedIPs 方法的错误处理
		// 手动创建一个格式错误的文件
		file, err := os.Create(mgr.blockedIPsFile)
		if err != nil {
			t.Fatalf("创建文件失败: %v", err)
		}
		// 写入错误的 JSON 格式
		if _, err := file.WriteString("invalid json"); err != nil {
			t.Fatalf("写入文件失败: %v", err)
		}
		if err := file.Close(); err != nil {
			t.Logf("关闭文件失败: %v", err)
		}

		// 调用 LoadBlockedIPs 方法，应该不会崩溃
		if err := mgr.LoadBlockedIPs(); err != nil {
			t.Logf("加载封禁IP列表失败: %v", err)
		}

		// 测试文件不存在的情况
		if err := os.Remove(mgr.blockedIPsFile); err != nil {
			t.Logf("删除文件失败: %v", err)
		}
		if err := mgr.LoadBlockedIPs(); err != nil {
			t.Logf("加载封禁IP列表失败: %v", err)
		}
	})

	t.Run("LoadPermanentBlockIPs_WhitelistConflict", func(t *testing.T) {
		// 测试加载包含白名单 IP 的永久封禁列表
		permanentIPs := []string{"192.168.1.1", "192.168.4.1"} // 192.168.1.1 在白名单中

		// 加载永久封禁 IP 列表
		mgr.LoadPermanentBlockIPs(permanentIPs, mockWhitelist)

		// 检查白名单中的 IP 是否未被封禁
		if mgr.IsBlocked("192.168.1.1") {
			t.Errorf("白名单中的 IP 192.168.1.1 不应该被封禁")
		}

		// 检查非白名单中的 IP 是否被封禁
		if !mgr.IsBlocked("192.168.4.1") {
			t.Errorf("非白名单中的 IP 192.168.4.1 应该被永久封禁")
		}
	})
}
