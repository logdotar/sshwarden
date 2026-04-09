# 使用指南

本文档提供 sshwarden 的详细使用指南。

## 基本使用

### 1. 编译和安装

```bash
# 克隆仓库
git clone https://github.com/logdotar/sshwarden.git
cd sshwarden

# 编译
make build

# 或使用 Go 命令
go build ./cmd/sshwarden
```

### 2. 无防火墙支持模式

sshwarden 支持在没有 iptables 或 firewall-cmd 的系统上运行：

- 当系统不支持配置的防火墙类型时，应用会自动切换到无防火墙支持模式
- 无防火墙支持模式下，应用会：
  - 继续监控 SSH 登录日志
  - 记录失败次数
  - 维护封禁记录
  - 但不会修改防火墙规则
- 这使得应用可以在 Windows 等系统上运行，作为一个监控工具使用

### 3. 配置

编辑 `config.toml` 文件，根据你的环境进行配置：

```toml
[ssh]
logpath = "/var/log/auth.log"
maxfailures = 5

[firewall]
type = "iptables"  # 或 "firewall-cmd"
# 是否使用 ipset 来提高性能（仅对 iptables 有效）
# use_ipset = true
# ipset_name = "ssh_guardian_blacklist"
```

### 4. 运行

```bash
# 直接运行
./sshwarden

# 或使用 Makefile
make run
```

### 5. 主动封禁和解除 IP 封禁

#### 5.1 主动封禁 IP

当需要手动封禁某个 IP 时，可以使用以下命令：

```bash
# 临时封禁指定 IP（使用配置文件中的封禁时间）
sshwarden block 192.168.1.100

# 永久封禁指定 IP
sshwarden block --permanent 192.168.1.100
```

#### 5.2 解除 IP 封禁

当需要手动解除某个 IP 或网段的封禁时，可以使用以下命令：

```bash
# 解除指定 IP 的封禁
sshwarden unblock 192.168.1.100

# 解除指定网段的封禁
sshwarden unblock 192.168.1.0/24
```

### 6. 配置热重载

项目支持配置文件热重载，当修改配置文件后，系统会自动重新加载配置，包括：
- 白名单 IP
- 正则表达式模式
- 时间窗口设置 (findtime)
- 封禁时间设置 (bantime)

无需重启应用即可应用新的配置。

### 7. 命令行接口

sshwarden 提供了以下命令行命令：

- **install**：安装 sshwarden 为系统服务
  ```bash
  sudo ./sshwarden install
  ```

- **uninstall**：卸载 sshwarden 系统服务
  ```bash
  sudo ./sshwarden uninstall
  ```

- **block**：封禁指定 IP 地址
  ```bash
  # 临时封禁指定 IP（使用配置文件中的封禁时间）
  ./sshwarden block 192.168.1.100
  
  # 永久封禁指定 IP
  ./sshwarden block --permanent 192.168.1.100
  ```

- **unblock**：解除指定 IP 地址或网段的封禁
  ```bash
  # 解除指定 IP 的封禁
  ./sshwarden unblock 192.168.1.100
  
  # 解除指定网段的封禁
  ./sshwarden unblock 192.168.1.0/24
  ```

## 高级使用

### 查看日志

sshwarden 会在 `sshwarden.log` 文件中记录详细的日志信息：

```bash
# 实时查看日志
tail -f sshwarden.log

# 查看封禁记录
grep "已封禁IP" sshwarden.log

# 查看清理过期封禁记录
grep "清理了过期的封禁记录" sshwarden.log
```

### 时间窗口和封禁时间设置

sshwarden 支持基于时间窗口的失败计数和灵活的封禁时间设置：

1. **时间窗口（findtime）**：
   - 在此时间内的失败次数会被累计
   - 超过时间窗口后，失败计数会重置
   - 例如：`findtime = "10m"` 表示 10 分钟内的失败次数会被累计

