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
	"github.com/gin-gonic/gin"
	"github.com/showx/ginshow"
)

func main() {
	r := gin.Default()

	// 一行接入
	ginshow.Mount(r, ginshow.Default())

	r.GET("/api/hello", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "hello"})
	})

	r.Run(":8080")
}
```

启动后可用以下端点：

| 端点 | 说明 |
|------|------|
| `GET /debug/pprof/` | pprof 索引页 |
| `GET /debug/pprof/profile` | CPU 采样 |
| `GET /debug/pprof/heap` | 堆内存 |
| `GET /debug/pprof/goroutine` | Goroutine |
| `GET /debug/pprof/trace` | 执行追踪 |
| `GET /debug/metrics` | 运行时 JSON 指标 |

## 功能

- **pprof 集成** — 标准 `net/http/pprof` 端点，支持 CPU、heap、goroutine、mutex、block、trace
- **运行时指标** — 内存、GC、goroutine 数量及请求统计，JSON 格式便于对接监控
- **请求监控** — 自动统计 QPS、平均延迟、慢请求；debug 路由不计入统计
- **生产环境保护** — 内置 Basic Auth，一行切换生产配置

## 配置

### 默认配置（开发环境）

```go
ginshow.Mount(r, ginshow.Default())
```

默认行为：

- 开启 pprof（前缀 `/debug/pprof`）
- 开启 metrics（路径 `/debug/metrics`）
- 开启请求监控中间件
- 慢请求阈值 500ms

### 生产环境

```go
ginshow.Mount(r, ginshow.Production("admin", "your-secret"))
```

所有 debug 端点将启用 HTTP Basic Auth 保护。

### 自定义配置

```go
cfg := ginshow.Default()
cfg.PprofPrefix = "/internal/pprof"
cfg.MetricsPath = "/internal/metrics"
cfg.SlowRequestThreshold = 200 * time.Millisecond
cfg.BlockProfileRate = 1           // 开启 block profiling
cfg.MutexProfileFraction = 1       // 开启 mutex profiling
ginshow.Mount(r, cfg)
```

### 挂载到路由组

若希望 debug 端点挂在特定路由组下：

```go
admin := r.Group("/admin")
ginshow.Attach(admin, ginshow.Production("admin", "secret"))
// 实际路径: /admin/debug/pprof/*, /admin/debug/metrics
```

## pprof 使用

```bash
# CPU 采样 30 秒
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30

# 堆内存分析
go tool pprof http://localhost:8080/debug/pprof/heap

# Goroutine 分析
go tool pprof http://localhost:8080/debug/pprof/goroutine

# 执行追踪（5 秒）
wget -O trace.out "http://localhost:8080/debug/pprof/trace?seconds=5"
go tool trace trace.out
```

生产环境需携带认证：

```bash
go tool pprof -http=:8081 http://admin:your-secret@localhost:8080/debug/pprof/heap
```

## 运行时指标

```bash
curl http://localhost:8080/debug/metrics
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
