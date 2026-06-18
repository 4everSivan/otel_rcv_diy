# 使用 OpenTelemetry Collector Builder 构建自定义 Collector组装你自己的 OpenTelemetry Collector 发行版

LLMS index: [llms.txt]()⁠

------

OpenTelemetry Collector 提供五种官方[发行版]()⁠，这些发行版已预先配置了特定组件。如果你需要更高的灵活性，可以使用 [OpenTelemetry Collector Builder](https://github.com/open-telemetry/opentelemetry-collector/tree/main/cmd/builder)⁠（简称 `ocb`）来生成你自己的自定义 Collector 二进制发行版，其中可以包含自定义组件、上游组件以及其他公开可用的组件。

以下指南将帮助你使用 `ocb` 开始构建自己的 Collector。在这个示例中，你将创建一个用于开发和测试自定义组件的 Collector 发行版。你可以直接在你喜欢的 Golang 集成开发环境（IDE）中启动并调试 Collector 组件。利用 IDE 的全部调试能力（例如堆栈跟踪——非常好的学习工具！）来理解 Collector 如何与你的组件代码交互。

## **前置条件**

`ocb` 工具需要 Go 才能构建 Collector 发行版。在开始之前，请确保你的机器已安装 [Go](https://go.dev/doc/install) 的 [兼容版本](https://github.com/open-telemetry/opentelemetry-collector/blob/main/README.md#compatibility)⁠。

## **安装 OpenTelemetry Collector Builder**

`ocb` 二进制文件可从 OpenTelemetry Collector 发布版本中下载，带有 [`cmd/builder`](https://github.com/open-telemetry/opentelemetry-collector-releases/tags)⁠[ 标签](https://github.com/open-telemetry/opentelemetry-collector-releases/tags)⁠。请选择适合你操作系统和芯片架构的安装包：

* Linux (AMD 64)

  ```sh
  curl --proto '=https' --tlsv1.2 -fL -o ocb \
  https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/cmd%2Fbuilder%2Fv0.154.0/ocb_0.154.0_linux_amd64
  
  chmod +x ocb
  ```

* Linux (ARM 64)

  ```sh
  curl --proto '=https' --tlsv1.2 -fL -o ocb \
  https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/cmd%2Fbuilder%2Fv0.154.0/ocb_0.154.0_linux_arm64

  chmod +x ocb
  ```

* Linux (ppc64le)

  ```sh
  curl --proto '=https' --tlsv1.2 -fL -o ocb \
  https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/cmd%2Fbuilder%2Fv0.154.0/ocb_0.154.0_linux_ppc64le

  chmod +x ocb
  ```

* macOS (AMD 64)

  ```sh
  curl --proto '=https' --tlsv1.2 -fL -o ocb \
  https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/cmd%2Fbuilder%2Fv0.154.0/ocb_0.154.0_darwin_amd64

  chmod +x ocb
  ```

* macOS (ARM 64)

  ```sh
  curl --proto '=https' --tlsv1.2 -fL -o ocb \
  https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/cmd%2Fbuilder%2Fv0.154.0/ocb_0.154.0_darwin_arm64

  chmod +x ocb
  ```

* Windows (AMD 64)

  ```powershell
  Invoke-WebRequest -Uri "https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/cmd%2Fbuilder%2Fv0.154.0/ocb_0.154.0_windows_amd64.exe" -OutFile "ocb.exe"
  
  Unblock-File -Path "ocb.exe"
  ```

为了确认 `ocb` 是否安装成功，请在终端输入 `./ocb help`。你应该能看到 help 命令的输出。

## **配置 OpenTelemetry Collector Builder**

使用 YAML 清单文件配置 `ocb`。该清单主要包含两个部分：

- 第一部分 `dist`：用于代码生成和编译过程的配置
- 第二部分：顶层模块类型，如 `extensions`、`exporters`、`receivers` 或 `processors`，每个模块类型都接受组件列表

`dist` 部分的标签等价于 `ocb` 命令行参数。以下是配置项说明：

| **标签**          | **说明**                  | **是否可选**     | **默认值**                                                   |
| ----------------- | ------------------------- | ---------------- | ------------------------------------------------------------ |
| module            | 新发行版的 Go module 名称 | 可选，但建议填写 | `go.opentelemetry.io/collector/cmd/builder`                  |
| name              | 发行版二进制名称          | 可选             | `otelcol-custom`                                             |
| description       | 应用的描述                | 可选             | `Custom OpenTelemetry Collector distribution`                |
| output_path       | 输出路径（源码和二进制）  | 可选             | `/var/folders/86/s7l1czb16g124tng0d7wyrtw0000gn/T/otelcol-distribution3618633831` |
| version           | 自定义 Collector 版本     | 可选             | `1.0.0`                                                      |
| go                | 用于编译的 Go 可执行文件  | 可选             | PATH 中的 go                                                 |
| debug_compilation | 是否保留调试符号          | 可选             | False                                                        |

所有 `dist` 配置项都是可选的。你可以根据是否要发布该 Collector 发行版或仅用于开发测试环境来调整配置。

按照以下步骤配置 `ocb`：

1. 创建名为 `builder-config.yaml` 的清单文件：

```yaml
dist:
  name: otelcol-dev
  description: Basic OTel Collector distribution for Developers
  output_path: ./otelcol-dev
```

1. 为你要包含在自定义 Collector 发行版中的组件添加模块。可参考
    [`ocb`](https://github.com/open-telemetry/opentelemetry-collector/tree/main/cmd/builder#configuration)⁠[ 配置文档](https://github.com/open-telemetry/opentelemetry-collector/tree/main/cmd/builder#configuration)⁠ 了解各模块类型。

   

   在本示例中，我们添加以下组件：

   - Exporters：OTLP 和 Debug
   - Receivers：OTLP
   - Processors：Batch

   完整 `builder-config.yaml` 如下：

```yaml
dist:
  name: otelcol-dev
  description: Basic OTel Collector distribution for Developers
  output_path: ./otelcol-dev

exporters:
  - gomod:
      go.opentelemetry.io/collector/exporter/debugexporter v0.154.0
  - gomod:
      go.opentelemetry.io/collector/exporter/otlpexporter v0.154.0

processors:
  - gomod:
      go.opentelemetry.io/collector/processor/batchprocessor v0.154.0

receivers:
  - gomod:
      go.opentelemetry.io/collector/receiver/otlpreceiver v0.154.0

providers:
  - gomod:
      go.opentelemetry.io/collector/confmap/provider/envprovider v1.48.0
  - gomod:
      go.opentelemetry.io/collector/confmap/provider/fileprovider v1.48.0
  - gomod:
      go.opentelemetry.io/collector/confmap/provider/httpprovider v1.48.0
  - gomod:
      go.opentelemetry.io/collector/confmap/provider/httpsprovider v1.48.0
  - gomod:
      go.opentelemetry.io/collector/confmap/provider/yamlprovider v1.48.0
```

[!TIP]

如果你想查看可用组件列表，请访问
 [OpenTelemetry Registry]()⁠。每个条目都包含你需要写入 `builder-config.yaml` 的完整名称和版本。

## **生成代码并构建 Collector 发行版**

[!NOTE]

本节使用 `ocb` 二进制构建自定义 Collector。如果你希望将 Collector 部署到 Kubernetes 等容器平台，请跳过本节，参考
 [容器化你的 Collector 发行版](#containerize-your-collector-distribution)⁠。

安装并配置好 `ocb` 后，就可以开始构建发行版了。

在终端中执行：

```sh
./ocb --config builder-config.yaml
```

命令输出如下：

```text
2025-06-13T14:25:03.037-0500	INFO	internal/command.go:85	OpenTelemetry Collector distribution builder	{"version": "0.154.0", "date": "2025-06-03T15:05:37Z"}
2025-06-13T14:25:03.039-0500	INFO	internal/command.go:108	使用配置文件	{"path": "builder-config.yaml"}
2025-06-13T14:25:03.040-0500	INFO	builder/config.go:99	使用 Go	{"go-executable": "/usr/local/go/bin/go"}
2025-06-13T14:25:03.041-0500	INFO	builder/main.go:76	已生成源码	{"path": "./otelcol-dev"}
2025-06-13T14:25:03.445-0500	INFO	builder/main.go:108	获取 Go modules
2025-06-13T14:25:04.675-0500	INFO	builder/main.go:87	编译中
2025-06-13T14:25:17.259-0500	INFO	builder/main.go:94	编译完成	{"binary": "./otelcol-dev/otelcol-dev"}
```

根据 `dist` 配置，你现在会得到一个名为 `otelcol-dev` 的文件夹，其中包含 Collector 发行版的源码和二进制文件。

目录结构如下：

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

你可以使用这些生成代码来启动组件开发项目，并基于这些组件构建和发布自己的 Collector 发行版。

## **容器化你的 Collector 发行版**

[!NOTE]

本节介绍如何通过 `Dockerfile` 构建 Collector 发行版。如果你需要部署到 Kubernetes 等容器平台，请按照本节操作；如果不需要容器化，请参考
 [生成代码并构建发行版](#generate-the-code-and-build-your-collector-distribution)⁠。

按照以下步骤容器化你的自定义 Collector：

1. 添加两个新文件：

   - `Dockerfile`：Collector 发行版的容器镜像定义
   - `collector-config.yaml`：用于测试的最小 Collector 配置

   此时目录结构如下：

```text
.
├── builder-config.yaml
├── collector-config.yaml
└── Dockerfile
```

1. 在 `Dockerfile` 中添加如下内容。该 Dockerfile 会在容器内构建 Collector，并确保二进制与目标架构（如 `linux/amd64`、`linux/arm64`）一致：

```dockerfile
FROM alpine:3.19 AS certs
RUN apk --update add ca-certificates

FROM golang:1.25.0 AS build-stage
WORKDIR /build

COPY ./builder-config.yaml builder-config.yaml

RUN --mount=type=cache,target=/root/.cache/go-build GO111MODULE=on go install go.opentelemetry.io/collector/cmd/builder@v0.154.0
RUN --mount=type=cache,target=/root/.cache/go-build builder --config builder-config.yaml

FROM gcr.io/distroless/base:latest

ARG USER_UID=10001
USER ${USER_UID}

COPY ./collector-config.yaml /otelcol/collector-config.yaml
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --chmod=755 --from=build-stage /build/otelcol-dev /otelcol

ENTRYPOINT ["/otelcol/otelcol-dev"]
CMD ["--config", "/otelcol/collector-config.yaml"]

EXPOSE 4317 4318 12001
```

[!NOTE]

Dockerfile 中引用的发行版名称 `otelcol-dev` 来自 `builder-config.yaml`。如果你修改了 `name` 或 `output_path`，请同步更新：

- `COPY --chmod=755 --from=build-stage /build/<dist_name> /otelcol`
- `ENTRYPOINT ["/otelcol/<dist_name>"]`

1. 在 `collector-config.yaml` 中添加如下内容：

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

exporters:
  debug:
    verbosity: detailed

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [debug]
    metrics:
      receivers: [otlp]
      exporters: [debug]
    logs:
      receivers: [otlp]
      exporters: [debug]
```

1. 使用以下命令构建支持 `linux/amd64` 和 `linux/arm64` 的多架构 Docker 镜像：

```sh
# 启用 Docker 多架构支持
docker run --rm --privileged tonistiigi/binfmt --install all
docker buildx create --name mybuilder --use

# 构建多架构镜像
docker buildx build --load \
  -t <collector_distribution_image_name>:<version> \
  --platform=linux/amd64,linux/arm64 .

# 运行测试
docker run -it --rm -p 4317:4317 -p 4318:4318 \
    --name otelcol <collector_distribution_image_name>:<version>
```

## **延伸阅读**

- [构建 receiver]()⁠
- [构建 connector]()⁠