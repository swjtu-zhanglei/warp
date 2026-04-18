# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

Warp 是一个高性能 S3 基准测试工具，用于测试对象存储系统。它支持多种基准测试类型（GET、PUT、DELETE、LIST、STAT 等），可以在分布式模式下运行，多个客户端通过服务器协调执行。此外还包含 Iceberg REST catalog 基准测试，用于测试 Apache Iceberg 元数据操作。

构建需要 Go 1.25 或更高版本。

## 构建和测试命令

### 构建
```bash
# 构建二进制文件（需要 Go 1.25+）
go build

# 运行程序
./warp [command] [options]
```

### 测试
```bash
# 运行所有测试（启用竞态检测）
go test -v -race ./...

# 运行特定包的测试
go test -v ./pkg/bench
go test -v ./pkg/aggregate
```

### 代码检查和格式化
```bash
# 运行 golangci-lint（需要先安装）
golangci-lint run --timeout=5m --config ./.golangci.yml

# 运行 go vet
go vet ./...

# 检查代码格式
gofmt -d .

# 格式化代码
gofmt -w .
```

### 代码编辑后的检查流程

- 鼓励安装 gofumpt 和 golangci-lint
- 每次代码编辑后，先格式化再检查：
  - 两个工具都已安装时：在单个 shell 中执行以减少确认次数：
    ```bash
    cd <root-of-code-tree> && gofumpt -extra -w . && golangci-lint run -j16
    ```
  - 格式化（首选）：`gofumpt -extra -w .`
    - 如果 gofumpt 未安装，使用：`gofmt -w .` 并提示安装方法（`go install mvdan.cc/gofumpt@latest` 或 `brew install gofumpt`）
  - 然后运行 linter（尽量使用仓库配置）：
    - 首选 PATH：`golangci-lint run -j16`
    - 备选 GOPATH：`$(go env GOPATH)/bin/golangci-lint run -j16`
    - 如果缺失或使用旧版 Go 构建，提示升级/安装步骤，并在本地进行文件级别的 lint 检查
  - 修复后重新运行直到通过或遇到阻塞问题

## 架构

### 核心组件

包之间采用分层依赖结构：

