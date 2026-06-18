package patronireceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"patronireceiver/internal/metadata"
)

// NewFactory 创建 patronireceiver 的工厂，注册到 Collector 组件表中。
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, component.StabilityLevelDevelopment),
	)
}

// createDefaultConfig 返回 patronireceiver 的默认配置。
//
// 各部分默认值：
//   - scraperhelper.ControllerConfig  → collection_interval: 10s / initial_delay: 1s
//   - confighttp.ClientConfig         → endpoint: 占位（用户需显式配置） / timeout: 30s
//   - metadata.MetricsBuilderConfig   → 所有指标 enabled: true / 默认属性集
func createDefaultConfig() component.Config {
	return &Config{
		ControllerConfig:      scraperhelper.NewDefaultControllerConfig(),
		ClientConfig:          confighttp.NewDefaultClientConfig(),
		MetricsBuilderConfig:  metadata.NewDefaultMetricsBuilderConfig(),
	}
}

// createMetricsReceiver 实例化 patronireceiver：
//  1. 创建 patroniScraper（实现 scraper.Metrics 接口）
//  2. 用 scraperhelper.NewMetricsController 包装，自动管理定时 / context 取消 / 错误
//  3. 将下游消费者注入到 Controller 中
func createMetricsReceiver(
	_ context.Context,
	set receiver.Settings,
	baseCfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := baseCfg.(*Config)

	scraper := newPatroniScraper(set, cfg)

	return scraperhelper.NewMetricsController(
		&cfg.ControllerConfig,
		set,
		nextConsumer,
		scraperhelper.AddMetricsScraper(metadata.Type, scraper),
	)
}
