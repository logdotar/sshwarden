// Package main 是 sshwarden 的主程序入口
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hpcloud/tail"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/logdotar/sshwarden/internal/alert"
	"github.com/logdotar/sshwarden/internal/banmanager"
	"github.com/logdotar/sshwarden/internal/config"
	"github.com/logdotar/sshwarden/internal/firewall"
	"github.com/logdotar/sshwarden/internal/ipregion"
	"github.com/logdotar/sshwarden/internal/logger"
	"github.com/logdotar/sshwarden/internal/service"
	"github.com/logdotar/sshwarden/internal/whitelist"
)

// noopFirewallManager 空操作防火墙管理器，用于在无法初始化真实防火墙时使用
type noopFirewallManager struct {
	logger *zap.Logger
}

// BlockIP 封禁 IP (空操作)
func (m *noopFirewallManager) BlockIP(ip string) error {
	m.logger.Info("无防火墙支持，跳过封禁 IP", zap.String("ip", ip))
	return nil
}

// UnblockIP 解除封禁 IP (空操作)
func (m *noopFirewallManager) UnblockIP(ip string) error {
	m.logger.Info("无防火墙支持，跳过解除封禁 IP", zap.String("ip", ip))
	return nil
}

// RestoreRules 恢复防火墙规则 (空操作)
func (m *noopFirewallManager) RestoreRules() error {
	m.logger.Info("无防火墙支持，跳过恢复防火墙规则")
	return nil
}

// Close 关闭防火墙管理器 (空操作)
func (m *noopFirewallManager) Close() error {
	return nil
}

var version = "dev"

// App 是 sshwarden 应用的主结构体，包含所有模块的管理器
//
// 字段说明：
// - logger: 日志管理器
// - configManager: 配置管理器
// - cfg: 配置信息
// - firewallMgr: 防火墙管理器
// - whitelistMgr: 白名单管理器
// - ipRegionMgr: IP 归属地管理器
// - banMgr: 封禁管理器
// - emailAlert: 邮件告警管理器
type App struct {
	logger        *zap.Logger
	configManager *config.Manager
	cfg           *config.Config
	firewallMgr   firewall.Manager
	whitelistMgr  *whitelist.Manager
	ipRegionMgr   *ipregion.Manager
	banMgr        *banmanager.Manager
	emailAlert    *alert.EmailAlert
}

// NewApp 创建并初始化 sshwarden 应用实例
//
// 返回值:
// - *App: 初始化成功的应用实例
// - error: 初始化过程中的错误
func NewApp() (*App, error) {
	app := &App{}

	tempLogger, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("创建临时日志器失败: %w", err)
	}
	app.configManager = config.NewManager(tempLogger)

	if err := app.configManager.Load("config.toml"); err != nil {
		return nil, err
	}
	app.cfg = app.configManager.Get()

	app.logger, err = logger.New(&app.cfg.Log)
	if err != nil {
		return nil, fmt.Errorf("初始化日志器失败: %w", err)
	}

	app.firewallMgr, err = firewall.NewManager(&app.cfg.Firewall, app.logger)
	if err != nil {
		app.logger.Warn("初始化防火墙管理器失败，将在无防火墙支持模式下运行", zap.Error(err))
		// 创建一个空的防火墙管理器，实现 Manager 接口但不做任何操作
		app.firewallMgr = &noopFirewallManager{logger: app.logger}
	}

	app.whitelistMgr = whitelist.NewManager(app.logger)
	app.whitelistMgr.Load(app.cfg.SSH.IgnoreIPs)

	app.ipRegionMgr = ipregion.NewManager(&app.cfg.IP, app.logger)
	if err := app.ipRegionMgr.Init(); err != nil {
		app.logger.Warn("IP归属地查询模块初始化失败，将继续运行", zap.Error(err))
	}

	findTime, err := app.cfg.SSH.ParseFindTime()
	if err != nil {
		app.logger.Warn("解析 findtime 失败，使用默认值 10 分钟", zap.Error(err))
		findTime = 10 * time.Minute
	}

	banTime, err := app.cfg.SSH.ParseBanTime()
	if err != nil {
		app.logger.Warn("解析 bantime 失败，使用默认值 10 分钟", zap.Error(err))
		banTime = 10 * time.Minute
	}

	app.banMgr = banmanager.NewManager(app.cfg.SSH.BlockedIPsFile, findTime, banTime, app.firewallMgr, app.logger)
	if err := app.banMgr.LoadRegexPatterns(app.cfg.SSH.RegexPatterns); err != nil {
		return nil, fmt.Errorf("加载正则表达式失败: %w", err)
	}

	if err := app.banMgr.LoadBlockedIPs(); err != nil {
		app.logger.Warn("加载已封禁IP失败", zap.Error(err))
	}

	// 加载配置文件中的永久封禁 IP 列表
	app.banMgr.LoadPermanentBlockIPs(app.cfg.SSH.PermanentBlockIPs, app.whitelistMgr)

	// 初始化邮件告警
	if app.cfg.Alert.Email.Enabled {
		app.emailAlert = alert.NewEmailAlert(
			app.cfg.Alert.Email.SMTPHost,
			app.cfg.Alert.Email.SMTPPort,
			app.cfg.Alert.Email.Username,
			app.cfg.Alert.Email.Password,
			app.cfg.Alert.Email.From,
			app.cfg.Alert.Email.To,
			app.cfg.Alert.Email.Subject,
			app.logger,
		)
		app.logger.Info("邮件告警功能已启用")
	}

	app.configManager.Watch(app.onConfigChange)

	return app, nil
}

