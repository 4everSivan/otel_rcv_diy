# 构建一个 Receiver

LLMS 索引: [llms.txt](/llms.txt)

---

<!-- markdownlint-disable heading-increment no-duplicate-heading -->

OpenTelemetry 将 [分布式追踪 (distributed tracing)](/docs/concepts/glossary/#distributed-tracing) 定义为：

> 跟踪单个请求（称为 trace）在组成应用程序的服务中流转的过程。该请求可以由用户或应用程序发起。分布式追踪是一种穿越进程、网络和安全边界的追踪形式。

虽然分布式追踪是以应用为中心的方式定义的，但您可以将其视为在您的系统中流动的 *任何* 请求的时间线。每个分布式追踪展示了一个请求从开始到结束花费了多长时间，并分解了完成该请求所采取的步骤。

如果您的系统生成追踪遥测数据，您可以为您的 [OpenTelemetry Collector](https://opentelemetry.io/zh/docs/collector/extend/ocb/) 配置一个专门的 trace receiver（追踪接收器），用于接收并转换该遥测数据。接收器将您的数据从其原始格式转换为 OpenTelemetry trace 模型，以便 Collector 可以对其进行处理。

要实现一个 trace receiver，您需要以下内容：

- 一个 `Config` 实现，以便 trace receiver 可以收集并校验它在 Collector `config.yaml` 中的配置。

- 一个 `receiver.Factory` 实现，以便 Collector 可以正确实例化该 trace receiver 组件。

- 一个 `receiver.Traces` 实现，用于收集遥测数据，将其转换为内部 trace 表示，并将遥测数据传递给管道中的下一个 consumer（消费者）。

本教程将向您展示如何创建一个名为 `tailtracer` 的 trace receiver，它模拟拉取（pull）操作并生成追踪作为该操作的结果。

## 设置接收器开发和测试环境

首先，使用 [构建自定义 Collector](https://opentelemetry.io/zh/docs/collector/extend/ocb/) 教程来创建一个名为 `otelcol-dev` 的 Collector 实例；您只需要复制 [配置 OpenTelemetry Collector Builder](https://github.com/open-telemetry/opentelemetry-collector/tree/main/cmd/builder) 中描述的 `builder-config.yaml` 并运行 builder。运行后，您应该会得到如下所示的文件夹结构：

```text
.
├── builder-config.yaml
├── ocb
└── otelcol-dev
    ├── components.go
    ├── components_test.go
    ├── go.mod
    ├── go.sum
    ├── main.go
    ├── main_others.go
    ├── main_windows.go
    └── otelcol-dev
```

为了正确测试您的 trace receiver，您可能需要一个分布式追踪后端，以便 Collector 可以将遥测数据发送给它。我们将使用 [Jaeger](https://www.jaegertracing.io/docs/latest/getting-started/)。如果您没有运行 `Jaeger` 实例，可以使用 Docker 运行以下命令轻松启动一个：

```sh
docker run -d --name jaeger \
  -e COLLECTOR_OTLP_ENABLED=true \
  -p 16686:16686 \
  -p 14317:4317 \
  -p 14318:4318 \
  jaegertracing/all-in-one:1.41
```

容器启动并运行后，您可以通过此 URL 访问 Jaeger UI：
<http://localhost:16686/>

现在，创建一个名为 `config.yaml` 的 Collector 配置文件来设置 Collector 的组件和管道。

```sh
touch config.yaml
```

目前，您只需要一个包含 `otlp` 接收器以及 `otlp` 和 `debug` 导出器的基础 traces 管道。以下是您的 `config.yaml` 文件的样子：

> config.yaml

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

exporters:
  debug:
    verbosity: detailed
  otlp/jaeger:
    endpoint: localhost:14317
    tls:
      insecure: true
    sending_queue:
      batch:

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlp/jaeger, debug]
  telemetry:
    logs:
      level: debug
```

> [!NOTE]
>
> 在这里，为了简单起见，我们在 `otlp` 导出器配置中使用了 `insecure` 标志；在生产环境中运行 Collector 时，您应该遵循此 [指南](/docs/collector/configuration/#setting-up-certificates) 使用 TLS 证书进行安全通信，或使用 mTLS 进行双向认证。

要验证 Collector 是否正确设置，运行此命令：

```sh
./otelcol-dev/otelcol-dev --config config.yaml
```

输出可能如下所示：

```log
2023-11-08T18:38:37.183+0800	info	service@v0.88.0/telemetry.go:84	Setting up own telemetry...
2023-11-08T18:38:37.185+0800	info	service@v0.88.0/telemetry.go:201	Serving Prometheus metrics	{"address": ":8888", "level": "Basic"}
2023-11-08T18:38:37.185+0800	debug	exporter@v0.88.0/exporter.go:273	Stable component.	{"kind": "exporter", "data_type": "traces", "name": "otlp/jaeger"}
2023-11-08T18:38:37.186+0800	info	exporter@v0.88.0/exporter.go:275	Development component. May change in the future.	{"kind": "exporter", "data_type": "traces", "name": "debug"}
2023-11-08T18:38:37.186+0800	debug	receiver@v0.88.0/receiver.go:294	Stable component.	{"kind": "receiver", "name": "otlp", "data_type": "traces"}
2023-11-08T18:38:37.186+0800	info	service@v0.88.0/service.go:143	Starting otelcol-dev...	{"Version": "1.0.0", "NumCPU": 10}

<OMITTED>

2023-11-08T18:38:37.189+0800	info	service@v0.88.0/service.go:169	Everything is ready. Begin running and processing data.
2023-11-08T18:38:37.189+0800	info	zapgrpc/zapgrpc.go:178	[core] [Server #3 ListenSocket #4] ListenSocket created	{"grpc_log": true}
2023-11-08T18:38:37.195+0800	info	zapgrpc/zapgrpc.go:178	[core] [Channel #1 SubChannel #2] Subchannel Connectivity change to READY	{"grpc_log": true}
2023-11-08T18:38:37.195+0800	info	zapgrpc/zapgrpc.go:178	[core] [pick-first-lb 0x140005efdd0] Received SubConn state update: 0x140005eff80, {ConnectivityState:READY ConnectionError:<nil>}	{"grpc_log": true}
2023-11-08T18:38:37.195+0800	info	zapgrpc/zapgrpc.go:178	[core] [Channel #1] Channel Connectivity change to READY	{"grpc_log": true}
```

如果一切顺利，Collector 实例应该已经启动并运行。

您可以使用 [telemetrygen](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/cmd/telemetrygen) 来进一步验证设置。例如，打开另一个终端并运行以下命令：

```sh
go install github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen@latest

telemetrygen traces --otlp-insecure --traces 1
```

您应该能够在终端中看到详细的日志，并通过此 URL 在 Jaeger UI 中看到 traces：<http://localhost:16686/>。

在 Collector 终端中按 <kbd>Ctrl + C</kbd> 停止 Collector 实例。

## 设置 Go 模块

每个 Collector 组件都应该作为一个 Go 模块（Go module）创建。让我们创建一个 `tailtracer` 文件夹来存放我们的接收器项目，并将其初始化为 Go 模块。

```sh
mkdir tailtracer
cd tailtracer
go mod init github.com/open-telemetry/opentelemetry-tutorials/trace-receiver/tailtracer
```

> [!NOTE]
>
> 上面的模块路径是一个模拟路径，可以是您想要的任何私有或公有路径。参见 [初始 trace-receiver 代码](https://github.com/rquedas/otel4devs/tree/main/collector/receiver/trace-receiver)。

建议启用 Go [工作区 (Workspaces)](https://go.dev/doc/tutorial/workspaces)，因为我们将管理多个 Go 模块：`otelcol-dev` 和 `tailtracer`，并且随着时间的推移可能会有更多组件。

```sh
cd ..
go work init
go work use otelcol-dev
go work use tailtracer
```

## 设计并校验接收器设置

接收器可能有一些可配置的设置，可以通过 Collector 配置文件进行设置。

`tailtracer` 接收器将具有以下设置：

- `interval`：代表遥测数据拉取操作之间的时间间隔的字符串（以分钟为单位）。
- `number_of_traces`：每个间隔生成的模拟 trace 的数量。

以下是 `tailtracer` 接收器设置的样子：

```yaml
receivers:
  tailtracer: # 此行代表您接收器的 ID
    interval: 1m
    number_of_traces: 1
```

在 `tailtracer` 文件夹下创建一个名为 `config.go` 的文件，您将在其中编写所有代码以支持您的接收器配置设置。

```sh
touch tailtracer/config.go
```

要实现接收器的配置方面，您需要创建一个 `Config` 结构体。将以下代码添加到您的 `config.go` 文件中：

```go
package tailtracer

type Config struct{

}
```

为了让您的接收器能够访问其设置，`Config` 结构体必须为接收器的每个设置提供一个字段。

以下是实现上述要求后的 `config.go` 文件的样子：

> tailtracer/config.go

```go
package tailtracer

// Config 代表 Collector config.yaml 中接收器的配置设置
type Config struct {
   Interval    string `mapstructure:"interval"`
   NumberOfTraces int `mapstructure:"number_of_traces"`
}
```

> [!NOTE] 检查您的工作
>
> - 添加了 `Interval` 和 `NumberOfTraces` 字段，以便能够从 config.yaml 正确访问它们的值。

现在您已经可以访问设置，您可以通过实现可选的 [ConfigValidator](https://github.com/open-telemetry/opentelemetry-collector/blob/677b87e3ab5c615bc3f93b8f99bb1fa5be951751/component/config.go#L28) 接口来提供这些值所需的任何校验。

在这种情况下，`interval` 值将是可选的（我们稍后会看如何生成默认值）。但是一旦定义，它应该至少为 1 分钟 (1m)，并且 `number_of_traces` 将是一个必填值。以下是实现 `Validate` 方法后的 `config.go` 文件的样子：

> tailtracer/config.go

```go
package tailtracer

import (
	"fmt"
	"time"
)

// Config 代表 Collector config.yaml 中接收器的配置设置
type Config struct {
	Interval       string `mapstructure:"interval"`
	NumberOfTraces int    `mapstructure:"number_of_traces"`
}

// Validate 检查接收器配置是否有效
func (cfg *Config) Validate() error {
	interval, _ := time.ParseDuration(cfg.Interval)
	if interval.Minutes() < 1 {
		return fmt.Errorf("when defined, the interval has to be set to at least 1 minute (1m)")
	}

	if cfg.NumberOfTraces < 1 {
		return fmt.Errorf("number_of_traces must be greater or equal to 1")
	}
	return nil
}
```

> [!NOTE] 检查您的工作
>
> - 导入了 `fmt` 包以正确格式化输出错误消息。
> - 向 Config 结构体添加了 `Validate` 方法，以检查 `interval` 设置值是否至少为 1 分钟 (1m)，以及 `number_of_traces` 设置值是否大于或等于 1。如果不满足，Collector 将在启动过程中生成一个错误并相应地显示消息。

如果您想更深入地查看涉及组件配置方面的结构体和接口，请参阅 Collector GitHub 项目中的 [component/config.go](<https://github.com/open-telemetry/opentelemetry-collector/blob/v0.153.0/component/config.go>) 文件。

## 实现 receiver.Factory 接口

`tailtracer` 接收器必须提供一个 `receiver.Factory` 实现。虽然 `receiver.Factory` 接口是在 Collector 项目的 [receiver/receiver.go](<https://github.com/open-telemetry/opentelemetry-collector/blob/v0.153.0/receiver/receiver.go#L58>) 文件中定义的，但实现它的正确方法是使用 `go.opentelemetry.io/collector/receiver` 包中提供的函数。

创建一个名为 `factory.go` 的文件：

```sh
touch tailtracer/factory.go
```

现在，让我们按照惯例添加一个名为 `NewFactory()` 的函数，该函数将负责实例化 `tailtracer` 工厂。继续并将以下代码添加到您的 `factory.go` 文件中：

```go
package tailtracer

import (
	"go.opentelemetry.io/collector/receiver"
)

// NewFactory 创建一个 tailtracer 接收器的工厂。
func NewFactory() receiver.Factory {
	return nil
}
```

要实例化您的 `tailtracer` 接收器工厂，您将使用来自 `receiver` 包的以下函数：

```go
func NewFactory(cfgType component.Type, createDefaultConfig component.CreateDefaultConfigFunc, options ...FactoryOption) Factory
```

`receiver.NewFactory()` 实例化并返回一个 `receiver.Factory`，它需要以下参数：

- `component.Type`：您的接收器在所有 Collector 组件中的唯一字符串标识符。

- `component.CreateDefaultConfigFunc`：返回您的接收器的 `component.Config` 实例的函数的引用。

- `...FactoryOption`：`receiver.FactoryOption` 切片，它将决定您的接收器能够处理什么类型的信号。

现在让我们实现代码以支持 `receiver.NewFactory()` 所需的所有参数。

## 识别并提供默认设置

之前我们提到过 `tailtracer` 接收器的 `interval` 设置是可选的。您需要为其提供一个默认值，以便它可以作为默认设置的一部分被使用。

继续并将以下代码添加到您的 `factory.go` 文件中：

```go
var (
	typeStr         = component.MustNewType("tailtracer")
)

const (
	defaultInterval = 1 * time.Minute
)
```

至于默认设置，您只需要添加一个返回持有 `tailtracer` 接收器默认配置的 `component.Config` 的函数。

为此，继续并将以下代码添加到您的 `factory.go` 文件中：

```go
func createDefaultConfig() component.Config {
	return &Config{
		Interval: string(defaultInterval),
	}
}
```

在这两个更改之后，您会发现缺少了一些导入，所以以下是带有正确导入的 `factory.go` 文件的样子：

> tailtracer/factory.go

```go
package tailtracer

import (
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
)

var (
	typeStr         = component.MustNewType("tailtracer")
)

const (
	defaultInterval = 1 * time.Minute
)

func createDefaultConfig() component.Config {
	return &Config{
		Interval: string(defaultInterval),
	}
}

// NewFactory 创建一个 tailtracer 接收器的工厂。
func NewFactory() receiver.Factory {
	return nil
}
```

> [!NOTE] 检查您的工作
>
> - 导入了 `time` 包以支持 `defaultInterval` 的 `time.Duration` 类型。
> - 导入了 `go.opentelemetry.io/collector/component` 包，这是声明 `component.Config` 的地方。
> - 导入了 `go.opentelemetry.io/collector/receiver` 包，这是声明 `receiver.Factory` 的地方。
> - 添加了一个名为 `defaultInterval` 的 `time.Duration` 常量，代表我们接收器的 `Interval` 设置的默认值。我们将默认值设置为 1 分钟，因此赋值为 `1 * time.Minute`。
> - 添加了一个名为 `createDefaultConfig` 的函数，该函数负责返回一个 `component.Config` 实现，在这种情况下将是我们的 `tailtracer.Config` 结构体的一个实例。
> - `tailtracer.Config.Interval` 字段被初始化为 `defaultInterval` 常量。

## 指定接收器的能力

一个接收器组件可以处理 trace、metrics 和 logs。接收器的工厂负责指定接收器将提供的能力。

鉴于追踪是本教程的主题，我们将启用 `tailtracer` 接收器使其仅支持 trace。`receiver` 包提供了以下函数和类型来帮助工厂描述 trace 处理能力：

```go
func WithTraces(createTracesReceiver CreateTracesFunc, sl component.StabilityLevel) FactoryOption
```

`receiver.WithTraces()` 实例化并返回一个 `receiver.FactoryOption`，它需要以下参数：

- `createTracesReceiver`：符合 `receiver.CreateTracesFunc` 类型的函数的引用。`receiver.CreateTracesFunc` 类型是指向一个负责实例化并返回 `receiver.Traces` 实例的函数的指针，它需要以下参数：
    - `context.Context`：Collector `context.Context` 的引用，以便您的 trace 接收器能够正确管理其执行上下文。
    - `receiver.Settings`：创建您的接收器所依据的一些 Collector 设置的引用。
    - `component.Config`：Collector 传递给工厂的接收器配置设置的引用，以便它可以从 Collector 配置中正确读取其设置。
    - `consumer.Traces`：管道中下一个 `consumer.Traces` 的引用，这是接收到的 traces 的流向。这通常是一个 processor 或一个 exporter。

首先添加引导代码以正确实现 `receiver.CreateTracesFunc` 函数指针。继续并将以下代码添加到您的 `factory.go` 文件中：

```go
func createTracesReceiver(_ context.Context, params receiver.Settings, baseCfg component.Config, consumer consumer.Traces) (receiver.Traces, error) {
	return nil, nil
}
```

您现在拥有了使用 `receiver.NewFactory` 函数成功实例化您的接收器工厂所需的所有组件。继续并在 `factory.go` 文件中更新您的 `NewFactory()` 函数，如下所示：

```go
// NewFactory 创建一个 tailtracer 接收器的工厂。
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithTraces(createTracesReceiver, component.StabilityLevelAlpha))
}
```

在这些更改之后，您会发现缺少了一些导入，所以以下是带有正确导入的 `factory.go` 文件的样子：

> tailtracer/factory.go

```go
package tailtracer

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

var (
	typeStr         = component.MustNewType("tailtracer")
)

const (
	defaultInterval = 1 * time.Minute
)

func createDefaultConfig() component.Config {
	return &Config{
		Interval: string(defaultInterval),
	}
}

func createTracesReceiver(_ context.Context, params receiver.Settings, baseCfg component.Config, consumer consumer.Traces) (receiver.Traces, error) {
	return nil, nil
}

// NewFactory 创建一个 tailtracer 接收器的工厂。
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithTraces(createTracesReceiver, component.StabilityLevelAlpha))
}
```

> [!NOTE] 检查您的工作
>
> - 导入了 `context` 包以支持 `createTracesReceiver` 函数中引用的 `context.Context` 类型。
> - 导入了 `go.opentelemetry.io/collector/consumer` 包以支持 `createTracesReceiver` 函数中引用的 `consumer.Traces` 类型。
> - 更新了 `NewFactory()` 函数，使其返回通过调用带有必需参数的 `receiver.NewFactory()` 生成的 `receiver.Factory`。通过调用 `receiver.WithTraces(createTracesReceiver, component.StabilityLevelAlpha)`，生成的接收器工厂将能够处理 trace。

## 实现接收器组件

所有接收器 API 当前都在 Collector 项目的 [receiver/receiver.go](<https://github.com/open-telemetry/opentelemetry-collector/blob/v0.153.0/receiver/receiver.go>) 文件中声明。打开该文件并花一分钟浏览所有接口。

请注意，此时 `receiver.Traces`（及其同级 `receiver.Metrics` 和 `receiver.Logs`）除了它从 `component.Component` “继承”的方法之外，并没有描述任何特定的方法。

这可能会让人觉得奇怪，但请记住，Collector API 的设计初衷是为了具备可扩展性。组件及其信号可能会以不同的方式演进，因此这些接口的作用就是为了帮助支持这一点。

要创建一个 `receiver.Traces`，您需要实现由 `component.Component` 接口描述的以下方法：

```go
Start(ctx context.Context, host Host) error
Shutdown(ctx context.Context) error
```

这两个方法都作为事件处理程序，由 Collector 在其组件的生命周期中与其进行通信。

`Start()` 方法代表 Collector 通知该组件开始其处理的信号。作为事件的一部分，Collector 将传递以下信息：

- `context.Context`：大多数情况下，接收器将处理一个长期运行的操作，因此建议忽略此上下文，而是从 `context.Background()` 创建一个新的上下文。
- `Host`：Host 旨在让接收器在启动并运行后能够与 Collector 主机进行通信。

`Shutdown()` 方法代表 Collector 通知该组件服务正在关闭的信号，因此该组件应该停止其处理并进行所有所需的必要清理工作：

- `context.Context`：Collector 作为关闭操作的一部分传递的上下文。

您将通过在 `tailtracer` 文件夹中创建一个名为 `trace-receiver.go` 的新文件来开始实现：

```sh
touch tailtracer/trace-receiver.go
```

然后向名为 `tailtracerReceiver` 的类型添加声明，如下所示：

```go
type tailtracerReceiver struct{

}
```

现在您拥有了 `tailtracerReceiver` 类型，您可以实现 `Start()` 和 `Shutdown()` 方法，以便该接收器类型符合 `receiver.Traces` 接口。

> tailtracer/trace-receiver.go

```go
package tailtracer

import (
	"context"
	"go.opentelemetry.io/collector/component"
)

type tailtracerReceiver struct {
}

func (tailtracerRcvr *tailtracerReceiver) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (tailtracerRcvr *tailtracerReceiver) Shutdown(ctx context.Context) error {
	return nil
}
```

> [!NOTE] 检查您的工作
>
> - 导入了 `context` 包，这是声明 `Context` 类型和函数的地方。
> - 导入了 `go.opentelemetry.io/collector/component` 包，这是声明 `Host` 类型的地方。
> - 实现了 `Start(ctx context.Context, host component.Host)` 方法的骨架，以符合 `receiver.Traces` 接口。
> - 实现了 `Shutdown(ctx context.Context)` 方法的骨架，以符合 `receiver.Traces` 接口。
`Start()` 方法传递了 2 个引用（`context.Context` 和 `component.Host`），您的接收器可能需要保留它们，以便在它的处理操作中使用它们。

`context.Context` 引用应该用于创建一个新的上下文以支持接收器的处理操作。您需要决定处理上下文取消的最佳方式，以便作为 `Shutdown()` 方法中组件关闭的一部分正确终止它。

`component.Host` 在接收器的整个生命周期中都很有用，因此请将该引用保留在 `tailtracerReceiver` 类型中。

在包含保留上述建议引用的字段后，`tailtracerReceiver` 类型声明如下所示：

```go
type tailtracerReceiver struct {
	host   component.Host
	cancel context.CancelFunc
}
```

现在您需要更新 `Start()` 方法，以便接收器能够正确初始化自己的处理上下文，将取消函数保存在 `cancel` 字段中，并初始化其 `host` 字段值。您还将更新 `Shutdown()` 方法，通过调用 `cancel` 函数来终止上下文。

以下是做出这些更改后的 `trace-receiver.go` 文件的样子：

> tailtracer/trace-receiver.go

```go
package tailtracer

import (
	"context"
	"go.opentelemetry.io/collector/component"
)

type tailtracerReceiver struct {
	host   component.Host
	cancel context.CancelFunc
}

func (tailtracerRcvr *tailtracerReceiver) Start(ctx context.Context, host component.Host) error {
	tailtracerRcvr.host = host
	ctx = context.Background()
	ctx, tailtracerRcvr.cancel = context.WithCancel(ctx)

	return nil
}

func (tailtracerRcvr *tailtracerReceiver) Shutdown(ctx context.Context) error {
	if tailtracerRcvr.cancel != nil {
		tailtracerRcvr.cancel()
	}
	return nil
}
```

> [!NOTE] 检查您的工作
>
> - 更新了 `Start()` 方法，添加了对 `host` 字段的初始化，使用 Collector 传递的 `component.Host` 引用。
> - 将 `cancel` 函数字段设置为基于使用 `context.Background()` 创建的新上下文的取消函数（根据 Collector API 文档的建议）。
> - 更新了 `Shutdown()` 方法，添加了对 `cancel()` 上下文取消函数的调用。

## 保留接收器工厂传递的信息

现在您已经实现了 `receiver.Traces` 接口方法，您的 `tailtracer` 接收器组件已准备好被其工厂实例化和返回。

打开 `tailtracer/factory.go` 文件并导航到 `createTracesReceiver()` 函数。请注意，工厂将在 `createTracesReceiver()` 函数参数中传递您的接收器正常工作所需的引用。这些包括其配置设置 (`component.Config`)、管道中用于消费所生成 traces 的下一个 `Consumer` (`consumer.Traces`)，以及 Collector 记录器，以便 `tailtracer` 接收器向其添加有意义的事件 (`receiver.Settings`)。

鉴于所有这些信息仅在接收器被工厂实例化时提供，`tailtracerReceiver` 类型需要相应的字段来保存这些信息，并在其生命周期的其他阶段中使用。

以下是更新了 `tailtracerReceiver` 类型声明后的 `trace-receiver.go` 文件的样子：

> tailtracer/trace-receiver.go

```go
package tailtracer

import (
	"context"
	"time"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

type tailtracerReceiver struct {
	host         component.Host
	cancel       context.CancelFunc
	logger       *zap.Logger
	nextConsumer consumer.Traces
	config       *Config
}

func (tailtracerRcvr *tailtracerReceiver) Start(ctx context.Context, host component.Host) error {
	tailtracerRcvr.host = host
	ctx = context.Background()
	ctx, tailtracerRcvr.cancel = context.WithCancel(ctx)

	interval, _ := time.ParseDuration(tailtracerRcvr.config.Interval)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
				case <-ticker.C:
					tailtracerRcvr.logger.Info("I should start processing traces now!")
				case <-ctx.Done():
					return
			}
		}
	}()

	return nil
}

