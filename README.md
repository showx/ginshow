# ginshow

Gin 应用的性能监控组件，一行代码接入 **pprof**、**运行时指标** 和 **请求监控**。

## 安装

```bash
go get github.com/showx/ginshow
```

## 快速开始

```go
package main

import (
	"os"

	"github.com/gin-gonic/gin"
	"github.com/showx/ginshow"
)

func main() {
	r := gin.Default()

	// 推荐：启用账号密码保护（面板内置登录页 + API Basic Auth）
	user := os.Getenv("GINSHOW_USER")
	if user == "" {
		user = "admin"
	}
	pass := os.Getenv("GINSHOW_PASS")
	if pass == "" {
		pass = "ginshow"
	}
	ginshow.Mount(r, ginshow.Production(user, pass))

	r.Run(":8080")
}
```

运行示例：

```bash
go run ./example/
# 默认账号 admin / 密码 ginshow
# 打开 http://localhost:8080/__gs/x7f3a2c9 在登录页输入账号密码
```

本地开发若不需要登录，可使用无认证配置：

```go
ginshow.Mount(r, ginshow.Default())
```

启动后可用以下端点（默认前缀为 `ginshow.DefaultBasePath`，即 `/__gs/x7f3a2c9`）：

| 端点 | 说明 |
|------|------|
| `GET /__gs/x7f3a2c9` | **监控面板**（单文件内嵌 UI，自动刷新） |
| `GET /__gs/x7f3a2c9/pprof/` | pprof 索引页 |
| `GET /__gs/x7f3a2c9/pprof/profile` | CPU 采样 |
| `GET /__gs/x7f3a2c9/pprof/heap` | 堆内存 |
| `GET /__gs/x7f3a2c9/pprof/goroutine` | Goroutine |
| `GET /__gs/x7f3a2c9/pprof/trace` | 执行追踪 |
| `GET /__gs/x7f3a2c9/pprof/flame` | 火焰图 JSON 数据（面板内可视化） |

浏览器打开 **http://localhost:8080/__gs/x7f3a2c9**，在「火焰图」区域选择类型并加载即可交互查看（无需 `go tool pprof`）。

> 默认路径刻意避开 `/debug` 等常见扫描目标。生产环境请**自定义路径**并启用 **Basic Auth**（见下方「安全建议」）。

## 功能

- **内嵌监控面板** — 单 HTML 文件（CSS/JS 内联），Go `embed` 加载，零外部依赖
- **pprof 集成** — 标准 `net/http/pprof` 端点，支持 CPU、heap、goroutine、mutex、block、trace
- **运行时指标** — 内存、GC、goroutine 数量及请求统计，JSON 格式便于对接监控
- **请求监控** — 自动统计 QPS、平均延迟、慢请求；debug 路由不计入统计
- **账号密码登录** — `Production()` 启用后面板显示登录页，metrics / pprof / 火焰图 API 需认证
- **生产环境保护** — 内置 Basic Auth，一行切换生产配置

## 配置

### 默认配置（开发环境）

```go
ginshow.Mount(r, ginshow.Default())
```

默认行为：

- 开启 pprof（前缀 `/__gs/x7f3a2c9/pprof`）
- 开启 metrics（路径 `/__gs/x7f3a2c9/metrics`）
- 开启请求监控中间件
- 慢请求阈值 500ms
- 监控面板路径 `/__gs/x7f3a2c9`

### 安全建议

生产环境至少做到：

1. **自定义路径** — 不要使用默认前缀，改成仅团队知晓的随机路径
2. **启用认证** — 使用 `Production(user, pass)` 开启 Basic Auth
3. **网络隔离** — 如有条件，仅内网或 VPN 可访问

```go
base := "/your-random-path-a8k2m9" // 自行生成，勿提交到公开仓库
cfg := ginshow.Production("admin", os.Getenv("GINSHOW_PASS"))
cfg.DashboardPath = base
cfg.MetricsPath = base + "/metrics"
cfg.PprofPrefix = base + "/pprof"
ginshow.Mount(r, cfg)
```

### 关闭面板

```go
cfg := ginshow.Default()
cfg.EnableDashboard = false
ginshow.Mount(r, cfg)
```

### 生产环境（推荐）

```go
ginshow.Mount(r, ginshow.Production("admin", os.Getenv("GINSHOW_PASS")))
```