2. **封禁时间（bantime）**：
   - 控制 IP 被封禁的时间
   - 支持临时封禁和永久封禁
   - 例如：
     - `bantime = "1h"` 表示封禁 1 小时
     - `bantime = "-1"` 表示永久封禁

3. **永久封禁管理**：
   - 通过命令行可以永久封禁特定 IP，不受配置文件中 bantime 的影响
   - 永久封禁的 IP 不会被配置热重载修改
   - 永久封禁的 IP 不会被自动清理
   - 支持在配置文件中添加永久封禁 IP 名单
   - 例如：
     - `sshwarden block --permanent 192.168.1.100` 永久封禁 IP
     - `sshwarden unblock 192.168.1.100` 解除封禁

4. **配置文件中的永久封禁 IP 名单**：
   - 在 `config.toml` 文件中添加 `permanentblockip` 配置项
   - 支持单个 IP 地址和 CIDR 网段
   - 这些 IP 会在应用启动时自动被永久封禁
   - 配置热重载时会重新加载永久封禁 IP 名单
   - 例如：
     ```toml
     # 永久封禁的 IP 列表，支持单个 IP 和 CIDR 网段
     permanentblockip = [
         "192.168.1.100",
         "10.0.0.50",
         "172.16.0.0/12"
     ]
     ```

### 管理封禁 IP

#### 查看已封禁的 IP

```bash
# 查看 blockedips.json 文件
cat blockedips.json

# 或从日志中查看
grep "已封禁IP" sshwarden.log
```

#### 解除封禁

1. **从防火墙中移除**：

   - iptables：
     ```bash
     sudo iptables -D INPUT -s <IP> -j DROP
     ```

   - firewall-cmd：
     ```bash
     sudo firewall-cmd --zone=public --remove-rich-rule 'rule family="ipv4" source address="<IP>" reject' --permanent
     sudo firewall-cmd --reload
     ```

2. **从封禁记录中移除**：

   编辑 `blockedips.json` 文件，移除对应的 IP 条目。

### 测试 sshwarden

#### 测试封禁功能

使用 `ssh` 命令故意输错密码来测试：

```bash
# 连续输错密码 5 次（或配置的 maxfailures 值）
for i in {1..5}; do
    ssh -o PasswordAuthentication=yes -o PreferredAuthentications=password -o PubkeyAuthentication=no test@your-server
    sleep 1
done
```

#### 检查是否被封禁

```bash
# 尝试 SSH 连接
ssh your-server

# 查看防火墙规则
sudo iptables -L INPUT -n
# 或
sudo firewall-cmd --list-all
```

### 配置热重载

修改 `config.toml` 文件后，sshwarden 会自动加载新的配置：

```bash
# 修改配置文件
nano config.toml

# 查看日志确认配置已重载
tail -f sshwarden.log
```

### ipset 支持

当需要封禁大量 IP 或网段时，推荐使用 ipset 来提高性能（仅对 iptables 有效）：

1. **启用 ipset**：
   ```toml
   [firewall]
   type = "iptables"
   use_ipset = true
   ipset_name = "ssh_guardian_blacklist"
   ```

2. **ipset 优势**：
   - 使用哈希表存储 IP 和网段，匹配速度极快
   - 即使封禁数千个 IP 或网段，也不会影响网络性能
   - 自动管理 ipset 集合和相关的 iptables 规则

3. **注意事项**：
   - ipset 仅在 iptables 模式下有效
   - 系统需要安装 ipset 工具
   - 首次启用时会自动创建 ipset 集合和相关规则

## 监控和维护

### 监控 sshwarden 状态

#### 作为系统服务运行时

```bash
# 查看状态
sudo systemctl status sshwarden

# 查看日志
sudo journalctl -u sshwarden
```

#### 手动运行时

```bash
# 查看进程是否运行
ps aux | grep sshwarden

# 查看日志
tail -f sshwarden.log
```

