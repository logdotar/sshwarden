# sshwarden

一个轻量级的 SSH 登录失败检测和 IP 封禁工具，使用 Go 语言编写。

## 仓库地址

[https://github.com/logdotar/sshwarden.git](https://github.com/logdotar/sshwarden.git)

## 特性更新

- **无防火墙支持模式**：在不支持 iptables 或 firewall-cmd 的系统（如 Windows）上，应用会自动切换到无防火墙支持模式，仍然会记录封禁信息但不会修改防火墙规则

## 功能特性

- 实时监控 SSH 登录日志
- 自动封禁频繁失败的 IP 地址
- 支持基于时间窗口的失败计数（findtime）
- 支持临时封禁和永久封禁（bantime）
- 支持 iptables 和 firewall-cmd 两种防火墙
- 无防火墙支持模式，可在 Windows 等系统上运行
- IP 白名单支持（单个 IP 和 CIDR 网段）
- IP 归属地查询
- 配置文件热重载
- 结构化日志输出
- 支持日志轮转和压缩
- 封禁记录持久化
- 自动清理过期封禁记录
- 主动解除 IP 封禁功能
- 配置热重载时自动更新时间设置
- 优雅的退出机制
- 命令行接口（install/uninstall 服务）
- GitHub 自动构建和发布功能
- 配置文件永久封禁 IP 列表支持
- 支持 IP 段（CIDR）封禁

## 项目结构

```
sshwarden/
├── cmd/
│   └── sshwarden/         # 主程序入口
│       └── main.go
├── internal/
│   ├── alert/               # 告警管理模块
│   ├── banmanager/          # 封禁管理模块
│   ├── config/              # 配置管理模块
│   ├── firewall/            # 防火墙管理模块
│   ├── ipregion/            # IP 归属地查询模块
│   ├── logger/              # 日志管理模块
│   └── whitelist/           # 白名单管理模块
├── docs/                    # 文档目录
│   ├── ARCHITECTURE.md      # 架构设计文档
│   ├── CONFIGURATION.md     # 配置指南
│   └── USAGE.md             # 使用指南
├── scripts/                 # 脚本目录
│   └── sshwarden.service  # Systemd 服务文件
├── config.toml              # 配置文件
├── go.mod
├── go.sum
├── Makefile                 # 构建脚本
└── README.md
```

## 快速开始

### 前置条件

- Go 1.25+ 环境
- IP 归属地数据库（ip2region_v4.xdb，可选）

**注意**：如果系统支持 iptables 或 firewall-cmd，应用会自动使用它们进行 IP 封禁；如果不支持，应用会切换到无防火墙支持模式，仍然会记录封禁信息但不会修改防火墙规则。

### 编译

使用 Go 命令编译：

```bash
go build ./cmd/sshwarden
```

或使用 Makefile：

```bash
make build
```

### 配置

编辑 `config.toml` 文件：

```toml
[ssh]
# SSH 日志文件路径
logpath = "/var/log/auth.log"
# 最大失败次数，超过此值将封禁 IP
maxfailures = 5
# 封禁 IP 记录文件
blockedipsfile = "blockedips.json"
# 登录失败匹配正则表达式
regex = [
    "authentication failure;.*rhost=(\\S+)",
    "Failed password for .* from (\\S+)"
]
# 白名单 IP 和 CIDR
ignoreip = [
    "192.168.1.1",
    "192.168.0.0/16"
]
# 检测时间窗口，在此时间内的失败次数会被累计
findtime = "10m"
# 封禁时间，-1 表示永久封禁
bantime = "1h"
# 永久封禁的 IP 列表，支持单个 IP 和 CIDR 网段
permanentblockip = [
    "192.168.1.100",
    "10.0.0.50",
    "172.16.0.0/12"
]

[log]
# 日志级别: debug, info, warn, error
level = "info"
# 日志文件路径
filename = "sshwarden.log"
# 日志文件最大大小（MB）
maxsize = 50
# 最大备份文件数
maxbackups = 5
# 日志文件保留天数
maxage = 7
# 是否使用本地时间
localtime = true
# 是否压缩旧日志
compress = true

[firewall]
# 防火墙类型: iptables 或 firewall-cmd
type = "iptables"
# 是否导出 iptables 规则
export_iptables_rules = true
# 是否加载 iptables 规则
load_iptables_rules = true

[ip]
# IP 归属地数据库路径
region_db_path = "ip2region_v4.xdb"

[alert]
[alert.email]
# 是否启用邮件告警
enabled = false
# SMTP服务器地址
smtp_host = "smtp.example.com"
# SMTP服务器端口
smtp_port = "587"
# SMTP用户名
username = "user@example.com"
# SMTP密码
password = "your_password"
# 发件人邮箱
from = "sshwarden@example.com"
# 收件人邮箱（多个用逗号分隔）
to = ["admin@example.com"]
# 邮件主题
subject = "sshwarden 告警"
```

### 运行

直接运行：

```bash
./sshwarden
```

或使用 Makefile：

```bash
make run
```

### 命令行接口

sshwarden 提供了以下命令行命令：

- **install**：安装 sshwarden 为系统服务
  ```bash
  sudo ./sshwarden install
  ```

- **uninstall**：卸载 sshwarden 系统服务
  ```bash
  sudo ./sshwarden uninstall
  ```

- **uninstall**：卸载 sshwarden 系统服务
  ```bash
  sudo ./sshwarden uninstall
  ```

- **block**：封禁指定 IP 地址或网段
  ```bash
  # 临时封禁指定 IP（使用配置文件中的封禁时间）
  ./sshwarden block 192.168.1.100
  
  # 永久封禁指定 IP
  ./sshwarden block --permanent 192.168.1.100
  
  # 封禁指定网段
  ./sshwarden block 192.168.1.0/24
  ```

- **unblock**：解除指定 IP 地址或网段的封禁
  ```bash
  # 解除指定 IP 的封禁
  ./sshwarden unblock 192.168.1.100
  
  # 解除指定网段的封禁
  ./sshwarden unblock 192.168.1.0/24
  ```

## 部署指南

### 作为系统服务运行

#### Systemd 服务

1. 创建服务文件：

```bash
sudo nano /etc/systemd/system/sshwarden.service
```

2. 写入以下内容：

```ini
[Unit]
Description=sshwarden Service
After=network.target

[Service]
ExecStart=/path/to/sshwarden
WorkingDirectory=/path/to/sshwarden
Restart=always
RestartSec=10
User=root

[Install]
WantedBy=multi-user.target
```

3. 启动服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable sshwarden
sudo systemctl start sshwarden
```

### 容器化部署

#### Docker 部署

创建 `Dockerfile`：

```dockerfile
FROM golang:1.25 AS builder
WORKDIR /app
COPY . .
RUN go build ./cmd/sshwarden

FROM debian:bookworm-slim
WORKDIR /app
COPY --from=builder /app/sshwarden .
COPY config.toml .
COPY ip2region_v4.xdb .

RUN apt-get update && apt-get install -y iptables

CMD ["./sshwarden"]
```

### GitHub 自动构建

sshwarden 配置了完整的 GitHub Actions CI/CD 流程：

- **构建和测试**：当代码推送到 main/master 分支或创建 pull request 时，会在 Ubuntu 24.04 平台上执行：
  - Golangci-lint 代码质量检查
  - 应用构建
  - 带竞争检测（race）的单元测试

- **自动版本管理**：使用 Google Release Please 自动分析提交历史，生成版本号、CHANGELOG 并创建发布 PR

- **多平台构建**：当发布 PR 合并后，使用 GoReleaser 自动构建以下平台的二进制文件：
  - Linux (amd64)

- **自动发布**：构建完成后自动创建 GitHub Release，包含所有平台的预构建二进制文件

## 模块说明

### config

配置管理模块，负责加载、验证和监听配置文件变化。

### logger

日志管理模块，提供结构化日志输出，支持日志轮转和压缩。

### firewall

防火墙管理模块，提供统一的防火墙接口，支持 iptables 和 firewall-cmd。

### whitelist

白名单管理模块，支持单个 IP 和 CIDR 网段的白名单配置。

### ipregion

IP 归属地查询模块，使用 ip2region 数据库查询 IP 所属地区。

### banmanager

封禁管理模块，负责记录失败次数、管理封禁 IP、主动解除封禁和持久化封禁记录。

### alert

告警管理模块，负责发送邮件告警，支持封禁和解除封禁的通知。

### service

服务管理模块，负责安装和卸载 sshwarden 系统服务。

## 常见问题

### 1. 为什么 IP 没有被封禁？

- 检查配置文件中的 `maxfailures` 设置
- 检查日志路径是否正确
- 检查正则表达式是否匹配你的 SSH 日志格式
- 检查 IP 是否在白名单中

### 2. 如何解除被封禁的 IP？

- 对于 iptables：
  ```bash
  sudo iptables -D INPUT -s <IP> -j DROP
  ```

- 对于 firewall-cmd：
  ```bash
  sudo firewall-cmd --zone=public --remove-rich-rule 'rule family="ipv4" source address="<IP>" reject' --permanent
  sudo firewall-cmd --reload
  ```

- 从 `blockedips.json` 文件中移除该 IP

### 3. 如何测试 sshwarden 是否工作？

可以使用 `ssh` 命令故意输错密码来测试：

```bash
ssh -o PasswordAuthentication=yes -o PreferredAuthentications=password -o PubkeyAuthentication=no test@your-server
```

## 性能优化

- 调整日志级别为 `info` 或 `warn` 以减少日志输出
- 合理设置 `maxfailures` 值，避免误封
- 确保 IP 归属地数据库文件存在且可读

## 许可证

MIT License
