package logger

import (
	"os"
	"testing"

	"github.com/logdotar/sshwarden/internal/config"
)

func TestNewLogger(t *testing.T) {
	// 测试创建日志器
	cfg := &config.LogConfig{
		Level:      "info",
		Filename:   "test.log",
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     7,
		LocalTime:  true,
		Compress:   true,
	}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("创建日志器失败: %v", err)
	}

	// 测试日志输出
	logger.Info("测试信息日志")
	logger.Warn("测试警告日志")
	logger.Error("测试错误日志")

	// 清理测试日志文件
	defer func() {
		if err := os.Remove("test.log"); err != nil {
			t.Logf("清理测试日志文件失败: %v", err)
		}
	}()
	defer func() {
		if err := os.Remove("test.log.1"); err != nil {
			t.Logf("清理测试日志备份文件失败: %v", err)
		}
	}()
}

func TestNewLoggerWithDifferentLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error"}

	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			cfg := &config.LogConfig{
				Level:      level,
				Filename:   "test_" + level + ".log",
				MaxSize:    10,
				MaxBackups: 3,
				MaxAge:     7,
				LocalTime:  true,
				Compress:   true,
			}

			logger, err := New(cfg)
			if err != nil {
				t.Fatalf("创建 %s 级别日志器失败: %v", level, err)
			}

			// 测试不同级别的日志输出
			logger.Debug("测试调试日志")
			logger.Info("测试信息日志")
			logger.Warn("测试警告日志")
			logger.Error("测试错误日志")

			// 清理测试日志文件
			defer func() {
				if err := os.Remove("test_" + level + ".log"); err != nil {
					t.Logf("清理测试日志文件失败: %v", err)
				}
			}()
		})
	}
}