### 定期维护

1. **清理旧的封禁记录**：
   ```bash
   # 编辑 blockedips.json 文件，移除过期的封禁记录
   ```

2. **备份配置和封禁记录**：
   ```bash
   # 备份配置文件和封禁记录
   cp config.toml config.toml.bak
   cp blockedips.json blockedips.json.bak
   ```

3. **更新 IP 归属地数据库**：
   ```bash
   # 下载最新的 ip2region 数据库
   wget https://github.com/lionsoul2014/ip2region/raw/master/data/ip2region.xdb -O ip2region_v4.xdb
   ```

## 故障排除

### 常见问题

#### 1. sshwarden 启动失败

- **检查日志**：查看 `sshwarden.log` 文件中的错误信息
- **检查权限**：确保以 root 权限运行
- **检查配置**：验证 `config.toml` 文件格式是否正确
- **检查依赖**：确保防火墙工具（iptables 或 firewall-cmd）已安装

#### 2. IP 没有被封禁

- **检查日志路径**：确保 `logpath` 指向正确的 SSH 日志文件
- **检查正则表达式**：验证 `regex` 配置是否匹配你的 SSH 日志格式
- **检查白名单**：确认 IP 不在 `ignoreip` 列表中
- **检查失败次数**：确认失败次数达到了 `maxfailures` 值

#### 3. 防火墙规则不生效

- **检查防火墙服务**：确保防火墙服务正在运行
- **检查权限**：确保 sshwarden 有足够的权限修改防火墙规则
- **检查防火墙类型**：确认 `firewall.type` 配置正确

#### 4. IP 归属地查询失败

- **检查数据库文件**：确保 `ip2region_v4.xdb` 文件存在且可读
- **检查文件路径**：验证 `region_db_path` 配置正确

### 日志分析

sshwarden 的日志格式如下：

```
[sshwarden] 2024-01-01 12:00:00 INFO 记录第一次登录失败 {"ip": "192.168.1.100", "failures": 1, "region": "中国|0|广东省|深圳市|阿里云"}
[sshwarden] 2024-01-01 12:00:05 INFO IP登录失败次数增加 {"ip": "192.168.1.100", "failures": 2, "region": "中国|0|广东省|深圳市|阿里云"}
[sshwarden] 2024-01-01 12:00:10 INFO IP达到最大失败次数，正在封禁 {"ip": "192.168.1.100", "region": "中国|0|广东省|深圳市|阿里云"}
[sshwarden] 2024-01-01 12:00:10 INFO 已封禁IP {"ip": "192.168.1.100"}
```

通过分析日志，你可以了解：
- 哪些 IP 尝试登录失败
- IP 的归属地信息
- 封禁的具体时间
- 任何错误信息

## 最佳实践

1. **合理设置阈值**：
   - `maxfailures`：建议设置为 5-10 次
   - `bantime`：建议设置为 1-24 小时
   - `findtime`：建议设置为 10-30 分钟

2. **配置白名单**：
   - 将你的管理 IP 添加到白名单
   - 将内部网络 CIDR 添加到白名单

3. **优化日志级别**：
   - 生产环境：使用 `info` 或 `warn` 级别
   - 调试时：使用 `debug` 级别

4. **定期更新**：
   - 定期更新 IP 归属地数据库
   - 定期检查和清理封禁记录

5. **监控**：
   - 将 sshwarden 作为系统服务运行
   - 监控 sshwarden 的运行状态
   - 设置日志告警

## 安全注意事项

1. **权限管理**：
   - 以最小必要权限运行 sshwarden
   - 保护配置文件和封禁记录文件

2. **防止误封**：
   - 合理设置 `maxfailures` 值
   - 确保白名单配置正确
   - 定期检查封禁记录

3. **性能考虑**：
   - 避免使用过于复杂的正则表达式
   - 合理设置日志级别和日志轮转
   - 监控系统资源使用情况