func (tailtracerRcvr *tailtracerReceiver) Shutdown(ctx context.Context) error {
	if tailtracerRcvr.cancel != nil {
		tailtracerRcvr.cancel()
	}
	return nil
}
```

> [!NOTE] 检查您的工作
>
> - 导入了 `go.opentelemetry.io/collector/consumer`，这是管道消费者类型和接口声明的地方。
> - 导入了 `go.uber.org/zap` 包，这是 Collector 用于其调试功能的包。
> - 添加了一个名为 `logger` 的 `zap.Logger` 字段，以便我们可以在接收器内部访问 Collector 记录器引用。
> - 添加了一个名为 `nextConsumer` 的 `consumer.Traces` 字段，以便我们可以将 `tailtracer` 接收器生成的 traces 推送到管道中声明的下一个消费者。
> - 添加了一个名为 `config` 的 `Config` 字段，以便我们可以访问在 Collector 配置中定义的接收器配置设置。
> - 添加了一个名为 `interval` 的变量，它根据 Collector 配置中 `tailtracer` 接收器的 `interval` 设置的值初始化为 `time.Duration`。
> - 添加了一个 `go func()` 来实现 `ticker` 机制，以便接收器可以在 `ticker` 达到 `interval` 变量指定的时间量时生成 traces。
> - 使用 `tailtracerRcvr.logger` 字段在每次应该生成 traces 时生成一条信息消息。

`tailtracerReceiver` 类型已准备好被实例化，并将保留其工厂传递的所有有意义的信息。

打开 `tailtracer/factory.go` 文件并导航到 `createTracesReceiver()` 函数。

接收器只有在管道中被声明为组件时才会被实例化，工厂负责确保管道中的下一个消费者（无论是处理器还是导出器）有效。否则，它应该生成一个错误。

`createTracesReceiver()` 函数需要一个保护子句来进行该验证。

您还需要变量来正确初始化 `tailtracerReceiver` 实例的 `config` 和 `logger` 字段。

以下是更新了 `createTracesReceiver()` 函数后的 `factory.go` 文件：

> tailtracer/factory.go

```go
package tailtracer

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

