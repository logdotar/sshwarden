package ipregion

import (
	"testing"

	"go.uber.org/zap"

	"github.com/logdotar/sshwarden/internal/config"
)

func TestIPRegion(t *testing.T) {
	logger, err := zap.NewProduction()
	if err != nil {
		t.Fatalf("创建日志器失败: %v", err)
	}

	// 测试数据库不存在的情况
	cfg := &config.IPConfig{
		RegionDBPath: "nonexistent.db",
	}
	mgr := NewManager(cfg, logger)

	// 测试 IP 归属地查询
	region, err := mgr.GetRegion("8.8.8.8")
	if err != nil {
		t.Logf("查询 IP 归属地失败（数据库不存在）: %v", err)
	}
	if region != "未知" {
		t.Errorf("期望 IP 归属地为 '未知', 实际: %s", region)
	}

	// 测试空 IP
	region, err = mgr.GetRegion("")
	if err != nil {
		t.Logf("查询空 IP 归属地失败: %v", err)
	}
	if region != "未知" {
		t.Errorf("期望空 IP 归属地为 '未知', 实际: %s", region)
	}

	// 测试无效 IP
	region, err = mgr.GetRegion("invalid-ip")
	if err != nil {
		t.Logf("查询无效 IP 归属地失败: %v", err)
	}
	if region != "未知" {
		t.Errorf("期望无效 IP 归属地为 '未知', 实际: %s", region)
	}
}
