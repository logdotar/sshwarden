// Package service 提供服务安装和卸载功能
package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Install 安装 sshwarden 为系统服务
func Install() error {
	// 检查是否以 root 权限运行
	if os.Geteuid() != 0 {
		return fmt.Errorf("安装服务需要 root 权限，请使用 sudo 运行")
	}

	// 获取可执行文件路径
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}

	// 获取可执行文件所在目录
	workDir := filepath.Dir(execPath)

	// 确保服务文件目录存在
	serviceDir := "/etc/systemd/system"
	if _, err := os.Stat(serviceDir); os.IsNotExist(err) {
		return fmt.Errorf("系统服务目录不存在，可能不是 systemd 系统: %w", err)
	}

	// 创建服务文件内容，确保路径被正确引用
	serviceContent := fmt.Sprintf(`# SSH Guardian (sshwarden) Systemd Service Configuration
# 用于监控 SSH 登录尝试并自动封禁恶意 IP 的守护服务
# Author: logdotar
[Unit]
Description=sshwarden Service
Documentation=https://github.com/logdotar/sshwarden.git
After=network.target network-online.target
Wants=network-online.target
StartLimitIntervalSec=60
StartLimitBurst=5

[Service]
Type=simple
User=root
Group=root
ExecStart=%s
WorkingDirectory=%s
Restart=on-failure
RestartSec=10s
TimeoutStartSec=30s
TimeoutStopSec=30s

StandardOutput=null
StandardError=null

LimitNOFILE=65536

# Security Hardening
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=%s
NoNewPrivileges=false
ProtectKernelTunables=true
ProtectControlGroups=true
ProtectKernelModules=true
ProtectKernelLogs=true
ProtectClock=true
ProtectHostname=true
RestrictSUIDSGID=true
RestrictRealtime=true
RestrictNamespaces=true
LockPersonality=true
MemoryDenyWriteExecute=true
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
SystemCallFilter=@system-service

[Install]
WantedBy=multi-user.target
`, execPath, workDir, workDir)

	// 写入服务文件
	servicePath := "/etc/systemd/system/sshwarden.service"
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("写入服务文件失败: %w", err)
	}

	// 重新加载 systemd 配置
	if err := runCommand("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("重新加载 systemd 配置失败: %w", err)
	}

	// 启用服务
	if err := runCommand("systemctl", "enable", "sshwarden"); err != nil {
		return fmt.Errorf("启用服务失败: %w", err)
	}

	// 启动服务
	if err := runCommand("systemctl", "start", "sshwarden"); err != nil {
		return fmt.Errorf("启动服务失败: %w", err)
	}

	return nil
}

// Uninstall 卸载 sshwarden 系统服务
func Uninstall() error {
	// 检查是否以 root 权限运行
	if os.Geteuid() != 0 {
		return fmt.Errorf("卸载服务需要 root 权限，请使用 sudo 运行")
	}

	// 停止服务
	if err := runCommand("systemctl", "stop", "sshwarden"); err != nil {
		// 忽略错误，可能服务未运行
		_ = err
	}

	// 禁用服务
	if err := runCommand("systemctl", "disable", "sshwarden"); err != nil {
		// 忽略错误，可能服务未启用
		_ = err
	}

	// 删除服务文件
	servicePath := "/etc/systemd/system/sshwarden.service"
	if _, err := os.Stat(servicePath); !os.IsNotExist(err) {
		if err := os.Remove(servicePath); err != nil {
			return fmt.Errorf("删除服务文件失败: %w", err)
		}
	}

	// 重新加载 systemd 配置
	if err := runCommand("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("重新加载 systemd 配置失败: %w", err)
	}

	return nil
}

// runCommand 执行系统命令
func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("执行命令 %s %v 失败: %w, 输出: %s", name, args, err, string(output))
	}
	return nil
}
