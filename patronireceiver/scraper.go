package patronireceiver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"patronireceiver/internal/metadata"
)

// patroniScraper 实现 scraper.Metrics 接口。
type patroniScraper struct {
	logger         *zap.Logger
	cfg            *Config
	telemetry      component.TelemetrySettings
	client         *http.Client
	builder        *metadata.MetricsBuilder
}

func newPatroniScraper(set receiver.Settings, cfg *Config) *patroniScraper {
	return &patroniScraper{
		logger:    set.Logger,
		cfg:       cfg,
		telemetry: set.TelemetrySettings,
	}
}

var _ component.Component = (*patroniScraper)(nil)

// Start 初始化 HTTP 客户端和 MetricsBuilder。
func (s *patroniScraper) Start(ctx context.Context, host component.Host) error {
	client, err := s.cfg.ClientConfig.ToClient(ctx, host.GetExtensions(), s.telemetry)
	if err != nil {
		return fmt.Errorf("create http client: %w", err)
	}
	s.client = client

	s.builder = metadata.NewMetricsBuilder(
		s.cfg.MetricsBuilderConfig,
		receiver.Settings{
			ID:                component.NewID(metadata.Type),
			TelemetrySettings: s.telemetry,
		},
	)

	return nil
}

// Shutdown 释放 HTTP 客户端资源。
func (s *patroniScraper) Shutdown(_ context.Context) error {
	if s.client != nil {
		s.client.CloseIdleConnections()
	}
	return nil
}

// ScrapeMetrics 执行一次完整的指标采集。
func (s *patroniScraper) ScrapeMetrics(ctx context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(time.Now())

	// --- 1. GET /patroni ---
	patroniJSON, err := s.fetch(ctx, "/patroni")
	if err != nil {
		return pmetric.NewMetrics(), fmt.Errorf("fetch /patroni: %w", err)
	}

	var patroniResp patroniResponse
	if err := json.Unmarshal(patroniJSON, &patroniResp); err != nil {
		return pmetric.NewMetrics(), fmt.Errorf("parse /patroni: %w", err)
	}

	meta := extractNodeMeta(patroniResp)
	s.recordNodeMetrics(now, patroniResp, meta)

	// --- 2. GET /cluster ---
	clusterJSON, err := s.fetch(ctx, "/cluster")
	if err != nil {
		s.logger.Warn("fetch /cluster failed, skipping replication lag metrics",
			zap.Error(err))
	} else {
		var clusterResp clusterResponse
		if err := json.Unmarshal(clusterJSON, &clusterResp); err != nil {
			s.logger.Warn("parse /cluster failed", zap.Error(err))
		} else {
			s.recordReplicationLag(now, clusterResp, meta)
		}
	}

	// --- 3. GET /history ---
	historyJSON, err := s.fetch(ctx, "/history")
	if err != nil {
		s.logger.Warn("fetch /history failed, skipping history metrics",
			zap.Error(err))
	} else {
		var history []historyEntry
		if err := json.Unmarshal(historyJSON, &history); err != nil {
			s.logger.Warn("parse /history failed", zap.Error(err))
		} else {
			s.recordHistory(now, history, meta)
		}
	}

	// --- 4. Emit ---
	resource := s.buildResource(meta)
	return s.builder.Emit(metadata.WithResource(resource)), nil
}

// ============================================================
// HTTP 辅助
// ============================================================

// fetch 对 Patroni API 执行 GET 请求，返回原始 body。
func (s *patroniScraper) fetch(ctx context.Context, path string) ([]byte, error) {
	endpoint := strings.TrimRight(s.cfg.Endpoint, "/")
	url := endpoint + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request %s: %w", url, err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10 MB cap
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", url, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s returned %d: %s", url, resp.StatusCode, string(body))
	}

	return body, nil
}

// ============================================================
// 指标填充
// ============================================================

