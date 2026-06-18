# otel_rcv_diy 项目事实

本文件是 `constitution.md` 的项目实施层，维护项目事实：目录结构、Go 模块、构建命令、组件模式和配置约定。

边界（本文件不重复，只引用）:
- 红线 / 工作模式 / 组件开发原则 → `constitution.md`
- 开发教程与设计文档 → `docs/`
- 输出格式 / 模板 → 不适用（本项目无模板层）

---

## 1. 项目目标

面向 OTel Collector Receiver 开发学习场景:

1. 理解 OpenTelemetry Collector 架构与组件生命周期。
2. 从零搭建 Trace Receiver（`tailtracer`），掌握 Factory / Config / Consumer 骨架。
3. 构建生产级 Scraper 型 Metrics Receiver（`patronireceiver`），掌握 mdatagen + scraperhelper 技术栈。
4. 沉淀 OTel Collector 开发方法论，形成可复用的文档与代码模板。

---

## 2. 沟通与输出规范

- **[强制]** 面向用户的说明、文档、注释统一中文；中文内容默认英文半角标点。
- **[例外]** 第三方工具输出、错误信息、Go 标识符、OTel API 名称和 YAML 配置保留原始英文。
- **[强制]** 先给结论再给依据；代码修改说明变更意图和影响范围。

---

## 3. 目录与模块

### 3.1 顶层结构

```
otel_rcv_diy/
├── otelcol-dev/          # Go Module: 自定义 Collector 发行版（OCB 生成）
├── tailtracer/           # Go Module: Trace Receiver（教学示例）
├── patronireceiver/      # Go Module: Metrics Receiver（Scraper 型）
├── docs/                 # 架构文档 + 开发教程
├── config.yaml           # Collector 运行配置
├── build_config.yaml     # OCB 构建清单
├── go.work               # Go Workspace（管理三个 module）
├── go.work.sum           # Workspace 依赖锁定
├── ocb                   # OCB 二进制
├── constitution.md       # 项目治理宪法
├── AGENTS.md             # 本文件 — 项目事实
├── CLAUDE.md             # Claude Code 专属指令
├── README.md             # 项目说明
├── LICENSE               # MIT
└── .gitignore
```

### 3.2 Go Module 表

| Module | 路径 | Go 版本 | 用途 |
|--------|------|---------|------|
| `otelcol-dev` | `./otelcol-dev` | 1.25.0 | 自定义 Collector 发行版，注册所有组件 |
| `tailtracer` | `./tailtracer` | 1.25.0 | Trace Receiver，定时生成模拟 trace |
| `patronireceiver` | `./patronireceiver` | 1.25.0 | Metrics Receiver，抓取 Patroni REST API |

依赖关系: `otelcol-dev` → `tailtracer`（通过 `go.work` 本地解析）。`patronireceiver` 已实现但尚未注册到 `otelcol-dev/components.go`。

### 3.3 文档结构

```
docs/
├── collector_execution_flow.md   # Collector 整体架构与生命周期
├── receiver_execution_flow.md    # Receiver 设计模式与执行流程
└── devel/
    ├── build_custom_collector.md   # 使用 OCB 构建自定义 Collector
    ├── build_receiver.md           # 从零搭建 Trace Receiver（tailtracer 教程）
    └── build_patronireceiver.md    # 构建 Patroni Metrics Receiver（进阶实战）
```

---

## 4. 标准命令

### 4.1 构建与运行

```bash
# 生成 patronireceiver metadata 代码（修改 metadata.yaml 后必须执行）
go run go.opentelemetry.io/collector/cmd/mdatagen@latest ./patronireceiver/metadata.yaml

# 同步 workspace
go work sync

# 构建 otelcol-dev（使用 OCB 重新生成源码+二进制）
./ocb --config build_config.yaml

# 运行 Collector
go run ./otelcol-dev --config config.yaml

# 编译检查所有 module
go build ./...
go vet ./...
```

### 4.2 测试

```bash
# 运行所有测试
go test ./...

# 运行单个 module 测试
go test ./tailtracer/...
go test ./patronireceiver/...

# patronireceiver 测试（依赖 mdatagen 生成代码 + httptest mock）
go test -v ./patronireceiver/...
```

### 4.3 依赖管理

```bash
# 更新单个 module 依赖
cd otelcol-dev && go mod tidy
cd tailtracer && go mod tidy
cd patronireceiver && go mod tidy

# 新增 module 到 workspace
go work use ./<new-module>
```

