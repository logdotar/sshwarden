package config

import (
	"os"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestConfigLoad(t *testing.T) {
	// 创建临时配置文件
	tempFile, err := os.CreateTemp("", "config-*.toml")
	if err != nil {
		t.Fatalf("创建临时配置文件失败: %v", err)
	}
	defer func() {
		if err := os.Remove(tempFile.Name()); err != nil {
			t.Logf("清理临时文件失败: %v", err)
		}
	}()

	// 写入测试配置
	configContent := `
[ssh]
logpath = "/var/log/auth.log"
maxfailures = 5
blockedipsfile = "blockedips.json"
regex = [
    "authentication failure;.*rhost=(\\S+)",
    "Failed password for .* from (\\S+)"
]
ignoreip = [
    "192.168.1.1",
    "192.168.0.0/16"
]
findtime = "10m"
bantime = "1h"

[log]
level = "info"
filename = "ssh2ban.log"
maxsize = 50
maxbackups = 5
maxage = 7
localtime = true
compress = true

[firewall]
type = "iptables"
export_iptables_rules = true
load_iptables_rules = true

[ip]
region_db_path = "ip2region_v4.xdb"
`

	if _, err := tempFile.WriteString(configContent); err != nil {
		t.Fatalf("写入配置文件失败: %v", err)
	}
	if err := tempFile.Close(); err != nil {
		t.Fatalf("关闭临时文件失败: %v", err)
	}

	// 加载配置
	logger, err := zap.NewProduction()
	if err != nil {
		t.Fatalf("创建日志器失败: %v", err)
	}

	manager := NewManager(logger)
	if err := manager.Load(tempFile.Name()); err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	cfg := manager.Get()

	// 验证配置
	if cfg.SSH.LogPath != "/var/log/auth.log" {
		t.Errorf("期望 logpath 为 /var/log/auth.log, 实际: %s", cfg.SSH.LogPath)
	}

	if cfg.SSH.MaxFailures != 5 {
		t.Errorf("期望 maxfailures 为 5, 实际: %d", cfg.SSH.MaxFailures)
	}

	if len(cfg.SSH.RegexPatterns) != 2 {
		t.Errorf("期望 regex 数量为 2, 实际: %d", len(cfg.SSH.RegexPatterns))
	}

	if len(cfg.SSH.IgnoreIPs) != 2 {
		t.Errorf("期望 ignoreip 数量为 2, 实际: %d", len(cfg.SSH.IgnoreIPs))
	}

	if cfg.Log.Level != "info" {
		t.Errorf("期望 log.level 为 info, 实际: %s", cfg.Log.Level)
	}

	if cfg.Firewall.Type != "iptables" {
		t.Errorf("期望 firewall.type 为 iptables, 实际: %s", cfg.Firewall.Type)
	}

	// 验证 IP 配置
	if cfg.IP.RegionDBPath != "ip2region_v4.xdb" {
		t.Errorf("期望 region_db_path 为 ip2region_v4.xdb, 实际: %s", cfg.IP.RegionDBPath)
	}
}

func TestConfigParseTime(t *testing.T) {
	// 创建测试配置
	cfg := &Config{
		SSH: SSHConfig{
			FindTime: "10m",
			BanTime:  "1h",
		},
	}

	// 测试解析 findtime
	findTime, err := cfg.SSH.ParseFindTime()
	if err != nil {
		t.Fatalf("解析 findtime 失败: %v", err)
	}
	if findTime != 10*60*time.Second {
		t.Errorf("期望 findtime 为 10 分钟, 实际: %v", findTime)
	}

	// 测试解析 bantime
	banTime, err := cfg.SSH.ParseBanTime()
	if err != nil {
		t.Fatalf("解析 bantime 失败: %v", err)
	}
	if banTime != 60*60*time.Second {
		t.Errorf("期望 bantime 为 1 小时, 实际: %v", banTime)
	}

	// 测试永久封禁
	cfg.SSH.BanTime = "-1"
	banTime, err = cfg.SSH.ParseBanTime()
	if err != nil {
		t.Fatalf("解析永久封禁失败: %v", err)
	}
	if banTime != -1 {
		t.Errorf("期望 bantime 为 -1, 实际: %d", banTime)
	}
}

func TestConfigEmailAlert(t *testing.T) {
	// 创建临时配置文件，包含邮件告警配置
	tempFile, err := os.CreateTemp("", "config-email-*.toml")
	if err != nil {
		t.Fatalf("创建临时配置文件失败: %v", err)
	}
	defer func() {
		if err := os.Remove(tempFile.Name()); err != nil {
			t.Logf("清理临时文件失败: %v", err)
		}
	}()

	// 写入测试配置
	configContent := `
[ssh]
logpath = "/var/log/auth.log"
maxfailures = 5
blockedipsfile = "blockedips.json"
regex = [
    "authentication failure;.*rhost=(\\S+)",
    "Failed password for .* from (\\S+)"
]
ignoreip = [
    "192.168.1.1",
    "192.168.0.0/16"
]
findtime = "10m"
bantime = "1h"

[log]
level = "info"
filename = "ssh2ban.log"

[firewall]
type = "iptables"

[ip]
region_db_path = "ip2region_v4.xdb"

[alert.email]
enabled = true
smtp_host = "smtp.example.com"
smtp_port = "587"
username = "user@example.com"
password = "password"
from = "from@example.com"
to = ["to@example.com"]
subject = "sshwarden 告警"
`

	if _, err := tempFile.WriteString(configContent); err != nil {
		t.Fatalf("写入配置文件失败: %v", err)
	}
	if err := tempFile.Close(); err != nil {
		t.Fatalf("关闭临时文件失败: %v", err)
	}

	// 加载配置
	logger, err := zap.NewProduction()
	if err != nil {
		t.Fatalf("创建日志器失败: %v", err)
	}

	manager := NewManager(logger)
	if err := manager.Load(tempFile.Name()); err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	cfg := manager.Get()

	// 验证邮件告警配置
	if !cfg.Alert.Email.Enabled {
		t.Errorf("期望 email.enabled 为 true, 实际: %v", cfg.Alert.Email.Enabled)
	}

	if cfg.Alert.Email.SMTPHost != "smtp.example.com" {
		t.Errorf("期望 smtp_host 为 smtp.example.com, 实际: %s", cfg.Alert.Email.SMTPHost)
	}

	if cfg.Alert.Email.SMTPPort != "587" {
		t.Errorf("期望 smtp_port 为 587, 实际: %s", cfg.Alert.Email.SMTPPort)
	}

	if cfg.Alert.Email.Username != "user@example.com" {
		t.Errorf("期望 username 为 user@example.com, 实际: %s", cfg.Alert.Email.Username)
	}

	if cfg.Alert.Email.Password != "password" {
		t.Errorf("期望 password 为 password, 实际: %s", cfg.Alert.Email.Password)
	}

	if cfg.Alert.Email.From != "from@example.com" {
		t.Errorf("期望 from 为 from@example.com, 实际: %s", cfg.Alert.Email.From)
	}

	if len(cfg.Alert.Email.To) != 1 || cfg.Alert.Email.To[0] != "to@example.com" {
		t.Errorf("期望 to 为 [\"to@example.com\"], 实际: %v", cfg.Alert.Email.To)
	}

	if cfg.Alert.Email.Subject != "sshwarden 告警" {
		t.Errorf("期望 subject 为 sshwarden 告警, 实际: %s", cfg.Alert.Email.Subject)
	}
}