var (
	typeStr         = component.MustNewType("tailtracer")
)

const (
	defaultInterval = 1 * time.Minute
)

func createDefaultConfig() component.Config {
	return &Config{
		Interval: string(defaultInterval),
	}
}

func createTracesReceiver(_ context.Context, params receiver.Settings, baseCfg component.Config, consumer consumer.Traces) (receiver.Traces, error) {

	logger := params.Logger
	tailtracerCfg := baseCfg.(*Config)

	traceRcvr := &tailtracerReceiver{
		logger:       logger,
		nextConsumer: consumer,
		config:       tailtracerCfg,
	}

	return traceRcvr, nil
}

// NewFactory 创建一个 tailtracer 接收器的工厂。
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithTraces(createTracesReceiver, component.StabilityLevelAlpha))
}
```

> [!NOTE] 检查您的工作
>
> - 添加了一个名为 `logger` 的变量，并使用 `receiver.Settings` 引用中名为 `Logger` 的字段中可用的 Collector 记录器对其进行初始化。
> - 添加了一个名为 `tailtracerCfg` 的变量，并通过将 `component.Config` 引用强制转换为 `tailtracer` 接收器的 `Config` 来对其进行初始化。
> - 添加了一个名为 `traceRcvr` 的变量，并使用存储在变量中的工厂信息通过 `tailtracerReceiver` 实例对其进行初始化。
> - 更新了返回语句以包含 `traceRcvr` 实例。

至此，接收器的骨架已经完全实现。

## 用接收器更新 Collector 的初始化流程

为了让接收器参与到 Collector 的管道中，我们需要对生成的 `otelcol-dev/components.go` 文件进行一些更新，该文件是注册和实例化所有 Collector 组件的地方。

必须将 `tailtracer` 接收器工厂实例添加到 `factories` map 中，以便 Collector 在其初始化过程中能够正确加载它。

以下是进行更改以支持该功能后的 `components.go` 文件的样子：

> otelcol-dev/components.go

```go
// Code generated by "go.opentelemetry.io/collector/cmd/builder". DO NOT EDIT.

package main

import (
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/receiver"
	debugexporter "go.opentelemetry.io/collector/exporter/debugexporter"
	otlpexporter "go.opentelemetry.io/collector/exporter/otlpexporter"
	otlpreceiver "go.opentelemetry.io/collector/receiver/otlpreceiver"
	tailtracer "github.com/open-telemetry/opentelemetry-tutorials/trace-receiver/tailtracer" // newly added line
)

func components() (otelcol.Factories, error) {
	var err error
	factories := otelcol.Factories{}

	factories.Extensions, err = otelcol.MakeFactoryMap[extension.Factory](
	)
	if err != nil {
		return otelcol.Factories{}, err
	}

	factories.Receivers, err = otelcol.MakeFactoryMap[receiver.Factory](
		otlpreceiver.NewFactory(),
		tailtracer.NewFactory(), // newly added line
	)
	if err != nil {
		return otelcol.Factories{}, err
	}

	factories.Exporters, err = otelcol.MakeFactoryMap[exporter.Factory](
		debugexporter.NewFactory(),
		otlpexporter.NewFactory(),
	)
	if err != nil {
		return otelcol.Factories{}, err
	}

	factories.Processors, err = otelcol.MakeFactoryMap[processor.Factory](
	)
	if err != nil {
		return otelcol.Factories{}, err
	}

	return factories, nil
}
```

> [!NOTE] 检查您的工作
>
> - 导入了接收器模块 `github.com/open-telemetry/opentelemetry-tutorials/trace-receiver/tailtracer`，这是接收器类型和函数所在的地方。
> - 添加了对 `tailtracer.NewFactory()` 的调用作为 `otelcol.MakeFactoryMap()` 调用的参数，以便正确地将您的 `tailtracer` 接收器工厂添加到 `factories` map 中。

## 运行和调试接收器

确保已正确更新 Collector `config.yaml`，并将 `tailtracer` 接收器配置为管道中使用的接收器之一。

> config.yaml

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
  tailtracer: # this line represents the ID of your receiver
    interval: 1m
    number_of_traces: 1

exporters:
  debug:
    verbosity: detailed
  otlp/jaeger:
    endpoint: localhost:14317
    tls:
      insecure: true
    sending_queue:
      batch:

service:
  pipelines:
    traces:
      receivers: [otlp, tailtracer]
      exporters: [otlp/jaeger, debug]
  telemetry:
    logs:
      level: debug
```

由于我们在 `otelcol-dev/components.go` 文件中进行了代码修改，让我们使用 `go run` 命令代替先前生成的 `./otelcol-dev/otelcol-dev` 二进制文件来启动更新后的 Collector。

```sh
go run ./otelcol-dev --config config.yaml
```

输出应该像这样：

```log
2023-11-08T21:38:36.621+0800	info	service@v0.88.0/telemetry.go:84	Setting up own telemetry...
2023-11-08T21:38:36.621+0800	info	service@v0.88.0/telemetry.go:201	Serving Prometheus metrics	{"address": ":8888", "level": "Basic"}
2023-11-08T21:38:36.621+0800	info	exporter@v0.88.0/exporter.go:275	Development component. May change in the future.	{"kind": "exporter", "data_type": "traces", "name": "debug"}
2023-11-08T21:38:36.621+0800	debug	exporter@v0.88.0/exporter.go:273	Stable component.	{"kind": "exporter", "data_type": "traces", "name": "otlp/jaeger"}
2023-11-08T21:38:36.621+0800	debug	receiver@v0.88.0/receiver.go:294	Stable component.	{"kind": "receiver", "name": "otlp", "data_type": "traces"}
2023-11-08T21:38:36.621+0800	debug	receiver@v0.88.0/receiver.go:294	Alpha component. May change in the future.	{"kind": "receiver", "name": "tailtracer", "data_type": "traces"}
2023-11-08T21:38:36.622+0800	info	service@v0.88.0/service.go:143	Starting otelcol-dev...	{"Version": "1.0.0", "NumCPU": 10}
2023-11-08T21:38:36.622+0800	info	extensions/extensions.go:33	Starting extensions...

<OMITTED>

2023-11-08T21:38:36.636+0800	info	zapgrpc/zapgrpc.go:178	[core] [Channel #1] Channel Connectivity change to READY	{"grpc_log": true}
2023-11-08T21:39:36.626+0800	info	tailtracer/trace-receiver.go:33	I should start processing traces now!	{"kind": "receiver", "name": "tailtracer", "data_type": "traces"}
2023-11-08T21:40:36.626+0800	info	tailtracer/trace-receiver.go:33	I should start processing traces now!	{"kind": "receiver", "name": "tailtracer", "data_type": "traces"}
...
```

如您从日志中看到的，`tailtracer` 已经成功初始化。每分钟都会有一条由 `tailtracer/trace-receiver.go` 中的虚拟定时器触发的消息，内容为：`I should start processing traces now!`。