---

## 5. 组件开发模式

### 5.1 Receiver 文件骨架

所有 Receiver 组件遵循统一文件结构:

```
<receiver>/
├── config.go       # Config 结构体 + Validate()
├── factory.go      # NewFactory() + createDefaultConfig() + createXxxReceiver()
├── <receiver>.go   # receiver 实例 + Start() / Shutdown()
├── go.mod
└── go.sum
```

如果是 Metrics Receiver（scraper 型），额外包含:

```
├── metadata.yaml                  # 指标/属性定义（mdatagen 输入）
├── model.go                       # 数据模型 + 解析逻辑
├── scraper.go                     # scraper.Metrics 实现（Start / Shutdown / Scrape）
├── doc.go                         # //go:generate 指令
└── internal/metadata/             # mdatagen 生成代码（不手动编辑）
    ├── generated_config.go
    ├── generated_metrics.go
    └── generated_status.go
```

### 5.2 组件注册流程

1. 在 `otelcol-dev/components.go` 中 import 模块。
2. 在 `factories.Receivers`（或 Exporters / Processors）中调用 `NewFactory()`。
3. 在 `config.yaml` 的 `service.pipelines` 中引用组件 ID。

> 只有在 `service.pipelines` 中被引用的 receiver 才会被 Collector 实例化并 `Start()`。

### 5.3 tailtracer 关键事实

- 类型: Trace Receiver（`receiver.WithTraces`）
- 触发: 手写 `time.Ticker`，按 `interval` 定时生成
- 数据: 模拟 ATM → BackendSystem 的 trace，含父子 span 关系
- 稳定性: `component.StabilityLevelAlpha`
- 模块路径 (import): `github.com/open-telemetry/opentelemetry-tutorials/trace-receiver/tailtracer`
- 已在 `otelcol-dev/components.go` 注册

### 5.4 patronireceiver 关键事实

- 类型: Scraper 型 Metrics Receiver（`receiver.WithMetrics`）
- 触发: `scraperhelper.NewMetricsController` 自动管理 ticker / context / 错误
- 数据: HTTP GET `/patroni`、`/cluster`、`/history` → `pmetric.Metrics`
- 指标定义: `metadata.yaml` → mdatagen → `internal/metadata/generated_*.go`
- 稳定性: `component.StabilityLevelDevelopment`
- 尚未注册到 `otelcol-dev/components.go`（待端到端验证）
- Patroni REST API 参考: <https://github.com/patroni/patroni/blob/master/docs/rest_api.rst>

### 5.5 关键 OTel 模块版本

| 包 | 模块路径 | 版本 |
|---|---|---|
| `receiver` | `go.opentelemetry.io/collector/receiver` | v1.60.0 |
| `scraper` | `go.opentelemetry.io/collector/scraper` | v0.154.0 |
| `scraperhelper` | `go.opentelemetry.io/collector/scraper/scraperhelper` | v0.154.0 |
| `confighttp` | `go.opentelemetry.io/collector/config/confighttp` | v0.154.0 |
| `pdata/pmetric` | `go.opentelemetry.io/collector/pdata/pmetric` | v1.60.0 |
| `pdata/ptrace` | `go.opentelemetry.io/collector/pdata/ptrace` | v1.60.0 |
| `consumer` | `go.opentelemetry.io/collector/consumer` | v1.60.0 |

---

## 6. 配置约定

- `config.yaml`: Collector 运行时配置，包含 receivers / exporters / processors / pipelines。
- `build_config.yaml`: OCB 构建清单，定义发行版名称、输出路径和组件列表。
- 本地私有配置（如生产 endpoint）通过 `.gitignore` 排除，不在仓库中出现。

---

## 7. 规则层级与单一事实源

优先级 (高→低): 工具系统指令 > `constitution.md` > 本文件 `AGENTS.md` > `CLAUDE.md` > skills > `docs/devel/` 设计说明 > 单次普通偏好。

| 概念 | 唯一归属 |
|------|---------|
| 红线 / 工作模式 / 开发原则 | `constitution.md` |
| 项目事实 / 路径 / 模块 / 命令 | `AGENTS.md` (本文件) |
| 组件开发模式与流程 | `AGENTS.md` §5 |
| 架构解析 / 设计文档 / 开发教程 | `docs/` |
| 工具专属使用方式 | `CLAUDE.md` |