- 面板：打开后先显示**登录页**（非浏览器弹窗），登录成功后加载数据
- API：`/metrics`、`/pprof/*`、`/flame` 等接口仍受 HTTP Basic Auth 保护
- 命令行访问 pprof 需携带认证：`http://user:pass@host/.../pprof/heap`

### 自定义配置

```go
cfg := ginshow.Default()
cfg.PprofPrefix = "/your-path/pprof"
cfg.MetricsPath = "/your-path/metrics"
cfg.SlowRequestThreshold = 200 * time.Millisecond
cfg.DashboardPath = "/your-path"
cfg.DashboardTitle = "My App Monitor"
cfg.BlockProfileRate = 1           // 开启 block profiling
cfg.MutexProfileFraction = 1       // 开启 mutex profiling
ginshow.Mount(r, cfg)
```

### 挂载到路由组

若希望 debug 端点挂在特定路由组下：

```go
admin := r.Group("/admin")
ginshow.Attach(admin, ginshow.Production("admin", "secret"))
// 实际路径: /admin/__gs/x7f3a2c9/pprof/* 等（取决于 Config 中的路径配置）
```

## 监控面板

面板由 `dashboard.html` 单文件构成，通过 Go `embed` 嵌入并在运行时渲染，特点：

- 概览卡片：Goroutine、内存、GC、请求统计
- 内存详情表格
- **火焰图**：Heap / CPU / Goroutine 等可视化，支持点击下钻与面包屑返回
- **登录页**：启用认证时先登录再访问数据，支持退出登录
- pprof 快捷链接 + 可调 CPU 采样秒数
- 每 3 秒自动刷新（可关闭）
- 命令行参考（`go tool pprof` / `go tool trace`）

生产环境启用 Basic Auth 后，浏览器打开面板会先进入登录页；登录成功后指标、火焰图等请求会自动携带凭证。

### 火焰图 API

```bash
# 堆内存火焰图（即时返回）
curl "http://localhost:8080{BASE}/pprof/flame?type=heap"

# CPU 火焰图（需等待采样，默认 10 秒，最大 120 秒）
curl "http://localhost:8080{BASE}/pprof/flame?type=cpu&seconds=10"
```

支持 `type`：`cpu`、`heap`、`goroutine`、`allocs`、`block`、`mutex`。

## pprof 使用

将 `{BASE}` 替换为你的实际路径前缀（默认 `/__gs/x7f3a2c9`）：

```bash
# CPU 采样 30 秒
go tool pprof http://localhost:8080{BASE}/pprof/profile?seconds=30

# 堆内存分析
go tool pprof http://localhost:8080{BASE}/pprof/heap

# Goroutine 分析
go tool pprof http://localhost:8080{BASE}/pprof/goroutine

# 执行追踪（5 秒）
wget -O trace.out "http://localhost:8080{BASE}/pprof/trace?seconds=5"
go tool trace trace.out
```

生产环境需携带认证：

```bash
go tool pprof -http=:8081 http://admin:your-secret@localhost:8080{BASE}/pprof/heap
```

## 运行时指标

```bash
curl http://localhost:8080{BASE}/metrics
```

响应示例：

```json
{
  "timestamp": "2026-06-22T12:00:00Z",
  "go_version": "go1.25.0",
  "num_cpu": 8,
  "num_goroutine": 12,
  "memory": {
    "alloc_bytes": 1234567,
    "heap_alloc_bytes": 1234567
  },
  "gc": {
    "num_gc": 5,
    "gc_cpu_fraction": 0.001
  },
  "requests": {
    "uptime": "1h30m0s",
    "total_requests": 1024,
    "in_flight": 3,
    "slow_requests": 2,
    "avg_latency_ms": 12.5
  }
}
```

代码中也可直接获取：

```go
data, err := ginshow.MetricsJSON()
```

## API 参考

| 函数 | 说明 |
|------|------|
| `Mount(r, cfg)` | 注册 debug 端点并启用中间件 |
| `Attach(group, cfg)` | 仅在路由组上注册 debug 端点 |
| `DefaultBasePath` | 默认路由前缀常量（`/__gs/x7f3a2c9`） |
| `Default()` | 返回开发环境默认配置 |
| `Production(user, pass)` | 返回带 Basic Auth 的生产配置 |
| `Middleware(cfg)` | 单独使用请求监控中间件 |
| `MetricsJSON()` | 获取当前运行时指标 JSON |

## 开发

```bash
# 运行测试
go test ./...

# 运行示例
go run ./example/
```

## License

MIT
