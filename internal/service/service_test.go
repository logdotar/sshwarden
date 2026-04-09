// Package service 提供服务安装和卸载功能测试
package service

import (
	"os"
	"runtime"
	"testing"
)

func TestService(t *testing.T) {
	// 检查是否在 Linux 系统上运行
	if runtime.GOOS != "linux" {
		t.Skip("服务安装和卸载测试只在 Linux 系统上运行")
	}

	// 检查是否以 root 权限运行
	if os.Geteuid() != 0 {
		t.Skip("服务安装和卸载测试需要 root 权限")
	}

	// 注意：这些测试会实际修改系统服务配置，所以默认跳过
	// 如果需要运行这些测试，请取消下面的跳过语句
	t.Skip("服务安装和卸载测试会修改系统配置，默认跳过")

	// 测试安装服务
	err := Install()
	if err != nil {
		t.Errorf("安装服务失败: %v", err)
	}

	// 测试卸载服务
	err = Uninstall()
	if err != nil {
		t.Errorf("卸载服务失败: %v", err)
	}
}