> [!TIP]
>
> 要停止该进程，请在 Collector 终端中按 <kbd>Ctrl + C</kbd>。

此外，您可以使用您喜欢的 IDE 来调试接收器，就像您平时调试 Go 项目一样。以下是一个简单的 [Visual Studio Code](https://code.visualstudio.com/) `launch.json` 文件供您参考：

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Launch otelcol-dev",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}/otelcol-dev",
      "args": ["--config", "${workspaceFolder}/config.yaml"]
    }
  ]
}
```

作为一个巨大的里程碑，让我们看看现在的文件夹结构：

```console
.
├── builder-config.yaml
├── config.yaml
├── go.work
├── go.work.sum
├── ocb
├── otelcol-dev
│   ├── components.go
│   ├── components_test.go
│   ├── go.mod
│   ├── go.sum
│   ├── main.go
│   ├── main_others.go
│   ├── main_windows.go
│   └── otelcol-dev
└── tailtracer
    ├── config.go
    ├── factory.go
    ├── go.mod
    └── trace-receiver.go
```

在下一节中，您将了解更多关于 OpenTelemetry Trace 数据模型的信息，以便 `tailtracer` 接收器最终可以生成有意义的 trace！

## Collector Trace 数据模型

您可能通过使用 SDK 并对应用程序进行埋点，以在像 Jaeger 这样的分布式追踪后端中观察和评估您的 trace，从而对 OpenTelemetry trace 有所了解。

以下是 trace 在 Jaeger 中的样子：

![Jaeger trace](https://opentelemetry.io/img/docs/tutorials/Jaeger.jpeg)

虽然这是一个 Jaeger trace，但它是通过 Collector 中的 trace 管道生成的。这可以帮助您了解有关 OTel trace 数据模型的一些事情：

- 一个 trace 由一个或多个呈层次结构组织的 span 组成，用以表示依赖关系。
- 这些 span 可以代表服务内部和/或跨服务的操作。

在 trace 接收器中创建 trace 将与您使用 SDK 创建 trace 的方式略有不同，因此让我们首先回顾一下高层概念。

### 使用 Resources

在 OTel 的世界中，所有遥测数据都是由 `Resource` 生成的。以下是根据 [OTel 规范](/docs/specs/otel/resource/sdk)的定义：

> `Resource` 是产生遥测数据的实体的不可变表示（以属性 Attributes 的形式）。例如，在 Kubernetes 上的容器中运行的产生遥测数据的进程具有 Pod 名称，运行在命名空间中，并且可能是具有其自身名称的 Deployment 的一部分。这三个属性都可以包含在 `Resource` 中。

Trace 最常用于表示服务请求（Jaeger 模型中描述 the Services 实体），通常实现为在计算单元中运行的进程。然而，OTel 通过属性描述 `Resource` 的 API 方法非常灵活，足以代表您可能需要的任何实体，例如 ATM、物联网传感器等。

因此可以肯定地说，对于要存在的 trace，必须先由 `Resource` 开始它。

在本教程中，我们将模拟一个带有遥测数据的系统，该系统展示了位于 2 个不同州（例如，伊利诺伊州和加利福尼亚州）的 ATM 访问账户后端系统以执行余额、存款和取款操作。为了实现这一目标，我们将编写代码来创建代表 ATM 和后端系统的 `Resource` 类型。

继续在 `tailtracer` 文件夹中创建一个名为 `model.go` 的文件。

```sh
touch tailtracer/model.go
```

现在，在 `model.go` 文件中，添加 `Atm` 和 `BackendSystem` 类型的定义，如下所示：

> tailtracer/model.go

```go
package tailtracer

type Atm struct{
	ID           int64
	Version      string
	Name         string
	StateID      string
	SerialNumber string
	ISPNetwork   string
}

type BackendSystem struct{
	Version       string
	ProcessName   string
	OSType        string
	OSVersion     string
	CloudProvider string
	CloudRegion   string
	Endpoint      string
}
```

这些类型旨在代表被观察系统中所展现的实体。它们包含的信息对于作为 `Resource` 定义的一部分添加到 trace 中非常有意义。您将添加一些辅助函数来生成这些类型的实例。

以下是添加了辅助函数后 `model.go` 文件的样子：

> tailtracer/model.go

```go
package tailtracer

import (
	"math/rand"
)

type Atm struct{
	ID           int64
	Version      string
	Name         string
	StateID      string
	SerialNumber string
	ISPNetwork   string
}

type BackendSystem struct{
	Version       string
	ProcessName   string
	OSType        string
	OSVersion     string
	CloudProvider string
	CloudRegion   string
	Endpoint      string
}

func generateAtm() Atm{
	i := getRandomNumber(1, 2)
	var newAtm Atm

	switch i {
		case 1:
			newAtm = Atm{
				ID: 111,
				Name: "ATM-111-IL",
				SerialNumber: "atmxph-2022-111",
				Version: "v1.0",
				ISPNetwork: "comcast-chicago",
				StateID: "IL",

			}

		case 2:
			newAtm = Atm{
				ID: 222,
				Name: "ATM-222-CA",
				SerialNumber: "atmxph-2022-222",
				Version: "v1.0",
				ISPNetwork: "comcast-sanfrancisco",
				StateID: "CA",
			}
	}

	return newAtm
}

func generateBackendSystem() BackendSystem{
	i := getRandomNumber(1, 3)

	newBackend := BackendSystem{
		ProcessName: "accounts",
		Version: "v2.5",
		OSType: "lnx",
		OSVersion: "4.16.10-300.fc28.x86_64",
		CloudProvider: "amzn",
		CloudRegion: "us-east-2",
	}

	switch i {
		case 1:
		 	newBackend.Endpoint = "api/v2.5/balance"
		case 2:
		  	newBackend.Endpoint = "api/v2.5/deposit"
		case 3:
			newBackend.Endpoint = "api/v2.5/withdrawn"

	}

	return newBackend
}

func getRandomNumber(min int, max int) int {
	i := (rand.Intn(max - min + 1) + min)
	return i
}
```

> [!NOTE] 检查您的工作
>
> - 导入了 `math/rand` 包以支持 `generateRandomNumber` 函数的实现。
> - 添加了 `generateAtm` 函数，该函数实例化一个 `Atm` 类型，并随机分配伊利诺伊州或加利福尼亚州作为 `StateID` 的值，以及相应的 `ISPNetwork` 值。
> - 添加了 `generateBackendSystem` 函数，该函数创建一个 `BackendSystem` 类型的实例，并随机分配服务端点值给 `Endpoint` 字段。
> - 添加了 `generateRandomNumber` 函数，以在指定范围内生成随机数。

现在您已经拥有了生成代表产生遥测数据的实体的对象实例的函数，您可以准备在 OTel Collector 的世界中代表这些实体了。

Collector API 提供了一个名为 `ptrace` 的包，嵌套在 `pdata` 包下。它包含了在 Collector 管道组件中处理 trace 所需的所有类型、接口和辅助函数。

打开 `tailtracer/model.go` 文件并将 `go.opentelemetry.io/collector/pdata/ptrace` 添加到 `import` 子句中，以便您可以访问 `ptrace` 包的能力。

在定义 `Resource` 之前，您需要创建一个负责在 Collector 管道中传输 trace 的 `ptrace.Traces`。您可以使用辅助函数 `ptrace.NewTraces()` 来实例化它。您还需要创建 `Atm` 和 `BackendSystem` 类型的实例，以便拥有代表参与到 trace 中的遥测源的数据。

打开 `tailtracer/model.go` 文件并向其中添加以下函数：

```go
func generateTraces(numberOfTraces int) ptrace.Traces{
	traces := ptrace.NewTraces()

	for i := 0; i < numberOfTraces; i++{
		newAtm := generateAtm()
		newBackendSystem := generateBackendSystem()
	}

	return traces
}
```

到目前为止，您已经听过和读过很多关于 trace 是如何由 span 组成的内容。您甚至可能已经使用 SDK 中可用的函数和类型编写了一些埋点代码来创建它们。然而，您可能不知道的是，在 Collector API 中创建 trace 时，还涉及其他类型的“span”。

您将从一个名为 `ptrace.ResourceSpans` 的类型开始，该类型代表资源以及它在参与 trace 时发起或接收的所有操作。您可以在 [/pdata/ptrace/generated_resourcespans.go](<https://github.com/open-telemetry/opentelemetry-collector/blob/v0.153.0/pdata/ptrace/generated_resourcespans.go>) 文件中找到其定义。

`ptrace.Traces` 具有一个名为 `ResourceSpans()` 的方法，该方法返回一个名为 `ptrace.ResourceSpansSlice` 的辅助类型的实例。`ptrace.ResourceSpansSlice` 类型具有一些方法来帮助您处理 `ptrace.ResourceSpans` 数组。该数组包含的项目数与参与 trace 所代表的请求的 `Resource` 实体数相同。

`ptrace.ResourceSpansSlice` 具有一个名为 `AppendEmpty()` 的方法，该方法向数组添加一个新的 `ptrace.ResourceSpan` 并返回其引用。

一旦您获得 `ptrace.ResourceSpan` 的实例，您将使用一个名为 `Resource()` 的方法，该方法将返回与该 `ResourceSpan` 关联的 `pcommon.Resource` 实例。

更新 `generateTrace()` 函数并做出以下更改：

- 添加一个名为 `resourceSpan` 的变量来表示 `ResourceSpan`。
- 添加一个名为 `atmResource` 的变量来表示与 `ResourceSpan` 关联的 `pcommon.Resource`。
- 使用上面提到的方法分别初始化这两个变量。

以下是实现更改后该函数样子：

```go
func generateTraces(numberOfTraces int) ptrace.Traces{
	traces := ptrace.NewTraces()

	for i := 0; i < numberOfTraces; i++{
		newAtm := generateAtm()
		newBackendSystem := generateBackendSystem()

		resourceSpan := traces.ResourceSpans().AppendEmpty()
		atmResource := resourceSpan.Resource()
	}

	return traces
}
```

> [!NOTE] 检查您的工作
>
> - 添加了 `resourceSpan` 变量，并使用 `traces.ResourceSpans().AppendEmpty()` 调用返回的 `ResourceSpan` 引用对其进行初始化。
> - 添加了 `atmResource` 变量，并使用 `resourceSpan.Resource()` 调用返回的 `pcommon.Resource` 引用对其进行初始化。

### 通过属性描述 Resources

Collector API 提供了一个名为 `pcommon` 的包，嵌套在 `pdata` 包下。它包含了描述 `Resource` 所需的所有类型和辅助函数。

在 Collector 的上下文中，`Resource` 由键/值对格式的属性描述，由 `pcommon.Map` 类型表示。

您可以参考 Collector GitHub 项目中 [/pdata/pcommon/map.go](<https://github.com/open-telemetry/opentelemetry-collector/blob/v0.153.0/pdata/pcommon/map.go>) 文件中 `pcommon.Map` 类型的定义及其用于使用支持的格式创建属性值的相关辅助函数。

键/值对提供了极大的灵活性来帮助建模您的 `Resource` data。OTel 规范制定了一些指南，以帮助组织并减少不同类型的遥测数据产生实体之间可能需要表示的冲突。

这些指南被称为 [资源语义约定 (Resource Semantic Conventions)](/docs/specs/semconv/resource/)，并记录在 OTel 规范中。

在创建代表您自己遥测产生实体的属性时，您应该遵循规范提供的指南：

> 属性按其描述的概念类型进行逻辑分组。同一组中的属性具有以点结尾的共同前缀。例如，描述 Kubernetes 属性的所有属性都以 `k8s.` 开头。

首先打开 `tailtracer/model.go` 文件并将 `go.opentelemetry.io/collector/pdata/pcommon` 添加到 `import` 子句中，以便您可以访问 `pcommon` 包的能力。

现在，继续添加一个函数，从 `Atm` 实例中读取字段值，并将它们作为属性（以前缀 "atm." 分组）写入 `pcommon.Resource` 实例中。以下是该函数的样子：

```go
func fillResourceWithAtm(resource *pcommon.Resource, atm Atm){
   atmAttrs := resource.Attributes()
   atmAttrs.PutInt("atm.id", atm.ID)
   atmAttrs.PutStr("atm.stateid", atm.StateID)
   atmAttrs.PutStr("atm.ispnetwork", atm.ISPNetwork)
   atmAttrs.PutStr("atm.serialnumber", atm.SerialNumber)
}
```

> [!NOTE] 检查您的工作
>
> - 声明了一个名为 `atmAttrs` 的变量，并使用 `resource.Attributes()` 调用返回的 `pcommon.Map` 引用对其进行初始化。
> - 使用 `pcommon.Map` 的 `PutInt()` 和 `PutStr()` 方法，根据等效的 `Atm` 字段类型添加整型和字符串型属性。请注意，因为这些属性是特定的且仅代表 `Atm` 实体，它们都分在前缀 `atm.` 下。

资源语义约定还指定了规定性的属性名称和常用值，以表示在不同领域（如 [计算单元 (compute unit)](/docs/specs/semconv/resource/#compute-unit)、[环境 (environment)](/docs/specs/semconv/resource/#compute-unit) 等）中常见且适用的遥测数据产生实体。

对于 `BackendSystem` 实体，它具有代表与 [操作系统 (Operating System)](/docs/specs/semconv/resource/os/) 和 [云 (Cloud)](/docs/specs/semconv/resource/cloud/) 相关信息的字段。我们将使用资源语义约定指定的属性名称和值来在它的 `Resource` 上代表该信息。

资源语义约定的键和常见值由 OpenTelemetry 语义约定包定义：[`go.opentelemetry.io/otel/semconv/v1.38.0`](https://pkg.go.dev/go.opentelemetry.io/otel/semconv/v1.38.0)。

让我们创建一个函数，从 `BackendSystem` 实例中读取字段值并将它们作为属性写入 `pcommon.Resource` 实例中。打开 `tailtracer/model.go` 文件并添加以下函数：

```go
func fillResourceWithBackendSystem(resource *pcommon.Resource, backend BackendSystem){
	backendAttrs := resource.Attributes()
	var osType, cloudProvider string

	switch {
		case backend.CloudProvider == "amzn":
			cloudProvider = semconv.CloudProviderAWS.Value.AsString()
		case backend.CloudProvider == "mcrsft":
			cloudProvider = semconv.CloudProviderAzure.Value.AsString()
		case backend.CloudProvider == "gogl":
			cloudProvider = semconv.CloudProviderGCP.Value.AsString()
	}

	backendAttrs.PutStr(string(semconv.CloudProviderKey), cloudProvider)
	backendAttrs.PutStr(string(semconv.CloudRegionKey), backend.CloudRegion)

	switch {
		case backend.OSType == "lnx":
			osType = semconv.OSTypeLinux.Value.AsString()
		case backend.OSType == "wndws":
			osType = semconv.OSTypeWindows.Value.AsString()
		case backend.OSType == "slrs":
			osType = semconv.OSTypeSolaris.Value.AsString()
	}

	backendAttrs.PutStr(string(semconv.OSTypeKey), osType)
	backendAttrs.PutStr(string(semconv.OSVersionKey), backend.OSVersion)
 }
