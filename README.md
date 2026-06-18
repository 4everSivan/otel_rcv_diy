# otel_rcv_diy

从零开始学习如何构建 OpenTelemetry Collector Receiver 的实战项目。

## 项目概述

本项目通过动手实践，深入理解 OpenTelemetry Collector 的 Receiver 开发流程。包含一个完整的自定义 Collector 发行版和两个自研 Receiver 组件：

- **`otelcol-dev`** — 使用 OpenTelemetry Collector Builder (OCB) 构建的自定义 Collector 发行版，集成了 OTLP Receiver、Debug/OTLP Exporter、Batch Processor 以及两个自研 Receiver。
- **`tailtracer`** — 基于 OTel 官方教程搭建的 **Trace Receiver**，定时生成模拟 trace 数据（ATM 系统场景），用于学习 Receiver 骨架与 `ptrace` 数据模型。
- **`patronireceiver`** — **Scraper 型 Metrics Receiver**，定时拉取 [Patroni](https://github.com/patroni/patroni) 集群 REST API，将 PostgreSQL 高可用集群的节点/集群状态转换为 OpenTelemetry Metrics。

## 目录结构

```
otel_rcv_diy/
├── otelcol-dev/                  # 自定义 Collector 发行版（OCB 生成）
│   ├── components.go             # 组件注册（tailtracer 已在此注册）
│   ├── main.go / main_others.go / main_windows.go
│   └── go.mod
├── tailtracer/                   # Trace Receiver（官方教程示例）
│   ├── config.go                 # 配置结构体（interval / number_of_traces）
│   ├── factory.go                # receiver.Factory 实现
│   ├── trace-receiver.go         # receiver.Traces 实现（ticker → ConsumeTraces）
│   └── go.mod
├── patronireceiver/              # Metrics Receiver（Scraper 型）
│   ├── config.go                 # Config（scraperhelper + confighttp + MetricsBuilder）
│   ├── factory.go                # receiver.Factory + scraperhelper controller
│   ├── scraper.go                # scraper.Metrics 实现（HTTP 抓取 + MetricsBuilder）
│   ├── model.go                  # Patroni JSON 模型 + 数据转换
│   ├── metadata.yaml             # 指标/属性定义（mdatagen 驱动）
│   └── go.mod
├── docs/                         # 文档
│   ├── collector_execution_flow.md  # Collector 架构与生命周期
│   ├── receiver_execution_flow.md   # Receiver 设计模式与执行流程
│   └── devel/                       # 开发教程
│       ├── build_custom_collector.md  # 使用 OCB 构建自定义 Collector
│       ├── build_receiver.md          # 从零搭建 Trace Receiver（tailtracer）
│       └── build_patronireceiver.md   # 构建 Patroni Metrics Receiver
├── config.yaml                   # Collector 运行配置
├── build_config.yaml             # OCB 构建配置（生成 otelcol-dev）
├── go.work                       # Go Workspace（管理多模块）
└── ocb                           # OCB 二进制
```

## 技术栈

| 类别 | 技术/版本 |
|------|----------|
| Go | 1.25.0 |
| OpenTelemetry Collector | v0.154.0 / v1.60.0 |
| Collector Builder (OCB) | v0.154.0 |
| 模块管理 | Go Workspace |
| 代码生成 | mdatagen（metadata.yaml → generated_*.go） |

## 快速开始

### 前置条件

- Go 1.25+
- Docker（可选，用于运行 Jaeger 查看 trace）

### 构建

```bash
# 1. 使用 OCB 生成 otelcol-dev（首次或修改 build_config.yaml 后）
./ocb --config build_config.yaml

# 2. 生成 patronireceiver 的 metadata 代码
go run go.opentelemetry.io/collector/cmd/mdatagen@latest ./patronireceiver/metadata.yaml

# 3. 确保所有 module 在 workspace 中
go work sync
```

### 运行

```bash
# 启动自定义 Collector
go run ./otelcol-dev --config config.yaml
```

### 查看 Trace（使用 Jaeger）

```bash
# 启动 Jaeger
docker run -d --name jaeger \
  -e COLLECTOR_OTLP_ENABLED=true \
  -p 16686:16686 \
  -p 14317:4317 \
  -p 14318:4318 \
  jaegertracing/all-in-one:1.41

# 访问 Jaeger UI：http://localhost:16686
```

## 两个 Receiver 对比

| 维度 | `tailtracer` | `patronireceiver` |
|------|-------------|-------------------|
| 信号类型 | Traces（`receiver.WithTraces`） | **Metrics**（`receiver.WithMetrics`） |
| 数据来源 | 内存随机生成 | **HTTP GET Patroni REST API** |
| 数据模型 | `ptrace.Traces` | **`pmetric.Metrics`** |
| 触发机制 | 手写 `time.Ticker` | **scraperhelper 控制器** |
| 指标定义 | 无（手写 span） | **`metadata.yaml` + mdatagen 代码生成** |
| Config | `interval` + `number_of_traces` | `endpoint` / `collection_interval` / TLS |
| 稳定性 | Alpha（教学用途） | Development（开发中） |

## 学习路径

建议按以下顺序阅读文档：

1. **[Collector 架构与生命周期](docs/collector_execution_flow.md)** — 了解 Collector 启动流程、组件生命周期与管道机制
2. **[Receiver 设计模式](docs/receiver_execution_flow.md)** — 掌握 Receiver 接口、Scraper 模式、数据流向
3. **[构建自定义 Collector](docs/devel/build_custom_collector.md)** — 使用 OCB 构建你自己的 Collector 发行版
4. **[从零搭建 Trace Receiver](docs/devel/build_receiver.md)** — 跟随教程实现 `tailtracer`，掌握 Receiver 骨架
5. **[构建 Patroni Metrics Receiver](docs/devel/build_patronireceiver.md)** — 进阶实战：真实 HTTP 抓取 + mdatagen + scraperhelper

## 参考资源

- [OpenTelemetry Collector 官方文档](https://opentelemetry.io/docs/collector/)
- [OpenTelemetry Collector Builder](https://github.com/open-telemetry/opentelemetry-collector/tree/main/cmd/builder)
- [OCB 配置参考](https://github.com/open-telemetry/opentelemetry-collector/tree/main/cmd/builder#configuration)
- [Patroni REST API](https://github.com/patroni/patroni/blob/master/docs/rest_api.rst)

## License

MIT