// onConfigChange 处理配置文件更新事件
//
// 当配置文件发生变化时，此函数会被调用，重新加载配置并更新相关模块的设置
func (a *App) onConfigChange() {
	a.logger.Info("配置已更新，重新加载...")
	a.cfg = a.configManager.Get()
	a.whitelistMgr.Load(a.cfg.SSH.IgnoreIPs)
	if err := a.banMgr.LoadRegexPatterns(a.cfg.SSH.RegexPatterns); err != nil {
		a.logger.Error("重新加载正则表达式失败", zap.Error(err))
	}

	// 重新解析 findtime 和 bantime
	findTime, err := a.cfg.SSH.ParseFindTime()
	if err != nil {
		a.logger.Warn("解析 findtime 失败，使用默认值 10 分钟", zap.Error(err))
		findTime = 10 * time.Minute
	}

	banTime, err := a.cfg.SSH.ParseBanTime()
	if err != nil {
		a.logger.Warn("解析 bantime 失败，使用默认值 10 分钟", zap.Error(err))
		banTime = 10 * time.Minute
	}

	// 更新 banmanager 中的 findtime 和 bantime
	a.banMgr.UpdateTimeSettings(findTime, banTime)

	// 重新加载配置文件中的永久封禁 IP 列表
	a.banMgr.LoadPermanentBlockIPs(a.cfg.SSH.PermanentBlockIPs, a.whitelistMgr)

	// 更新邮件告警配置
	if a.cfg.Alert.Email.Enabled {
		a.emailAlert = alert.NewEmailAlert(
			a.cfg.Alert.Email.SMTPHost,
			a.cfg.Alert.Email.SMTPPort,
			a.cfg.Alert.Email.Username,
			a.cfg.Alert.Email.Password,
			a.cfg.Alert.Email.From,
			a.cfg.Alert.Email.To,
			a.cfg.Alert.Email.Subject,
			a.logger,
		)
		a.logger.Info("邮件告警功能已更新")
	} else {
		a.emailAlert = nil
		a.logger.Info("邮件告警功能已禁用")
	}

	a.logger.Info("配置热重载完成")
}

