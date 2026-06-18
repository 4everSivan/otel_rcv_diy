package patronireceiver

import (
	"encoding/json"
	"fmt"
	"time"
)

// ============================================================
// GET /patroni 响应结构
// ============================================================

type patroniResponse struct {
	State               string               `json:"state"`
	PostmasterStartTime string               `json:"postmaster_start_time"`
	Role                string               `json:"role"`
	ServerVersion       int                  `json:"server_version"`
	Xlog                patroniXlog          `json:"xlog"`
	Timeline            int64                `json:"timeline"`
	Replication         []patroniReplication `json:"replication"`
	ClusterUnlocked     *bool                `json:"cluster_unlocked"`
	FailsafeModeActive  *bool                `json:"failsafe_mode_is_active"`
	Pause               *bool                `json:"pause"`
	DCSSLastSeen        int64                `json:"dcs_last_seen"`
	Patroni             patroniInfo          `json:"patroni"`
	PendingRestart      *bool                `json:"pending_restart"`
}

type patroniXlog struct {
	Location          int64  `json:"location"`
	ReceivedLocation  int64  `json:"received_location"`
	ReplayedLocation  int64  `json:"replayed_location"`
	ReplayedTimestamp *int64 `json:"replayed_timestamp"`
	Paused            *bool  `json:"paused"`
}

type patroniReplication struct {
	ApplicationName string `json:"application_name"`
	ClientAddr      string `json:"client_addr"`
	State           string `json:"state"`
	SyncState       string `json:"sync_state"`
	SyncPriority    int    `json:"sync_priority"`
}

type patroniInfo struct {
	Version string `json:"version"`
	Scope   string `json:"scope"`
	Name    string `json:"name"`
}

// ============================================================
// GET /cluster 响应结构
// ============================================================

type clusterResponse struct {
	Members []clusterMember `json:"members"`
	Scope   string          `json:"scope"`
}

type clusterMember struct {
	Name       string `json:"name"`
	Role       string `json:"role"`
	State      string `json:"state"`
	Timeline   int64  `json:"timeline"`
	ReceiveLag *int64 `json:"receive_lag"`
	ReplayLag  *int64 `json:"replay_lag"`
	Lag        *int64 `json:"lag"`
}

// ============================================================
// GET /history 响应结构（[[timeline, lsn, reason, promoted_at], ...]）
// ============================================================

type historyEntry struct {
	Timeline   int64
	LSN        int64
	Reason     string
	PromotedAt time.Time
}

func (h *historyEntry) UnmarshalJSON(data []byte) error {
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		return fmt.Errorf("history entry: %w", err)
	}
	if len(arr) < 4 {
		return fmt.Errorf("history entry: expected 4 elements, got %d", len(arr))
	}
	if err := json.Unmarshal(arr[0], &h.Timeline); err != nil {
		return fmt.Errorf("history entry[0] timeline: %w", err)
	}
	if err := json.Unmarshal(arr[1], &h.LSN); err != nil {
		return fmt.Errorf("history entry[1] lsn: %w", err)
	}
	if err := json.Unmarshal(arr[2], &h.Reason); err != nil {
		return fmt.Errorf("history entry[2] reason: %w", err)
	}

	var promotedAt string
	if err := json.Unmarshal(arr[3], &promotedAt); err != nil {
		return fmt.Errorf("history entry[3] promoted_at: %w", err)
	}
	t, err := time.Parse(time.RFC3339, promotedAt)
	if err != nil {
		return fmt.Errorf("history entry[3] parse time %q: %w", promotedAt, err)
	}
	h.PromotedAt = t
	return nil
}

// ============================================================
// patroniVersionToInt 将 semver 字符串转为 mdatagen 要求的 int 格式
// ============================================================

func patroniVersionToInt(version string) int64 {
	v := version
	// 去掉 v 前缀
	if len(v) > 0 && v[0] == 'v' {
		v = v[1:]
	}
	// 解析 major.minor.patch
	var major, minor, patch int64
	_, err := fmt.Sscanf(v, "%d.%d.%d", &major, &minor, &patch)
	if err != nil {
		return 0
	}
	return major*10000 + minor*100 + patch
}

// ============================================================
// postgresStateToInt 将文本状态映射为 Patroni 定义的数值（0–14）
// ============================================================

var postgresStateMap = map[string]int64{
	"initdb":                 0,
	"initdb_failed":          1,
	"custom_bootstrap":       2,
	"custom_bootstrap_failed": 3,
	"creating_replica":       4,
	"running":                5,
	"starting":               6,
	"bootstrap_starting":     7,
	"start_failed":           8,
	"restarting":             9,
	"restart_failed":         10,
	"stopping":               11,
	"stopped":                12,
	"stop_failed":            13,
	"crashed":                14,
}

func postgresStateToInt(state string) int64 {
	if v, ok := postgresStateMap[state]; ok {
		return v
	}
	return -1 // unknown
}

// ============================================================
// 时间解析辅助
// ============================================================

// postgres time format: "2024-08-28 19:39:26.352526+00:00"
const postgresTimeLayout = "2006-01-02 15:04:05.999999999-07:00"

func parsePostgresTimestamp(s string) (time.Time, error) {
	// 尝试带时区后缀
	t, err := time.Parse(postgresTimeLayout, s)
	if err != nil {
		// 尝试不带时区后缀（UTC）
		t, err = time.Parse("2006-01-02 15:04:05.999999999", s)
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("parse postgres timestamp %q: %w", s, err)
	}
	return t, nil
}

// ============================================================
// 指标元信息（用于 ScrapeMetrics 返回值附带）
// ============================================================

// patroniNodeMeta 从 patroniResponse 中提取的节点标识信息
type patroniNodeMeta struct {
	scope   string
	name    string
	role    string
	state   string
	version string
}

func extractNodeMeta(p patroniResponse) patroniNodeMeta {
	return patroniNodeMeta{
		scope:   p.Patroni.Scope,
		name:    p.Patroni.Name,
		role:    p.Role,
		state:   p.State,
		version: p.Patroni.Version,
	}
}
