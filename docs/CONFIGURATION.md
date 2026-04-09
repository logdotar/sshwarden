# 配置指南

本文档详细说明 sshwarden 的配置选项。

## 仓库地址

[https://github.com/logdotar/sshwarden.git](https://github.com/logdotar/sshwarden.git)

## 配置文件结构

sshwarden 使用 TOML 格式的配置文件，默认名为 `config.toml`。

## 配置选项

### [ssh] 部分

| 配置项 | 类型 | 默认值 | 描述 |
|-------|------|-------|------|
| `logpath` | string | "/var/log/auth.log" | SSH 日志文件路径 |
| `maxfailures` | int | 5 | 最大失败次数，超过此值将封禁 IP |
| `blockedipsfile` | string | "blockedips.json" | 封禁 IP 记录文件路径 |
| `regex` | []string | ["authentication failure;.*rhost=(\\S+)"] | 登录失败匹配正则表达式 |
| `ignoreip` | []string | [] | 白名单 IP 和 CIDR 网段 |
| `permanentblockip` | []string | [] | 永久封禁的 IP 列表，支持单个 IP 地址和 CIDR 网段，这些 IP 会在应用启动时自动被永久封禁 |
| `findtime` | string | "10m" | 检测时间窗口，在此时间内的失败次数会被累计 |
| `bantime` | string | "10m" | 封禁时间，格式：数字+单位(s/m/h/d)，-1 表示永久 |

### [log] 部分

| 配置项 | 类型 | 默认值 | 描述 |
|-------|------|-------|------|
| `level` | string | "info" | 日志级别：debug, info, warn, error, dpanic, panic, fatal |
| `filename` | string | "sshwarden.log" | 日志文件路径 |
| `maxsize` | int | 100 | 日志文件最大大小（MB） |
| `maxbackups` | int | 3 | 最大备份文件数 |
| `maxage` | int | 7 | 日志文件保留天数 |
| `localtime` | bool | true | 是否使用本地时间 |
| `compress` | bool | true | 是否压缩旧日志 |

### [firewall] 部分

| 配置项 | 类型 | 默认值 | 描述 |
|-------|------|-------|------|
| `type` | string | "iptables" | 防火墙类型：iptables 或 firewall-cmd |
| `export_iptables_rules` | bool | true | 是否导出 iptables 规则到文件 |
| `load_iptables_rules` | bool | true | 是否在启动时加载 iptables 规则 |
| `use_ipset` | bool | false | 是否使用 ipset 来提高性能（仅对 iptables 有效） |
| `ipset_name` | string | "ssh_guardian_blacklist" | ipset 集合名称（仅对 iptables 有效） |

**无防火墙支持模式**:
- 当系统不支持配置的防火墙类型时，应用会自动切换到无防火墙支持模式
- 无防火墙支持模式下，应用会记录封禁信息但不会修改防火墙规则
- 这使得应用可以在 Windows 等不支持 iptables 或 firewall-cmd 的系统上运行

### [ip] 部分

| 配置项 | 类型 | 默认值 | 描述 |
|-------|------|-------|------|
| `region_db_path` | string | "ip2region_v4.xdb" | IP 归属地数据库路径 |

### [alert.email] 部分

| 配置项 | 类型 | 默认值 | 描述 |
|-------|------|-------|------|
| `enabled` | bool | false | 是否启用邮件告警 |
| `smtp_host` | string | "smtp.example.com" | SMTP 服务器地址 |
| `smtp_port` | int | 587 | SMTP 服务器端口 |
| `username` | string | "" | SMTP 用户名 |
| `password` | string | "" | SMTP 密码 |
| `from` | string | "sshwarden@example.com" | 发件人邮箱 |
| `to` | []string | [] | 收件人邮箱列表 |
| `subject` | string | "sshwarden 告警" | 邮件主题 |

## 正则表达式配置

正则表达式用于匹配 SSH 登录失败的日志行，提取出 IP 地址。

### 常用正则表达式

1. **默认规则**：
   ```
   authentication failure;.*rhost=(\S+)
   ```

2. **密码失败**：
   ```
   Failed password for .* from (\S+)
   ```

3. **无效用户**：
   ```
   Invalid user .* from (\S+)
   ```

4. **密钥认证失败**：
   ```
   Failed publickey for .* from (\S+)
   ```

## 时间格式

- `bantime` 和 `findtime` 支持以下格式：
  - 数字 + 单位：如 "10m"（10分钟）、"1h"（1小时）、"1d"（1天）
  - 纯数字：默认为秒，如 "3600"（1小时）
  - "-1"：表示永久封禁

## 白名单配置

白名单支持两种格式：

1. **单个 IP**：如 "192.168.1.1"
2. **CIDR 网段**：如 "192.168.0.0/16"

## 示例配置

```toml
[ssh]
logpath = "/var/log/auth.log"

blockedipsfile = "blockedips.json"
regex = [
    "authentication failure;.*rhost=(\\S+)",
    "Failed password for .* from (\\S+)",
    "Invalid user .* from (\\S+)"
]
ignoreip = [
    "192.168.1.1",
    "192.168.0.0/16",
    "10.0.0.0/8"
]
permanentblockip = [
    "192.168.1.100",
    "10.0.0.50"
]
bantime = "1h"
findtime = "10m"
maxfailures = 5

[log]
level = "info"
filename = "sshwarden.log"
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
```

## 配置热重载

sshwarden 支持配置文件热重载，修改配置文件后会自动加载新的配置，无需重启服务。

### 支持热重载的配置项

- 白名单 IP (`ignoreip`)
- 正则表达式模式 (`regex`)
- 永久封禁 IP 列表 (`permanentblockip`)
- 时间窗口设置 (`findtime`)
- 封禁时间设置 (`bantime`)

## 注意事项

1. **权限**：运行 sshwarden 需要足够的权限来修改防火墙规则，通常需要 root 权限。
2. **性能**：正则表达式越复杂，匹配速度越慢，建议使用简单高效的正则表达式。
3. **安全性**：确保配置文件权限为 644，防止未授权访问。
4. **备份**：定期备份 `blockedips.json` 文件，以防数据丢失。
5. **邮件配置**：如果启用邮件告警，确保 SMTP 服务器配置正确，否则可能会影响主程序性能。