// Run 启动 sshwarden 应用的主运行循环
//
// 参数:
// - ctx: 上下文，用于控制应用的生命周期
//
// 返回值:
// - error: 运行过程中的错误
func (a *App) Run(ctx context.Context) error {
	a.logger.Info("sshwarden 已启动", zap.String("version", version))
	defer func() { _ = a.logger.Sync() }()

	ips, cidrs := a.whitelistMgr.List()
	for _, ip := range ips {
		a.logger.Info("白名单IP", zap.String("ip", ip))
	}
	for _, cidr := range cidrs {
		a.logger.Info("白名单CIDR", zap.String("cidr", cidr))
	}

	for _, ip := range a.banMgr.GetBlockedIPs() {
		a.logger.Info("已封禁IP", zap.String("ip", ip))
	}

	if a.cfg.Firewall.Type == config.FirewallTypeIptables && a.cfg.Firewall.LoadIptables {
		if err := a.firewallMgr.RestoreRules(); err != nil {
			a.logger.Warn("恢复iptables规则失败", zap.Error(err))
		}
	}

	t, err := tail.TailFile(a.cfg.SSH.LogPath, tail.Config{
		Follow: true,
		ReOpen: true,
		Poll:   true,
	})
	if err != nil {
		return fmt.Errorf("跟踪日志文件失败: %w", err)
	}
	defer func() { _ = t.Stop() }()

	errCh := make(chan error, 1)
	go a.processLogLines(ctx, t, errCh)

	// 定期清理过期的封禁记录
	cleanupTicker := time.NewTicker(5 * time.Minute)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("接收到终止信号，正在优雅退出...")
			return nil
		case err := <-errCh:
			if err != nil {
				a.logger.Error("处理日志时出错", zap.Error(err))
				return err
			}
			return nil
		case <-cleanupTicker.C:
			expiredCount := a.banMgr.CleanupExpired()
			if expiredCount > 0 {
				a.logger.Info("清理了过期的封禁记录", zap.Int("count", expiredCount))
			}
		}
	}
}

// processLogLines 处理日志文件中的每一行，检测 SSH 登录失败并执行封禁操作
//
// 参数:
// - ctx: 上下文，用于控制 goroutine 的生命周期
// - t: 日志文件跟踪器
// - errCh: 错误通道，用于返回处理过程中的错误
func (a *App) processLogLines(ctx context.Context, t *tail.Tail, errCh chan<- error) {
	for {
		select {
		case line, ok := <-t.Lines:
			if !ok {
				a.logger.Info("日志文件通道已关闭")
				errCh <- nil
				return
			}

			ip := a.banMgr.ExtractIP(line.Text)
			if ip == "" {
				continue
			}

			region := "unknown"
			if a.ipRegionMgr != nil {
				var err error
				region, err = a.ipRegionMgr.GetRegion(ip)
				if err != nil {
					a.logger.Debug("查询IP归属地失败", zap.String("ip", ip), zap.Error(err))
				}
			}

			if a.whitelistMgr.Contains(ip) {
				a.logger.Debug("IP在白名单中，跳过处理", zap.String("ip", ip), zap.String("region", region))
				continue
			}

			if a.banMgr.IsBlocked(ip) {
				a.logger.Debug("IP已被封禁，跳过处理", zap.String("ip", ip), zap.String("region", region))
				continue
			}

			failures := a.banMgr.IncrementFailure(ip)
			if failures == 1 {
				a.logger.Info("记录第一次登录失败", zap.String("ip", ip), zap.Int("failures", failures), zap.String("region", region))
			} else {
				a.logger.Info("IP登录失败次数增加", zap.String("ip", ip), zap.Int("failures", failures), zap.String("region", region))
			}

			if failures >= a.cfg.SSH.MaxFailures {
				a.logger.Info("IP达到最大失败次数，正在封禁", zap.String("ip", ip), zap.String("region", region))
				if err := a.firewallMgr.BlockIP(ip); err != nil {
					a.logger.Error("封禁IP失败", zap.String("ip", ip), zap.Error(err))
					continue
				}
				if err := a.banMgr.BlockIP(ip); err != nil {
					a.logger.Error("更新封禁记录失败", zap.String("ip", ip), zap.Error(err))
				}
				// 发送邮件告警
				if a.emailAlert != nil {
					go func(ip, region string) {
						if err := a.emailAlert.SendBanAlert(ip, region); err != nil {
							a.logger.Warn("发送封禁邮件告警失败", zap.Error(err))
						}
					}(ip, region)
				}
			}

		case <-ctx.Done():
			a.logger.Debug("日志处理goroutine收到取消信号")
			errCh <- nil
			return
		}
	}
}