func (s *patroniScraper) recordNodeMetrics(
	now pcommon.Timestamp,
	p patroniResponse,
	meta patroniNodeMeta,
) {
	scope := meta.scope
	name := meta.name
	role := meta.role
	state := meta.state
	version := meta.version

	// 0/1 布尔 Gauge
	s.builder.RecordPatroniPostgresRunningDataPoint(now,
		boolToInt64(p.State == "running"), name, role, scope)
	s.builder.RecordPatroniPrimaryDataPoint(now,
		boolToInt64(p.Role == "primary"), scope, name)
	s.builder.RecordPatroniReplicaDataPoint(now,
		boolToInt64(p.Role == "replica"), scope, name)
	s.builder.RecordPatroniStandbyLeaderDataPoint(now,
		boolToInt64(p.Role == "standby_leader"), scope, name)

	// 同步/仲裁 standby
	var isSync, isQuorum int64
	for _, rep := range p.Replication {
		if rep.SyncState == "sync" {
			isSync = 1
		}
		if rep.SyncState == "quorum" {
			isQuorum = 1
		}
	}
	s.builder.RecordPatroniSyncStandbyDataPoint(now, isSync, scope, name)
	s.builder.RecordPatroniQuorumStandbyDataPoint(now, isQuorum, scope, name)

	// 流复制状态
	var streaming, inArchive int64
	for _, rep := range p.Replication {
		if rep.State == "streaming" {
			streaming = 1
		}
		if rep.State == "archive" {
			inArchive = 1
		}
	}
	s.builder.RecordPatroniPostgresStreamingDataPoint(now, streaming, scope, name)
	s.builder.RecordPatroniPostgresInArchiveRecoveryDataPoint(now, inArchive, scope, name)

	// 集群/故障保护/维护态
	s.builder.RecordPatroniClusterUnlockedDataPoint(now,
		ptrBoolToInt64(p.ClusterUnlocked), scope, name)
	s.builder.RecordPatroniFailsafeModeActiveDataPoint(now,
		ptrBoolToInt64(p.FailsafeModeActive), scope, name)
	s.builder.RecordPatroniXlogPausedDataPoint(now,
		ptrBoolToInt64(p.Xlog.Paused), scope, name)
	s.builder.RecordPatroniPendingRestartDataPoint(now,
		ptrBoolToInt64(p.PendingRestart), scope, name)
	s.builder.RecordPatroniIsPausedDataPoint(now,
		ptrBoolToInt64(p.Pause), scope, name)

	// 数值 Gauge
	s.builder.RecordPatroniPostgresStateDataPoint(now,
		postgresStateToInt(state), name, role, scope, state)
	s.builder.RecordPatroniVersionDataPoint(now,
		patroniVersionToInt(version), name, scope, version)
	s.builder.RecordPatroniPostgresServerVersionDataPoint(now,
		int64(p.ServerVersion), scope, name)

	// postmaster_start_time
	pmTime, err := parsePostgresTimestamp(p.PostmasterStartTime)
	if err != nil {
		s.logger.Debug("parse postmaster_start_time failed", zap.Error(err))
		s.builder.RecordPatroniPostmasterStartTimeDataPoint(now, 0, scope, name)
	} else {
		s.builder.RecordPatroniPostmasterStartTimeDataPoint(now,
			float64(pmTime.Unix()), scope, name)
	}

	// xlog replayed_timestamp
	replayedTS := int64(0)
	if p.Xlog.ReplayedTimestamp != nil {
		replayedTS = *p.Xlog.ReplayedTimestamp
	}
	s.builder.RecordPatroniXlogReplayedTimestampDataPoint(now,
		float64(replayedTS), scope, name)

	// dcs_last_seen
	s.builder.RecordPatroniDcsLastSeenDataPoint(now,
		p.DCSSLastSeen, scope, name)

	// Counter（WAL 位置 / timeline）
	s.builder.RecordPatroniXlogLocationDataPoint(now,
		p.Xlog.Location, scope, name)
	s.builder.RecordPatroniXlogReceivedLocationDataPoint(now,
		p.Xlog.ReceivedLocation, scope, name)
	s.builder.RecordPatroniXlogReplayedLocationDataPoint(now,
		p.Xlog.ReplayedLocation, scope, name)
	s.builder.RecordPatroniPostgresTimelineDataPoint(now,
		p.Timeline, scope, name)
}

func (s *patroniScraper) recordReplicationLag(
	now pcommon.Timestamp,
	c clusterResponse,
	meta patroniNodeMeta,
) {
	scope := meta.scope
	nodeName := meta.name

	for _, member := range c.Members {
		if member.Role != "replica" {
			continue
		}

		appName := member.Name
		clientAddr := "" // /cluster 不含 client_addr；可从 /patroni replication[] 补充
		syncState := "async"

		lag := int64(0)
		if member.Lag != nil {
			lag = *member.Lag
		}
		receiveLag := int64(0)
		if member.ReceiveLag != nil {
			receiveLag = *member.ReceiveLag
		}
		replayLag := int64(0)
		if member.ReplayLag != nil {
			replayLag = *member.ReplayLag
		}

		s.builder.RecordPatroniReplicationLagDataPoint(now,
			lag, nodeName, appName, clientAddr, syncState, scope)
		s.builder.RecordPatroniReplicationReceiveLagDataPoint(now,
			receiveLag, nodeName, appName, clientAddr, syncState, scope)
		s.builder.RecordPatroniReplicationReplayLagDataPoint(now,
			replayLag, nodeName, appName, clientAddr, syncState, scope)
	}
}

func (s *patroniScraper) recordHistory(
	now pcommon.Timestamp,
	entries []historyEntry,
	meta patroniNodeMeta,
) {
	scope := meta.scope
	nodeName := meta.name

	s.builder.RecordPatroniHistorySizeDataPoint(now,
		int64(len(entries)), scope, nodeName)

	for _, e := range entries {
		ts := pcommon.NewTimestampFromTime(e.PromotedAt)
		s.builder.RecordPatroniHistoryPromotedAtDataPoint(ts,
			float64(e.PromotedAt.Unix()), scope, nodeName, e.Timeline, e.Reason)
		s.builder.RecordPatroniHistoryLsnDataPoint(ts,
			e.LSN, scope, nodeName, e.Timeline, e.Reason)
	}
}

// ============================================================
// Resource 构建
// ============================================================

func (s *patroniScraper) buildResource(meta patroniNodeMeta) pcommon.Resource {
	res := pcommon.NewResource()
	attrs := res.Attributes()
	attrs.PutStr("service.name", "patroni-"+meta.name)
	attrs.PutStr("patroni.scope", meta.scope)
	attrs.PutStr("patroni.node.name", meta.name)
	return res
}

// ============================================================
// bool → int64 辅助（ptr / plain 两个变体）
// ============================================================

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func ptrBoolToInt64(b *bool) int64 {
	if b != nil && *b {
		return 1
	}
	return 0
}