**pkg/generator/** - 测试数据生成（基础包，无 warp 依赖）
- 随机数据生成用于基准测试对象
- 支持固定大小、随机大小和分桶大小

**pkg/bench/** - 基准测试实现（导入 pkg/generator）
- `benchmark.go` - 核心 `Benchmark` 接口，包含 `Prepare()`、`Start()`、`Cleanup()` 方法
- `Common` 结构体包含共享配置（bucket、concurrency、clients 等）
- 每种操作类型实现 Benchmark 接口（get.go、put.go、mixed.go 等）
- `ops.go` - 可复用的操作函数（上传、下载、删除操作）
- `collector.go` - 实时操作统计收集

**pkg/aggregate/** - 数据聚合和分析（导入 pkg/bench）
- `aggregate.go` - 将原始操作数据聚合为统计信息
- `throughput.go` - 吞吐量计算和统计
- `requests.go` - 每请求统计（延迟、TTFB、百分位数）
- `compare.go` - 基准测试运行结果对比
- `live.go` - 基准测试运行期间的实时统计更新

**pkg/iceberg/** - Iceberg REST catalog 工具（用于 Iceberg 基准测试）
- `catalog.go` - Catalog 连接创建（支持 AIStor Tables、Polaris）
- `tree.go` - Namespace 树结构管理
- `rest/helpers.go` - REST API 辅助函数
- 支持 MinIO AIStor Tables（SigV4 认证）和 Apache Polaris（OAuth2 认证）

**api/** - HTTP API 用于基准测试状态和控制（导入 pkg/bench 和 pkg/aggregate）
- `api.go` - 提供 HTTP 端点用于监控运行中的基准测试

**cli/** - 命令行接口层（导入 api、pkg/aggregate、pkg/bench、pkg/generator）
- 每种基准测试类型有对应文件（get.go、put.go、delete.go 等）
- `benchmark.go` - 主基准测试执行逻辑（`runBench`、`runServerBenchmark`、`runClientBenchmark`）
- `benchserver.go` / `benchclient.go` - 分布式基准测试协调
- `client.go` - S3 客户端创建和配置
- `flags.go` - 通用 flag 定义
- `analyze.go` - 基准测试后分析
- `ui.go` - 使用 bubbletea 的终端 UI

**wui/** - Web UI 服务器
- 使用 `--web` 参数可启动 Web 界面实时监控基准测试进度

### 关键模式

**基准测试执行流程：**
1. CLI 解析 flags 并创建基准测试实例
2. `Prepare()` - 创建 bucket，上传初始对象（如需要）
3. `Start()` - 运行并发操作直到时间结束或 autoterm 触发
4. 操作记录到 `Collector`，写入压缩 CSV 文件
5. `Cleanup()` - 清理测试数据（除非使用 `--keep-data` 或 `--noclear`）
6. 分析记录数据，输出统计信息

**分布式基准测试：**
- 服务器模式：协调多个客户端，合并结果
- 客户端模式：运行 `warp client [address]` 监听基准测试命令
- 服务器发送基准测试配置到所有客户端
- 客户端同时执行基准测试
- 结果由服务器收集和合并

**操作收集：**
- 每个操作创建 `Operation` 结构体，包含时间、大小、端点、错误信息
- 发送到 `Collector`，批量压缩为 `.csv.zst` 文件
- 格式：Tab 分隔值，字段包括 idx、thread、op、client_id、n_objects、bytes 等

### 重要文件

- `main.go` - 入口点，委托给 `cli.Main()`
- `cli/cli.go` - 命令注册（第 89-105 行列出所有基准测试命令）
- `pkg/bench/benchmark.go` - 核心 Benchmark 接口定义
- `cli/benchmark.go:108` - `runBench()` 是主基准测试运行器

### 基准测试命令列表

基准测试命令：mixed、get、put、delete、list、stat、versioned、retention、multipart、multipart-put、zip、snowball、fanout、append、iceberg

工具命令：analyze、cmp、merge、client、run

## 开发指南

### 添加新的基准测试类型

1. 在 `pkg/bench/` 创建新文件实现 `Benchmark` 接口
2. 在 `cli/` 添加对应的命令文件
3. 在 `cli/cli.go` init 函数中注册命令
4. 参考现有模式（如 `get.go`、`put.go`）

### 测试 S3 兼容性

Warp 设计用于测试任何 S3 兼容存储。连接配置：
- Flags: `--host`、`--access-key`、`--secret-key`、`--tls`、`--region`
- 环境变量: `WARP_HOST`、`WARP_ACCESS_KEY`、`WARP_SECRET_KEY`、`WARP_TLS`、`WARP_REGION`

### YAML 配置

基准测试可通过 `yml-samples/` 中的 YAML 文件配置。运行方式：
```bash
warp run <file.yml>
```

可注入变量：`warp run file.yml -var VarName=Value`

### Iceberg 基准测试

支持 Apache Iceberg REST catalog 操作基准测试：
- `warp iceberg catalog-read` - Catalog 读取操作
- `warp iceberg catalog-commits` - 表/视图属性更新提交
- `warp iceberg catalog-mixed` - 混合读写工作负载
- `warp iceberg sustained` - 持续工作负载（带 RPS 控制）

详见 `README_ICEBERG.md`

### 输出数据格式

基准测试数据保存为 `warp-operation-yyyy-mm-dd[hhmmss]-xxxx.csv.zst`：
- Zstandard 压缩的 CSV
- 可通过 `warp analyze <file>` 分析
- 可通过 `warp cmp <before> <after>` 对比
- 可通过 `warp merge <file1> <file2>...` 合并多个客户端结果

### 并发模型

- `--concurrent N` 设置并行操作线程数
- 每个线程通常有自己的前缀以避免冲突
- 操作使用 context cancellation 实现优雅关闭
- 客户端连接通过 `c.Client()` 函数池化

### 自动终止

启用 `--autoterm` 时：
- 持续采样吞吐量到 25 个时间块
- 检查最后 7 个块是否在 `--autoterm.pct` 阈值内（默认 7.5%）
- 必须在 `--autoterm.dur` 时间内保持稳定（默认 15s）
- 防止在预热或不稳定期间过早终止

### Web UI 监控

使用 `--web` 参数可启动 Web 界面实时监控基准测试进度，浏览器会自动打开。

## 常见问题

### 32 位架构
注意 64 位原子操作 - 使用 `atomic.AddUint64` 时需确保正确对齐（参见 commit 042a9fc）

### TLS 和 Kernel TLS
项目支持 HTTP/2 和 Kernel TLS (kTLS) 以提升 Linux 性能。参见 `cli/client_ktls.go` 和 `cli/client_transport.go`

### InfluxDB 集成
实时指标可推送到 InfluxDB v2+，使用 `--influxdb` flag。连接字符串格式：`<schema>://<token>@<hostname>:<port>/<bucket>/<org>?<tag=value>`

### 多 NIC 基准测试
客户端机器有多个 NIC 连接不同存储网络子网时，每个 NIC 运行一个 `warp client` 进程，指定监听 IP：
```bash
warp client 192.168.11.2:7761
warp client 192.168.12.2:7761
```
Warp 自动将所有 S3 连接源绑定到监听 IP，确保流量只通过指定 NIC。