```

`pcommon.Resource` to have a required attribute named `service.name` as
prescribed by the resource semantic convention.

We will also use a non-required attribute named `service.version` to represent
the version information for both the `Atm` and `BackendSystem` entities.

Here is what the `tailtracer/model.go` file looks like after adding the code for
properly assigning the "service." group attributes:

> tailtracer/model.go

```go
package tailtracer

import (
	"math/rand"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/semconv/v1.38.0"
)

type Atm struct {
	ID           int64
	Version      string
	Name         string
	StateID      string
	SerialNumber string
	ISPNetwork   string
}

type BackendSystem struct {
	Version       string
	ProcessName   string
	OSType        string
	OSVersion     string
	CloudProvider string
	CloudRegion   string
	Endpoint      string
}

func generateAtm() Atm {
	i := getRandomNumber(1, 2)
	var newAtm Atm

	switch i {
	case 1:
		newAtm = Atm{
			ID:           111,
			Name:         "ATM-111-IL",
			SerialNumber: "atmxph-2022-111",
			Version:      "v1.0",
			ISPNetwork:   "comcast-chicago",
			StateID:      "IL",
		}

	case 2:
		newAtm = Atm{
			ID:           222,
			Name:         "ATM-222-CA",
			SerialNumber: "atmxph-2022-222",
			Version:      "v1.0",
			ISPNetwork:   "comcast-sanfrancisco",
			StateID:      "CA",
		}
	}

	return newAtm
}

func generateBackendSystem() BackendSystem {
	i := getRandomNumber(1, 3)

	newBackend := BackendSystem{
		ProcessName:   "accounts",
		Version:       "v2.5",
		OSType:        "lnx",
		OSVersion:     "4.16.10-300.fc28.x86_64",
		CloudProvider: "amzn",
		CloudRegion:   "us-east-2",
	}

	switch i {
	case 1:
		newBackend.Endpoint = "api/v2.5/balance"
	case 2:
		newBackend.Endpoint = "api/v2.5/deposit"
	case 3:
		newBackend.Endpoint = "api/v2.5/withdrawn"
	}

	return newBackend
}

func getRandomNumber(min int, max int) int {
	i := (rand.Intn(max-min+1) + min)
	return i
}

func generateTraces(numberOfTraces int) ptrace.Traces {
	traces := ptrace.NewTraces()

	for i := 0; i < numberOfTraces; i++ {
		newAtm := generateAtm()
		newBackendSystem := generateBackendSystem()

		resourceSpan := traces.ResourceSpans().AppendEmpty()
		atmResource := resourceSpan.Resource()
		fillResourceWithAtm(&atmResource, newAtm)

		resourceSpan = traces.ResourceSpans().AppendEmpty()
		backendResource := resourceSpan.Resource()
		fillResourceWithBackendSystem(&backendResource, newBackendSystem)
	}

	return traces
}

func fillResourceWithAtm(resource *pcommon.Resource, atm Atm) {
	atmAttrs := resource.Attributes()
	atmAttrs.PutInt("atm.id", atm.ID)
	atmAttrs.PutStr("atm.stateid", atm.StateID)
	atmAttrs.PutStr("atm.ispnetwork", atm.ISPNetwork)
	atmAttrs.PutStr("atm.serialnumber", atm.SerialNumber)
	atmAttrs.PutStr(string(semconv.ServiceNameKey), atm.Name)
	atmAttrs.PutStr(string(semconv.ServiceVersionKey), atm.Version)

}

