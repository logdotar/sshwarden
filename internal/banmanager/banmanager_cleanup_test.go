package banmanager

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestCleanupExpired_ManualBlock 测试手动添加的临时封禁 IP 到期后是否被清理
func TestCleanupExpired_ManualBlock(t *testing.T) {
	// 创建临时文件用于测试
	tempFile, err := os.CreateTemp("", "blockedips")
	assert.NoError(t, err)
	tempFileName := tempFile.Name()
	defer func() {
		_ = os.Remove(tempFileName)
	}()
	_ = tempFile.Close()

	// 创建日志器
	logger, err := zap.NewDevelopment()
	assert.NoError(t, err)
	defer func() {
		_ = logger.Sync()
	}()

	// 创建 banmanager
	m := NewManager(tempFileName, 10*time.Minute, 1*time.Second, nil, logger)

	// 手动添加一个临时封禁 IP
	ip := "192.168.1.100"
	err = m.BlockIP(ip)
	assert.NoError(t, err)

	// 验证 IP 被封禁
	assert.True(t, m.IsBlocked(ip))

	// 等待 2 秒，确保封禁时间已过期
	time.Sleep(2 * time.Second)

	// 执行清理
	m.CleanupExpired()

	// 验证 IP 不再被封禁
	assert.False(t, m.IsBlocked(ip))

	// 验证文件中不再包含该 IP
	blockedIPs, err := loadBlockedIPsFromFile(tempFileName)
	assert.NoError(t, err)

	found := false
	for _, blockedIP := range blockedIPs.IPs {
		if blockedIP.IP == ip {
			found = true
			break
		}
	}
	assert.False(t, found, "过期的 IP 应该从文件中删除")
}

// loadBlockedIPsFromFile 从文件加载封禁记录
func loadBlockedIPsFromFile(filePath string) (BlockedIPs, error) {
	var blockedIPs BlockedIPs

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return BlockedIPs{IPs: []BlockedIP{}}, nil
		}
		return blockedIPs, err
	}

	err = json.Unmarshal(data, &blockedIPs)
	return blockedIPs, err
}
