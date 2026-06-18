package patronireceiver

import (
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"patronireceiver/internal/metadata"
)

// Config 定义了 patronireceiver 的配置结构。
//
// 通过 squash 继承三部分：
//   - scraperhelper.ControllerConfig  → collection_interval / initial_delay 等定时采集能力
//   - confighttp.ClientConfig         → endpoint / tls / auth / timeout 等 HTTP 客户端能力
//   - metadata.MetricsBuilderConfig   → 按指标粒度的 enabled / attributes 裁剪能力
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	confighttp.ClientConfig        `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
}