func fillResourceWithBackendSystem(resource *pcommon.Resource, backend BackendSystem) {
	backendAttrs := resource.Attributes()
	var osType, cloudProvider string

	switch {
	case backend.CloudProvider == "amzn":
		cloudProvider = semconv.CloudProviderAWS.Value.AsString()
	case backend.CloudProvider == "mcrsft":
		cloudProvider = semconv.CloudProviderAzure.Value.AsString()
	case backend.CloudProvider == "gogl":
		cloudProvider = semconv.CloudProviderGCP.Value.AsString()
	}

	backendAttrs.PutStr(string(semconv.CloudProviderKey), cloudProvider)
	backendAttrs.PutStr(string(semconv.CloudRegionKey), backend.CloudRegion)

	switch {
	case backend.OSType == "lnx":
		osType = semconv.OSTypeLinux.Value.AsString()
	case backend.OSType == "wndws":
		osType = semconv.OSTypeWindows.Value.AsString()
	case backend.OSType == "slrs":
		osType = semconv.OSTypeSolaris.Value.AsString()
	}

	backendAttrs.PutStr(string(semconv.OSTypeKey), osType)
	backendAttrs.PutStr(string(semconv.OSVersionKey), backend.OSVersion)

	backendAttrs.PutStr(string(semconv.ServiceNameKey), backend.ProcessName)
	backendAttrs.PutStr(string(semconv.ServiceVersionKey), backend.Version)
}
```

> [!NOTE] Check your work
>
> - Updated the `fillResourceWithAtm()` function by adding lines to properly
    >   assign the "service.name" and "service.version" attributes to the
    >   `pcommon.Resource` that represents the `Atm` entity.
> - Updated the `fillResourceWithBackendSystem()` function by adding lines to
    >   properly assign the "service.name" and "service.version" attributes to the
    >   `pcommon.Resource` that represents the `BackendSystem` entity.
> - Updated the `generateTraces` function by adding lines to properly
    >   instantiate a `pcommon.Resource` and fill in the attribute information for
    >   both `Atm` and `BackendSystem` entities using the `fillResourceWithAtm()`
    >   and `fillResourceWithBackendSystem()` functions.

### 使用 span 表示操作

你现在有了一个 `ResourceSpan` 实例，其中各自的 `Resource` 已经正确填充了属性，以表示 `Atm` 和 `BackendSystem` 实体。你现在可以开始表示每个 `Resource` 作为 `ResourceSpan` 中 trace 的一部分所执行的操作了。

在 OTel 的世界中，系统要生成遥测数据，需要通过手动或使用插桩库（instrumentation library）进行自动插桩。

插桩库负责设置范围（也称为插桩范围，instrumentation scope），参与 trace 的操作在该范围内发生，并在 trace 上下文中将这些操作描述为 span。

`pdata.ResourceSpans` 有一个名为 `ScopeSpans()` 的方法，该方法返回一个名为 `ptrace.ScopeSpansSlice` 的辅助类型的实例。`ptrace.ScopeSpansSlice` 类型包含许多方法，可以帮助你处理 `ptrace.ScopeSpans` 数组。该数组包含的项数将与表示不同插桩范围的 `ptrace.ScopeSpan` 数量以及在 trace 上下文中生成的 span 数量相同。

`ptrace.ScopeSpansSlice` 有一个名为 `AppendEmpty()` 的方法，它会向数组中添加一个新的 `ptrace.ScopeSpans` 并返回其引用。

让我们创建一个函数来实例化一个表示 ATM 系统插桩范围及其 span 的 `ptrace.ScopeSpans`。打开 `tailtracer/model.go` 文件并添加以下函数：

```go
func appendAtmSystemInstrScopeSpans(resourceSpans *ptrace.ResourceSpans) ptrace.ScopeSpans {
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()

	return scopeSpans
}
```

`ptrace.ScopeSpans` 有一个名为 `Scope()` 的方法，该方法返回对 `pcommon.InstrumentationScope` 实例的引用，该实例表示生成 span 的插桩范围（instrumentation scope）。

`pcommon.InstrumentationScope` 具有以下方法来描述插桩范围：

- `SetName(v string)` 设置插桩库的名称。

- `SetVersion(v string)` 设置插桩库的版本。

- `Name() string` 返回与插桩库关联的名称。

- `Version() string` 返回与插桩库关联的版本。

让我们更新 `appendAtmSystemInstrScopeSpans` 函数，以便我们可以为新的 `ptrace.ScopeSpans` 设置插桩范围的名称和版本。更新后的 `appendAtmSystemInstrScopeSpans` 如下所示：

```go
func appendAtmSystemInstrScopeSpans(resourceSpans *ptrace.ResourceSpans) ptrace.ScopeSpans {
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
	scopeSpans.Scope().SetName("atm-system")
	scopeSpans.Scope().SetVersion("v1.0")
	return scopeSpans
}
```

你现在可以更新 `generateTraces` 函数并添加变量来表示 `Atm` 和 `BackendSystem` 实体所使用的插桩范围，通过使用 `appendAtmSystemInstrScopeSpans()` 对它们进行初始化。更新后的 `generateTraces()` 如下所示：

```go
func generateTraces(numberOfTraces int) ptrace.Traces{
	traces := ptrace.NewTraces()

	for i := 0; i < numberOfTraces; i++{
		newAtm := generateAtm()
		newBackendSystem := generateBackendSystem()

		resourceSpan := traces.ResourceSpans().AppendEmpty()
		atmResource := resourceSpan.Resource()
		fillResourceWithAtm(&atmResource, newAtm)

		atmInstScope := appendAtmSystemInstrScopeSpans(&resourceSpan)

		resourceSpan = traces.ResourceSpans().AppendEmpty()
		backendResource := resourceSpan.Resource()
		fillResourceWithBackendSystem(&backendResource, newBackendSystem)

		backendInstScope := appendAtmSystemInstrScopeSpans(&resourceSpan)
	}

	return traces
}
```

至此，你已经拥有了表示系统中遥测数据生成实体所需的一切，以及负责识别操作并为系统生成 trace 的插桩范围。下一步是创建表示该插桩范围作为 trace 的一部分所生成的 span。

`ptrace.ScopeSpans` 有一个名为 `Spans()` 的方法，该方法返回一个名为 `ptrace.SpanSlice` 的辅助类型的实例。`ptrace.SpanSlice` 类型包含许多方法，可以帮助你处理 `ptrace.Span` 数组。该数组包含的项数将与插桩范围在 trace 中能够识别并描述的操作数量相同。

`ptrace.SpanSlice` 有一个名为 `AppendEmpty()` 的方法，它会向数组中添加一个新的 `ptrace.Span` 并返回其引用。

`ptrace.Span` 具有以下方法来描述操作：

- `SetTraceID(v pcommon.TraceID)` 设置唯一标识此 span 关联的 trace 的 `pcommon.TraceID`。

- `SetSpanID(v pcommon.SpanID)` 设置在此 span 关联的 trace 上下文中唯一标识该 span 的 `pcommon.SpanID`。

- `SetParentSpanID(v pcommon.SpanID)` 设置父 span/操作的 `pcommon.SpanID`（如果此 span 表示的操作是作为父操作的一部分执行的，即嵌套）。

- `SetName(v string)` 设置 span 的操作名称。

- `SetKind(v ptrace.SpanKind)` 设置 `ptrace.SpanKind`，用于定义 span 表示的操作类型。

- `SetStartTimestamp(v pcommon.Timestamp)` 设置 `pcommon.Timestamp`，表示与 span 关联的操作开始的日期和时间。

- `SetEndTimestamp(v pcommon.Timestamp)` 设置 `pcommon.Timestamp`，表示与 span 关联的操作结束的日期和时间。

从上面的方法可以看出，一个 `ptrace.Span` 由两个必需的 ID 唯一标识：由 `pcommon.SpanID` 类型表示的它们自己的唯一 ID，以及由 `pcommon.TraceID` 类型表示的它们所关联的 trace ID。

`pcommon.TraceID` 必须携带一个由 16 字节数组表示的全局唯一 ID，并且应遵循 [W3C Trace Context 规范](https://www.w3.org/TR/trace-context/#trace-id)。`pcommon.SpanID` 是在其关联的 trace 上下文中的唯一 ID，由 8 字节数组表示。

`pcommon` 包提供了以下用于生成 span ID 的类型：

- `type TraceID [16]byte`

- `type SpanID [8]byte`

在本教程中，你将使用 `github.com/google/uuid` 包中的函数生成 `pcommon.TraceID`，并使用 `crypto/rand` 包中的函数随机生成 `pcommon.SpanID`。首先，打开 `tailtracer/model.go` 文件并将这两个包添加到 `import` 语句中。之后，添加以下函数以帮助生成这两个 ID：

```go
import (
	crand "crypto/rand"
	"math/rand"
  	...
)

func NewTraceID() pcommon.TraceID {
	return pcommon.TraceID(uuid.New())
}

func NewSpanID() pcommon.SpanID {
	var rngSeed int64
	_ = binary.Read(crand.Reader, binary.LittleEndian, &rngSeed)
	randSource := rand.New(rand.NewSource(rngSeed))

	var sid [8]byte
	randSource.Read(sid[:])
	spanID := pcommon.SpanID(sid)

	return spanID
}
```

> [!NOTE] 检查你的工作
>
> - 将 `crypto/rand` 导入为 `crand`，以避免与 `math/rand` 冲突。
> - 添加了新函数 `NewTraceID()` 和 `NewSpanID()`，分别用于生成 trace ID 和 span ID。

现在你有了正确标识 span 的方法，可以开始创建它们来表示系统内部及跨实体的操作。

作为 `generateBackendSystem()` 函数的一部分，我们随机分配了 `BackEndSystem` 实体可以作为服务提供给系统的操作。接下来，我们将打开 `tailtracer/model.go` 文件并查看名为 `appendTraceSpans()` 的函数，该函数将负责创建一个 trace 并追加表示 `BackendSystem` 操作的 span。以下是 `appendTraceSpans()` 函数的初始实现：

```go
func appendTraceSpans(backend *BackendSystem, backendScopeSpans *ptrace.ScopeSpans, atmScopeSpans *ptrace.ScopeSpans) {
	traceId := NewTraceID()
	backendSpanId := NewSpanID()

	backendDuration, _ := time.ParseDuration("1s")
	backendSpanStartTime := time.Now()
	backendSpanFinishTime := backendSpanStartTime.Add(backendDuration)

	backendSpan := backendScopeSpans.Spans().AppendEmpty()
	backendSpan.SetTraceID(traceId)
	backendSpan.SetSpanID(backendSpanId)
	backendSpan.SetName(backend.Endpoint)
	backendSpan.SetKind(ptrace.SpanKindServer)
	backendSpan.SetStartTimestamp(pcommon.NewTimestampFromTime(backendSpanStartTime))
	backendSpan.SetEndTimestamp(pcommon.NewTimestampFromTime(backendSpanFinishTime))
}
```

> [检查你的工作
>
> - 添加了 `traceId` 和 `backendSpanId` 变量分别表示 trace ID 和 span ID，并使用之前创建的辅助函数对它们进行初始化。
> - 添加了 `backendSpanStartTime` 和 `backendSpanFinishTime` 来表示操作的开始和结束时间。在本教程中，任何 `BackendSystem` 操作都将耗时 1 秒。
> - 添加了一个名为 `backendSpan` 的变量，该变量将保存表示此操作的 `ptrace.Span` 实例。
> - 使用 `BackendSystem` 实例的 `Endpoint` 字段值设置 span 的 `Name`。
> - 将 span 的 `Kind` 设置为 `ptrace.SpanKindServer`。请参阅 trace 规范中的 [SpanKind 部分](/docs/specs/otel/trace/api/#spankind)以了解如何正确定义 SpanKind。
> - 使用上面提到的所有方法为 `ptrace.Span` 填充适当的值以表示 `BackendSystem` 操作。

你可能已经注意到，在 `appendTraceSpans()` 函数的参数中，有 2 个对 `ptrace.ScopeSpans` 的引用，但我们只使用了其中一个。现在不用担心；我们稍后会再讲到它。

接下来，你将更新 `generateTraces()` 函数，以便它可以通过调用 `appendTraceSpans()` 函数来生成 trace。以下是更新后的 `generateTraces()` 函数的外观：

```go
func generateTraces(numberOfTraces int) ptrace.Traces {
	traces := ptrace.NewTraces()

	for i := 0; i < numberOfTraces; i++ {
		newAtm := generateAtm()
		newBackendSystem := generateBackendSystem()

		resourceSpan := traces.ResourceSpans().AppendEmpty()
		atmResource := resourceSpan.Resource()
		fillResourceWithAtm(&atmResource, newAtm)

		atmInstScope := appendAtmSystemInstrScopeSpans(&resourceSpan)

		resourceSpan = traces.ResourceSpans().AppendEmpty()
		backendResource := resourceSpan.Resource()
		fillResourceWithBackendSystem(&backendResource, newBackendSystem)

		backendInstScope := appendAtmSystemInstrScopeSpans(&resourceSpan)

		appendTraceSpans(&newBackendSystem, &backendInstScope, &atmInstScope)
	}

	return traces
}
```

你现在已经将 `BackendSystem` 实体及其操作在适当的 trace 上下文中用 span 表示出来了！接下来，你需要将生成的 trace 推送给 pipeline，以便下一个消费者（无论是 processor 还是 exporter）能够接收并处理它。

以下是 `tailtracer/model.go` 文件的样子：

> tailtracer/model.go

```go
package tailtracer

import (
	crand "crypto/rand"
	"encoding/binary"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/semconv/v1.38.0"
)

type Atm struct {
	ID           int64
	Version      string
	Name         string
	StateID      string
	SerialNumber string
	ISPNetwork   string
}

type BackendSystem struct {
	Version       string
	ProcessName   string
	OSType        string
	OSVersion     string
	CloudProvider string
	CloudRegion   string
	Endpoint      string
}

func generateAtm() Atm {
	i := getRandomNumber(1, 2)
	var newAtm Atm

	switch i {
	case 1:
		newAtm = Atm{
			ID:           111,
			Name:         "ATM-111-IL",
			SerialNumber: "atmxph-2022-111",
			Version:      "v1.0",
			ISPNetwork:   "comcast-chicago",
			StateID:      "IL",
		}

	case 2:
		newAtm = Atm{
			ID:           222,
			Name:         "ATM-222-CA",
			SerialNumber: "atmxph-2022-222",
			Version:      "v1.0",
			ISPNetwork:   "comcast-sanfrancisco",
			StateID:      "CA",
		}
	}

	return newAtm
}

func generateBackendSystem() BackendSystem {
	i := getRandomNumber(1, 3)

	newBackend := BackendSystem{
		ProcessName:   "accounts",
		Version:       "v2.5",
		OSType:        "lnx",
		OSVersion:     "4.16.10-300.fc28.x86_64",
		CloudProvider: "amzn",
		CloudRegion:   "us-east-2",
	}

	switch i {
	case 1:
		newBackend.Endpoint = "api/v2.5/balance"
	case 2:
		newBackend.Endpoint = "api/v2.5/deposit"
	case 3:
		newBackend.Endpoint = "api/v2.5/withdrawn"
	}

	return newBackend
}