// Close 关闭应用并清理资源
//
// 此函数会关闭 IP 归属地查询模块和防火墙管理器，释放相关资源
func (a *App) Close() {
	if a.ipRegionMgr != nil {
		a.ipRegionMgr.Close()
	}
	if a.firewallMgr != nil {
		_ = a.firewallMgr.Close()
	}
	a.logger.Info("清理完成，退出程序")
}

// main 函数是程序的入口点
//
// 它创建应用实例，设置信号处理，并启动应用的主运行循环
func main() {
	rootCmd := &cobra.Command{
		Use:   "sshwarden",
		Short: "SSH 登录失败检测和 IP 封禁工具",
		Long:  `sshwarden 是一个轻量级的 SSH 登录失败检测和 IP 封禁工具，使用 Go 语言编写。`,
		Run: func(cmd *cobra.Command, args []string) {
			app, err := NewApp()
			if err != nil {
				fmt.Printf("初始化应用失败: %v\n", err)
				os.Exit(1)
			}
			defer app.Close()

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			if err := app.Run(ctx); err != nil {
				app.logger.Error("应用运行失败", zap.Error(err))
				os.Exit(1)
			}
		},
	}

	installCmd := &cobra.Command{
		Use:   "install",
		Short: "安装 sshwarden 为系统服务",
		Run: func(cmd *cobra.Command, args []string) {
			if err := service.Install(); err != nil {
				fmt.Printf("安装服务失败: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("sshwarden 服务安装成功")
		},
	}

	uninstallCmd := &cobra.Command{
		Use:   "uninstall",
		Short: "卸载 sshwarden 系统服务",
		Run: func(cmd *cobra.Command, args []string) {
			if err := service.Uninstall(); err != nil {
				fmt.Printf("卸载服务失败: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("sshwarden 服务卸载成功")
		},
	}

	blockCmd := &cobra.Command{
		Use:   "block [ip]",
		Short: "封禁指定 IP 地址",
		Long:  `封禁指定的 IP 地址，可以选择是否永久封禁。`,
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ip := args[0]
			permanent, _ := cmd.Flags().GetBool("permanent")

			app, err := NewApp()
			if err != nil {
				fmt.Printf("初始化应用失败: %v\n", err)
				os.Exit(1)
			}
			defer app.Close()

			if permanent {
				if err := app.banMgr.BlockIPPermanently(ip, app.whitelistMgr); err != nil {
					fmt.Printf("永久封禁 IP 失败: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("已永久封禁 IP: %s\n", ip)
			} else {
				if err := app.firewallMgr.BlockIP(ip); err != nil {
					fmt.Printf("封禁 IP 失败: %v\n", err)
					os.Exit(1)
				}
				if err := app.banMgr.BlockIP(ip); err != nil {
					fmt.Printf("更新封禁记录失败: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("已封禁 IP: %s\n", ip)
			}
		},
	}
	blockCmd.Flags().Bool("permanent", false, "是否永久封禁 IP")

	unblockCmd := &cobra.Command{
		Use:   "unblock [ip]",
		Short: "解除指定 IP 地址的封禁",
		Long:  `解除指定 IP 地址的封禁。`,
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ip := args[0]

			app, err := NewApp()
			if err != nil {
				fmt.Printf("初始化应用失败: %v\n", err)
				os.Exit(1)
			}
			defer app.Close()

			if err := app.banMgr.UnblockIP(ip); err != nil {
				fmt.Printf("解除封禁失败: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("已解除 IP 封禁: %s\n", ip)
		},
	}

	rootCmd.AddCommand(installCmd, uninstallCmd, blockCmd, unblockCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Printf("执行命令失败: %v\n", err)
		os.Exit(1)
	}
}
