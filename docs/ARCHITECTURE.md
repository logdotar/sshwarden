# 架构设计文档

## 概述

sshwarden 采用模块化设计，遵循 Go 语言的最佳实践。项目分为多个内部包，每个包负责特定的功能领域。

## 仓库地址

[https://github.com/logdotar/sshwarden.git](https://github.com/logdotar/sshwarden.git)

## 目录结构

```
sshwarden/
├── cmd/
│   └── sshwarden/
│       └── main.go          # 应用入口
├── internal/
│   ├── alert/               # 告警管理（邮件等）
│   ├── banmanager/          # 封禁管理
│   ├── config/              # 配置管理
│   ├── firewall/            # 防火墙抽象
│   ├── ipregion/            # IP 归属地
│   ├── logger/              # 日志管理
│   └── whitelist/           # 白名单管理
└── docs/                    # 文档
```

## 核心模块

### 1. config 包

**职责**: 
- 加载和解析 TOML 配置文件
- 配置验证
- 配置热重载监听

**主要类型**:
- `Config`: 完整配置结构
- `Manager`: 配置管理器

### 2. logger 包

**职责**:
- 初始化结构化日志
- 日志轮转和压缩
- 统一的日志格式

### 3. firewall 包

**职责**:
- 提供统一的防火墙接口
- 支持 iptables 和 firewall-cmd
- 规则保存和恢复
- 无防火墙支持模式

**接口设计**:
```go
type Manager interface {
    BlockIP(ip string) error
    UnblockIP(ip string) error
    RestoreRules() error
    Close() error
}
```

**无防火墙支持模式**:
- 当系统不支持 iptables 或 firewall-cmd 时，应用会使用空操作的防火墙管理器
- 空操作管理器会记录操作但不会实际修改防火墙规则
- 这样可以确保应用在不同系统上都能正常运行

### 4. whitelist 包

**职责**:
- 管理 IP 白名单
- 支持单个 IP 和 CIDR 网段
- 线程安全的访问

### 5. ipregion 包

**职责**:
- IP 归属地查询
- 数据库加载和释放

### 6. banmanager 包

**职责**:
- 记录登录失败次数
- 管理已封禁 IP
- 主动解除 IP 封禁
- 永久封禁 IP
- 持久化封禁记录
- IP 提取（正则匹配）
- 时间窗口和封禁时间管理
- 配置热重载时更新时间设置
- 永久封禁管理（不受配置热重载影响）

### 7. alert 包

**职责**:
- 邮件告警管理
- 发送封禁和解除封禁告警
- 异步发送邮件，不阻塞主流程

### 8. service 包

**职责**:
- 服务安装和卸载管理
- 注册 sshwarden 为系统服务
- 从系统服务中移除 sshwarden

## 设计原则

1. **单一职责原则**: 每个包只负责一个功能领域
2. **接口隔离**: 防火墙模块使用接口抽象，易于扩展
3. **依赖注入**: 通过构造函数注入依赖，便于测试
4. **线程安全**: 使用 sync.RWMutex 保护共享状态
5. **错误处理**: 所有错误都被正确处理和传播

## 并发模型

- 使用 context.Context 控制 goroutine 生命周期
- 使用 channel 进行 goroutine 间通信
- 使用 sync.RWMutex 保护共享数据结构