func getRandomNumber(min int, max int) int {
	i := (rand.Intn(max-min+1) + min)
	return i
}

func generateTraces(numberOfTraces int) ptrace.Traces {
	traces := ptrace.NewTraces()

	for i := 0; i < numberOfTraces; i++ {
		newAtm := generateAtm()
		newBackendSystem := generateBackendSystem()

		resourceSpan := traces.ResourceSpans().AppendEmpty()
		atmResource := resourceSpan.Resource()
		fillResourceWithAtm(&atmResource, newAtm)

		atmInstScope := appendAtmSystemInstrScopeSpans(&resourceSpan)

		resourceSpan = traces.ResourceSpans().AppendEmpty()
		backendResource := resourceSpan.Resource()
		fillResourceWithBackendSystem(&backendResource, newBackendSystem)

		backendInstScope := appendAtmSystemInstrScopeSpans(&resourceSpan)

		appendTraceSpans(&newBackendSystem, &backendInstScope, &atmInstScope)
	}

	return traces
}

func fillResourceWithAtm(resource *pcommon.Resource, atm Atm) {
	atmAttrs := resource.Attributes()
	atmAttrs.PutInt("atm.id", atm.ID)
	atmAttrs.PutStr("atm.stateid", atm.StateID)
	atmAttrs.PutStr("atm.ispnetwork", atm.ISPNetwork)
	atmAttrs.PutStr("atm.serialnumber", atm.SerialNumber)
	atmAttrs.PutStr(string(semconv.ServiceNameKey), atm.Name)
	atmAttrs.PutStr(string(semconv.ServiceVersionKey), atm.Version)

}

func fillResourceWithBackendSystem(resource *pcommon.Resource, backend BackendSystem) {
	backendAttrs := resource.Attributes()
	var osType, cloudProvider string

	switch {
	case backend.CloudProvider == "amzn":
		cloudProvider = semconv.CloudProviderAWS.Value.AsString()
	case backend.CloudProvider == "mcrsft":
		cloudProvider = semconv.CloudProviderAzure.Value.AsString()
	case backend.CloudProvider == "gogl":
		cloudProvider = semconv.CloudProviderGCP.Value.AsString()
	}

	backendAttrs.PutStr(string(semconv.CloudProviderKey), cloudProvider)
	backendAttrs.PutStr(string(semconv.CloudRegionKey), backend.CloudRegion)

	switch {
	case backend.OSType == "lnx":
		osType = semconv.OSTypeLinux.Value.AsString()
	case backend.OSType == "wndws":
		osType = semconv.OSTypeWindows.Value.AsString()
	case backend.OSType == "slrs":
		osType = semconv.OSTypeSolaris.Value.AsString()
	}

	backendAttrs.PutStr(string(semconv.OSTypeKey), osType)
	backendAttrs.PutStr(string(semconv.OSVersionKey), backend.OSVersion)

	backendAttrs.PutStr(string(semconv.ServiceNameKey), backend.ProcessName)
	backendAttrs.PutStr(string(semconv.ServiceVersionKey), backend.Version)
}

func appendAtmSystemInstrScopeSpans(resourceSpans *ptrace.ResourceSpans) ptrace.ScopeSpans {
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
	scopeSpans.Scope().SetName("atm-system")
	scopeSpans.Scope().SetVersion("v1.0")
	return scopeSpans
}

func NewTraceID() pcommon.TraceID {
	return pcommon.TraceID(uuid.New())
}

func NewSpanID() pcommon.SpanID {
	var rngSeed int64
	_ = binary.Read(crand.Reader, binary.LittleEndian, &rngSeed)
	randSource := rand.New(rand.NewSource(rngSeed))

	var sid [8]byte
	randSource.Read(sid[:])
	spanID := pcommon.SpanID(sid)

	return spanID
}

func appendTraceSpans(backend *BackendSystem, backendScopeSpans *ptrace.ScopeSpans, atmScopeSpans *ptrace.ScopeSpans) {
	traceId := NewTraceID()
	backendSpanId := NewSpanID()

	backendDuration, _ := time.ParseDuration("1s")
	backendSpanStartTime := time.Now()
	backendSpanFinishTime := backendSpanStartTime.Add(backendDuration)

	backendSpan := backendScopeSpans.Spans().AppendEmpty()
	backendSpan.SetTraceID(traceId)
	backendSpan.SetSpanID(backendSpanId)
	backendSpan.SetName(backend.Endpoint)
	backendSpan.SetKind(ptrace.SpanKindServer)
	backendSpan.SetStartTimestamp(pcommon.NewTimestampFromTime(backendSpanStartTime))
	backendSpan.SetEndTimestamp(pcommon.NewTimestampFromTime(backendSpanFinishTime))
}
```

`consumer.Traces` 有一个名为 `ConsumeTraces()` 的方法，该方法负责将生成的 trace 推送给 pipeline 中的下一个消费者。你需要更新 `tailtracerReceiver` 类型中的 `Start()` 方法，并添加代码以使用它。

打开 `tailtracer/trace-receiver.go` 文件并按如下方式更新 `Start()` 方法：

```go
func (tailtracerRcvr *tailtracerReceiver) Start(ctx context.Context, host component.Host) error {
	tailtracerRcvr.host = host
	ctx = context.Background()
	ctx, tailtracerRcvr.cancel = context.WithCancel(ctx)

	interval, _ := time.ParseDuration(tailtracerRcvr.config.Interval)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
				case <-ticker.C:
					tailtracerRcvr.logger.Info("I should start processing traces now!")
					tailtracerRcvr.nextConsumer.ConsumeTraces(ctx, generateTraces(tailtracerRcvr.config.NumberOfTraces)) // new line added
				case <-ctx.Done():
					return
			}
		}
	}()

	return nil
}
```

> [!NOTE] 检查你的工作
>
> - 在 `case <-ticker.C` 条件下添加了一行，调用了 `tailtracerRcvr.nextConsumer.ConsumeTraces()` 方法，传递了在 `Start()` 方法中创建的新 context (`ctx`)，并调用了 `generateTraces()` 函数，以便将生成的 trace 推送到 pipeline 中的下一个消费者。

现在让我们再次运行 `otelcol-dev`：

```sh
go run ./otelcol-dev --config config.yaml
```

几分钟后，你应该会看到类似这样的输出：

```log
2023-11-09T11:38:19.890+0800	info	service@v0.88.0/telemetry.go:84	Setting up own telemetry...
2023-11-09T11:38:19.890+0800	info	service@v0.88.0/telemetry.go:201	Serving Prometheus metrics	{"address": ":8888", "level": "Basic"}
2023-11-09T11:38:19.890+0800	debug	exporter@v0.88.0/exporter.go:273	Stable component.	{"kind": "exporter", "data_type": "traces", "name": "otlp/jaeger"}
2023-11-09T11:38:19.890+0800	info	exporter@v0.88.0/exporter.go:275	Development component. May change in the future.	{"kind": "exporter", "data_type": "traces", "name": "debug"}
2023-11-09T11:38:19.891+0800	debug	receiver@v0.88.0/receiver.go:294	Stable component.	{"kind": "receiver", "name": "otlp", "data_type": "traces"}
2023-11-09T11:38:19.891+0800	debug	receiver@v0.88.0/receiver.go:294	Alpha component. May change in the future.	{"kind": "receiver", "name": "tailtracer", "data_type": "traces"}
2023-11-09T11:38:19.891+0800	info	service@v0.88.0/service.go:143	Starting otelcol-dev...	{"Version": "1.0.0", "NumCPU": 10}
2023-11-09T11:38:19.891+0800	info	extensions/extensions.go:33	Starting extensions...

<OMITTED>

