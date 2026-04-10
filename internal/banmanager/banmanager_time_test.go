package banmanager

import (
	"os"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestBanManagerWithTime(t *testing.T) {
	logger, err := zap.NewProduction()
	if err != nil {
		t.Fatalf("创建日志器失败: %v", err)
	}

	// 测试时间窗口功能（5秒）
	findTime := 5 * time.Second
	// 测试封禁时间功能（10秒）
	banTime := 10 * time.Second

	t.Run("时间窗口内的失败计数", func(t *testing.T) {
		tempFile, err := os.CreateTemp("", "blockedips-*.json")
		if err != nil {
			t.Fatalf("创建临时文件失败: %v", err)
		}
		defer func() {
			if err := os.Remove(tempFile.Name()); err != nil {
				t.Logf("清理临时文件失败: %v", err)
			}
		}()

		mockFirewall := &mockFirewall{}
		mgr := NewManager(tempFile.Name(), findTime, banTime, mockFirewall, logger)

		ip := "192.168.1.100"

		// 第一次失败
		failures := mgr.IncrementFailure(ip)
		if failures != 1 {
			t.Errorf("期望第1次失败，实际: %d", failures)
		}

		// 立即第二次失败（在时间窗口内）
		failures = mgr.IncrementFailure(ip)
		if failures != 2 {
			t.Errorf("期望第2次失败，实际: %d", failures)
		}

		// 等待时间窗口过期
		time.Sleep(findTime + 1*time.Second)

		// 再次失败（应该重置计数）
		failures = mgr.IncrementFailure(ip)
		if failures != 1 {
			t.Errorf("期望重置计数为1，实际: %d", failures)
		}
	})

	t.Run("封禁时间功能", func(t *testing.T) {
		tempFile, err := os.CreateTemp("", "blockedips-*.json")
		if err != nil {
			t.Fatalf("创建临时文件失败: %v", err)
		}
		defer func() {
			if err := os.Remove(tempFile.Name()); err != nil {
				t.Logf("清理临时文件失败: %v", err)
			}
		}()

		mockFirewall := &mockFirewall{}
		mgr := NewManager(tempFile.Name(), findTime, banTime, mockFirewall, logger)

		ip := "10.0.0.100"

		// 封禁IP
		if err := mgr.BlockIP(ip); err != nil {
			t.Fatalf("封禁IP失败: %v", err)
		}

		// 检查是否被封禁
		if !mgr.IsBlocked(ip) {
			t.Errorf("IP应该被封禁")
		}

		// 等待封禁时间过期
		time.Sleep(banTime + 1*time.Second)

		// 检查是否自动解除封禁
		if mgr.IsBlocked(ip) {
			t.Errorf("IP应该已解除封禁")
		}
	})

	t.Run("永久封禁功能", func(t *testing.T) {
		tempFile, err := os.CreateTemp("", "blockedips-*.json")
		if err != nil {
			t.Fatalf("创建临时文件失败: %v", err)
		}
		defer func() {
			if err := os.Remove(tempFile.Name()); err != nil {
				t.Logf("清理临时文件失败: %v", err)
			}
		}()

		ip := "10.0.0.200"

		// 创建一个永久封禁的管理器
		mockFirewall := &mockFirewall{}
		permanentMgr := NewManager(tempFile.Name(), findTime, -1, mockFirewall, logger)

		// 封禁IP
		if err := permanentMgr.BlockIP(ip); err != nil {
			t.Fatalf("封禁IP失败: %v", err)
		}

		// 检查是否被封禁
		if !permanentMgr.IsBlocked(ip) {
			t.Errorf("IP应该被永久封禁")
		}

		// 等待一段时间
		time.Sleep(2 * time.Second)

		// 检查是否仍然被封禁
		if !permanentMgr.IsBlocked(ip) {
			t.Errorf("IP应该仍然被永久封禁")
		}
	})

	t.Run("清理过期封禁记录", func(t *testing.T) {
		tempFile, err := os.CreateTemp("", "blockedips-*.json")
		if err != nil {
			t.Fatalf("创建临时文件失败: %v", err)
		}
		defer func() {
			if err := os.Remove(tempFile.Name()); err != nil {
				t.Logf("清理临时文件失败: %v", err)
			}
		}()

		mockFirewall := &mockFirewall{}
		mgr := NewManager(tempFile.Name(), findTime, banTime, mockFirewall, logger)

		ip1 := "10.0.0.100"
		ip2 := "10.0.0.101"

		// 封禁两个IP
		if err := mgr.BlockIP(ip1); err != nil {
			t.Fatalf("封禁IP1失败: %v", err)
		}
		if err := mgr.BlockIP(ip2); err != nil {
			t.Fatalf("封禁IP2失败: %v", err)
		}

		// 等待封禁时间过期
		time.Sleep(banTime + 1*time.Second)

		// 清理过期记录
		mgr.CleanupExpired()

		// 检查是否都已解除封禁
		if mgr.IsBlocked(ip1) || mgr.IsBlocked(ip2) {
			t.Errorf("所有IP都应该已解除封禁")
		}
	})
}
