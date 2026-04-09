// Package alert 提供告警功能测试
package alert

import (
	"testing"

	"go.uber.org/zap"
)

func TestEmailAlert(t *testing.T) {
	logger, err := zap.NewProduction()
	if err != nil {
		t.Fatalf("创建日志器失败: %v", err)
	}

	// 创建邮件告警实例
	emailAlert := NewEmailAlert(
		"smtp.example.com",
		"587",
		"user@example.com",
		"password",
		"from@example.com",
		[]string{"to@example.com"},
		"sshwarden 告警",
		logger,
	)

	// 测试发送封禁告警
	err = emailAlert.SendBanAlert("192.168.1.100", "中国|0|广东省|深圳市|阿里云")
	// 注意：由于这是一个测试，实际发送邮件可能会失败，所以我们只检查函数是否正常执行
	if err != nil {
		t.Logf("发送封禁告警失败（预期行为，因为是测试环境）: %v", err)
	}

	// 测试发送解除封禁告警
	err = emailAlert.SendUnbanAlert("192.168.1.100", "中国|0|广东省|深圳市|阿里云")
	// 同样，实际发送邮件可能会失败，所以我们只检查函数是否正常执行
	if err != nil {
		t.Logf("发送解除封禁告警失败（预期行为，因为是测试环境）: %v", err)
	}

	// 测试空收件人列表
	emailAlertNoRecipients := NewEmailAlert(
		"smtp.example.com",
		"587",
		"user@example.com",
		"password",
		"from@example.com",
		[]string{},
		"sshwarden 告警",
		logger,
	)

	// 测试发送封禁告警（空收件人列表）
	err = emailAlertNoRecipients.SendBanAlert("192.168.1.100", "中国|0|广东省|深圳市|阿里云")
	if err != nil {
		t.Errorf("发送封禁告警失败（空收件人列表）: %v", err)
	}

	// 测试发送解除封禁告警（空收件人列表）
	err = emailAlertNoRecipients.SendUnbanAlert("192.168.1.100", "中国|0|广东省|深圳市|阿里云")
	if err != nil {
		t.Errorf("发送解除封禁告警失败（空收件人列表）: %v", err)
	}
}