2023-11-09T11:38:19.903+0800	info	zapgrpc/zapgrpc.go:178	[core] [Channel #1] Channel Connectivity change to READY	{"grpc_log": true}
2023-11-09T11:39:19.894+0800	info	tailtracer/trace-receiver.go:33	I should start processing traces now!	{"kind": "receiver", "name": "tailtracer", "data_type": "traces"}
2023-11-09T11:39:19.913+0800	info	TracesExporter	{"kind": "exporter", "data_type": "traces", "name": "debug", "resource spans": 4, "spans": 2}
2023-11-09T11:39:19.913+0800	info	ResourceSpans #0
Resource SchemaURL:
Resource attributes:
     -> atm.id: Int(222)
     -> atm.stateid: Str(CA)
     -> atm.ispnetwork: Str(comcast-sanfrancisco)
     -> atm.serialnumber: Str(atmxph-2022-222)
     -> service.name: Str(ATM-222-CA)
     -> service.version: Str(v1.0)
ScopeSpans #0
ScopeSpans SchemaURL:
InstrumentationScope
ResourceSpans #1
Resource SchemaURL:
Resource attributes:
     -> cloud.provider: Str(aws)
     -> cloud.region: Str(us-east-2)
     -> os.type: Str(linux)
     -> os.version: Str(4.16.10-300.fc28.x86_64)
     -> service.name: Str(accounts)
     -> service.version: Str(v2.5)
ScopeSpans #0
ScopeSpans SchemaURL:
InstrumentationScope
Span #0
    Trace ID       : bbcb00aead044a138cf96c0bf4a4ba83
    Parent ID      :
    ID             : 5056fe4e9adf621c
    Name           : api/v2.5/withdrawn
    Kind           : Server
    Start time     : 2023-11-09 03:39:19.894881 +0000 UTC
    End time       : 2023-11-09 03:39:20.894881 +0000 UTC
    Status code    : Unset
    Status message :
ResourceSpans #2
Resource SchemaURL:
Resource attributes:
     -> atm.id: Int(111)
     -> atm.stateid: Str(IL)
     -> atm.ispnetwork: Str(comcast-chicago)
     -> atm.serialnumber: Str(atmxph-2022-111)
     -> service.name: Str(ATM-111-IL)
     -> service.version: Str(v1.0)
ScopeSpans #0
ScopeSpans SchemaURL:
InstrumentationScope
ResourceSpans #3
Resource SchemaURL:
Resource attributes:
     -> cloud.provider: Str(aws)
     -> cloud.region: Str(us-east-2)
     -> os.type: Str(linux)
     -> os.version: Str(4.16.10-300.fc28.x86_64)
     -> service.name: Str(accounts)
     -> service.version: Str(v2.5)
ScopeSpans #0
ScopeSpans SchemaURL:
InstrumentationScope
Span #0
    Trace ID       : ba013b8223ec4d29806ae493ecd1a5e4
    Parent ID      :
    ID             : 4feb47b55c9c4129
    Name           : api/v2.5/withdrawn
    Kind           : Server
    Start time     : 2023-11-09 03:39:19.894953 +0000 UTC
    End time       : 2023-11-09 03:39:20.894953 +0000 UTC
    Status code    : Unset
    Status message :
	{"kind": "exporter", "data_type": "traces", "name": "debug"}
...
```

以下是生成的 trace 在 Jaeger 中的外观：
![Jaeger trace](/img/docs/tutorials/Jaeger-BackendSystem-Trace.png)

你目前在 Jaeger 中看到的内容表示一个服务正在接收来自未被 OTel SDK 插桩的外部实体的请求。因此，它不能被识别为 trace 的起点/开始。为了让 `ptrace.Span` 理解它所表示的操作是由于在相同 trace 上下文中、源自同一 `Resource` 内部或外部的另一个操作（嵌套/子操作）而执行的，你需要：

- 通过调用 `SetTraceID()` 方法并将父/调用者 `ptrace.Span` 的 `pcommon.TraceID` 作为参数传递，来设置与调用者操作相同的 trace 上下文。
- 通过调用 `SetParentSpanID()` 方法并将父/调用者 `ptrace.Span` 的 `pcommon.SpanID` 作为参数传递，在 trace 的上下文中定义调用者操作。

你现在将创建一个表示 `Atm` 实体操作的 `ptrace.Span`，并将其设置为 `BackendSystem` span 的父 span。打开 `tailtracer/model.go` 文件并按如下方式更新 `appendTraceSpans()` 函数：

```go
func appendTraceSpans(backend *BackendSystem, backendScopeSpans *ptrace.ScopeSpans, atmScopeSpans *ptrace.ScopeSpans) {
	traceId := NewTraceID()

	var atmOperationName string

	switch {
		case strings.Contains(backend.Endpoint, "balance"):
			atmOperationName = "Check Balance"
		case strings.Contains(backend.Endpoint, "deposit"):
			atmOperationName = "Make Deposit"
		case strings.Contains(backend.Endpoint, "withdraw"):
			atmOperationName = "Fast Cash"
		}

	atmSpanId := NewSpanID()
	atmSpanStartTime := time.Now()
	atmDuration, _ := time.ParseDuration("4s")
	atmSpanFinishTime := atmSpanStartTime.Add(atmDuration)

	atmSpan := atmScopeSpans.Spans().AppendEmpty()
	atmSpan.SetTraceID(traceId)
	atmSpan.SetSpanID(atmSpanId)
	atmSpan.SetName(atmOperationName)
	atmSpan.SetKind(ptrace.SpanKindClient)
	atmSpan.Status().SetCode(ptrace.StatusCodeOk)
	atmSpan.SetStartTimestamp(pcommon.NewTimestampFromTime(atmSpanStartTime))
	atmSpan.SetEndTimestamp(pcommon.NewTimestampFromTime(atmSpanFinishTime))

	backendSpanId := NewSpanID()

	backendDuration, _ := time.ParseDuration("2s")
	backendSpanStartTime := atmSpanStartTime.Add(backendDuration)

	backendSpan := backendScopeSpans.Spans().AppendEmpty()
	backendSpan.SetTraceID(atmSpan.TraceID())
	backendSpan.SetSpanID(backendSpanId)
	backendSpan.SetParentSpanID(atmSpan.SpanID())
	backendSpan.SetName(backend.Endpoint)
	backendSpan.SetKind(ptrace.SpanKindServer)
	backendSpan.Status().SetCode(ptrace.StatusCodeOk)
	backendSpan.SetStartTimestamp(pcommon.NewTimestampFromTime(backendSpanStartTime))
	backendSpan.SetEndTimestamp(atmSpan.EndTimestamp())
}
```

以下是最终的 `tailtracer/model.go` 文件的样子：

> tailtracer/model.go

```go
package tailtracer

import (
	crand "crypto/rand"
	"encoding/binary"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	 "go.opentelemetry.io/otel/semconv/v1.38.0"
)

type Atm struct {
	ID           int64
	Version      string
	Name         string
	StateID      string
	SerialNumber string
	ISPNetwork   string
}

type BackendSystem struct {
	Version       string
	ProcessName   string
	OSType        string
	OSVersion     string
	CloudProvider string
	CloudRegion   string
	Endpoint      string
}

func generateAtm() Atm {
	i := getRandomNumber(1, 2)
	var newAtm Atm

	switch i {
	case 1:
		newAtm = Atm{
			ID:           111,
			Name: "ATM-111-IL",
			SerialNumber: "atmxph-2022-111",
			Version:      "v1.0",
			ISPNetwork:   "comcast-chicago",
			StateID:      "IL",
		}

	case 2:
		newAtm = Atm{
			ID:           222,
			Name: "ATM-222-CA",
			SerialNumber: "atmxph-2022-222",
			Version:      "v1.0",
			ISPNetwork:   "comcast-sanfrancisco",
			StateID:      "CA",
		}
	}

	return newAtm
}

func generateBackendSystem() BackendSystem {
	i := getRandomNumber(1, 3)

	newBackend := BackendSystem{
		ProcessName:   "accounts",
		Version:       "v2.5",
		OSType:        "lnx",
		OSVersion:     "4.16.10-300.fc28.x86_64",
		CloudProvider: "amzn",
		CloudRegion:   "us-east-2",
	}

	switch i {
	case 1:
		newBackend.Endpoint = "api/v2.5/balance"
	case 2:
		newBackend.Endpoint = "api/v2.5/deposit"
	case 3:
		newBackend.Endpoint = "api/v2.5/withdrawn"
	}

	return newBackend
}

func getRandomNumber(min int, max int) int {
	i := (rand.Intn(max-min+1) + min)
	return i
}

func generateTraces(numberOfTraces int) ptrace.Traces {
	traces := ptrace.NewTraces()

	for i := 0; i < numberOfTraces; i++ {
		newAtm := generateAtm()
		newBackendSystem := generateBackendSystem()

		resourceSpan := traces.ResourceSpans().AppendEmpty()
		atmResource := resourceSpan.Resource()
		fillResourceWithAtm(&atmResource, newAtm)

		atmInstScope := appendAtmSystemInstrScopeSpans(&resourceSpan)

		resourceSpan = traces.ResourceSpans().AppendEmpty()
		backendResource := resourceSpan.Resource()
		fillResourceWithBackendSystem(&backendResource, newBackendSystem)

		backendInstScope := appendAtmSystemInstrScopeSpans(&resourceSpan)

		appendTraceSpans(&newBackendSystem, &backendInstScope, &atmInstScope)
	}

	return traces
}

func fillResourceWithAtm(resource *pcommon.Resource, atm Atm) {
	atmAttrs := resource.Attributes()
	atmAttrs.PutInt("atm.id", atm.ID)
	atmAttrs.PutStr("atm.stateid", atm.StateID)
	atmAttrs.PutStr("atm.ispnetwork", atm.ISPNetwork)
	atmAttrs.PutStr("atm.serialnumber", atm.SerialNumber)
	atmAttrs.PutStr(string(semconv.ServiceNameKey), atm.Name)
	atmAttrs.PutStr(string(semconv.ServiceVersionKey), atm.Version)

}

func fillResourceWithBackendSystem(resource *pcommon.Resource, backend BackendSystem) {
	backendAttrs := resource.Attributes()
	var osType, cloudProvider string

	switch {
	case backend.CloudProvider == "amzn":
		cloudProvider = semconv.CloudProviderAWS.Value.AsString()
	case backend.CloudProvider == "mcrsft":
		cloudProvider = semconv.CloudProviderAzure.Value.AsString()
	case backend.CloudProvider == "gogl":
		cloudProvider = semconv.CloudProviderGCP.Value.AsString()
	}

	backendAttrs.PutStr(string(semconv.CloudProviderKey), cloudProvider)
	backendAttrs.PutStr(string(semconv.CloudRegionKey), backend.CloudRegion)

	switch {
	case backend.OSType == "lnx":
		osType = semconv.OSTypeLinux.Value.AsString()
	case backend.OSType == "wndws":
		osType = semconv.OSTypeWindows.Value.AsString()
	case backend.OSType == "slrs":
		osType = semconv.OSTypeSolaris.Value.AsString()
	}

	backendAttrs.PutStr(string(semconv.OSTypeKey), osType)
	backendAttrs.PutStr(string(semconv.OSVersionKey), backend.OSVersion)

	backendAttrs.PutStr(string(semconv.ServiceNameKey), backend.ProcessName)
	backendAttrs.PutStr(string(semconv.ServiceVersionKey), backend.Version)
}

func appendAtmSystemInstrScopeSpans(resourceSpans *ptrace.ResourceSpans) ptrace.ScopeSpans {
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
	scopeSpans.Scope().SetName("atm-system")
	scopeSpans.Scope().SetVersion("v1.0")
	return scopeSpans
}

func NewTraceID() pcommon.TraceID {
	return pcommon.TraceID(uuid.New())
}

func NewSpanID() pcommon.SpanID {
	var rngSeed int64
	_ = binary.Read(crand.Reader, binary.LittleEndian, &rngSeed)
	randSource := rand.New(rand.NewSource(rngSeed))

	var sid [8]byte
	randSource.Read(sid[:])
	spanID := pcommon.SpanID(sid)

	return spanID
}

func appendTraceSpans(backend *BackendSystem, backendScopeSpans *ptrace.ScopeSpans, atmScopeSpans *ptrace.ScopeSpans) {
	traceId := NewTraceID()

	var atmOperationName string

	switch {
	case strings.Contains(backend.Endpoint, "balance"):
		atmOperationName = "Check Balance"
	case strings.Contains(backend.Endpoint, "deposit"):
		atmOperationName = "Make Deposit"
	case strings.Contains(backend.Endpoint, "withdraw"):
		atmOperationName = "Fast Cash"
	}

	atmSpanId := NewSpanID()
	atmSpanStartTime := time.Now()
	atmDuration, _ := time.ParseDuration("4s")
	atmSpanFinishTime := atmSpanStartTime.Add(atmDuration)

	atmSpan := atmScopeSpans.Spans().AppendEmpty()
	atmSpan.SetTraceID(traceId)
	atmSpan.SetSpanID(atmSpanId)
	atmSpan.SetName(atmOperationName)
	atmSpan.SetKind(ptrace.SpanKindClient)
	atmSpan.Status().SetCode(ptrace.StatusCodeOk)
	atmSpan.SetStartTimestamp(pcommon.NewTimestampFromTime(atmSpanStartTime))
	atmSpan.SetEndTimestamp(pcommon.NewTimestampFromTime(atmSpanFinishTime))

	backendSpanId := NewSpanID()

	backendDuration, _ := time.ParseDuration("2s")
	backendSpanStartTime := atmSpanStartTime.Add(backendDuration)

	backendSpan := backendScopeSpans.Spans().AppendEmpty()
	backendSpan.SetTraceID(atmSpan.TraceID())
	backendSpan.SetSpanID(backendSpanId)
	backendSpan.SetParentSpanID(atmSpan.SpanID())
	backendSpan.SetName(backend.Endpoint)
	backendSpan.SetKind(ptrace.SpanKindServer)
	backendSpan.Status().SetCode(ptrace.StatusCodeOk)
	backendSpan.SetStartTimestamp(pcommon.NewTimestampFromTime(backendSpanStartTime))
	backendSpan.SetEndTimestamp(atmSpan.EndTimestamp())
}
```

再次运行 `otelcol-dev`：

```sh
go run ./otelcol-dev --config config.yaml
```

大约 2 分钟后，你应该会开始在 Jaeger 中看到类似于以下内容的 trace：
![Jaeger trace](/img/docs/tutorials/Jaeger-FullSystem-Traces-List.png)

我们现在在系统中拥有了同时表示 `Atm` 和 `BackendSystem` 遥测数据生成实体的服务。我们完全理解了这两个实体是如何被使用的，以及它们如何对用户执行的操作的性能做出贡献。

以下是其中一个 trace 在 Jaeger 中的详细视图：
![Jaeger trace](/img/docs/tutorials/Jaeger-FullSystem-Trace-Details.png)

就是这样！你现在已经完成了本教程，并成功实现了一个 trace receiver，恭喜！
