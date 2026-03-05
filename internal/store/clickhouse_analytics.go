// ABOUTME: Analytics query methods for team-level usage insights.
// ABOUTME: Implements UsageByUser, ActivityHeatmap, and ToolUsageDistribution on ClickHouseStore.

package store

import (
	"context"
	"fmt"
)

// userUsageRow is the scan target for UsageByUser queries.
type userUsageRow struct {
	UserID        string `ch:"user_id"`
	UserName      string `ch:"user_name"`
	AgentType     string `ch:"agent_type"`
	ProjectName   string `ch:"project_name"`
	SessionCount  uint64 `ch:"session_count"`
	TotalMessages uint64 `ch:"total_messages"`
}

// UsageByUser returns per-user/agent/project session and message counts.
func (s *ClickHouseStore) UsageByUser(ctx context.Context, orgID string, dateFrom, dateTo string) ([]UserUsage, error) {
	var conditions []string
	var args []interface{}

	conditions = append(conditions, "s.org_id = ?")
	args = append(args, orgID)
	conditions = append(conditions, "s.parent_session_id = ''")
	conditions = append(conditions, "s.user_message_count > 0")

	if dateFrom != "" {
		conditions = append(conditions, "s.started_at >= ?")
		args = append(args, dateFrom)
	}
	if dateTo != "" {
		conditions = append(conditions, "s.started_at < toDate(?) + 1")
		args = append(args, dateTo)
	}

	q := fmt.Sprintf(`SELECT s.user_id, s.user_name, s.agent_type, s.project_name,
		count() AS session_count,
		sum(s.message_count) AS total_messages
		FROM sessions AS s FINAL
		%s
		GROUP BY s.user_id, s.user_name, s.agent_type, s.project_name
		ORDER BY session_count DESC`, chWhereClause(conditions))

	var rows []userUsageRow
	if err := s.conn.Select(ctx, &rows, q, args...); err != nil {
		return nil, fmt.Errorf("usage by user: %w", err)
	}

	results := make([]UserUsage, 0, len(rows))
	for _, r := range rows {
		results = append(results, UserUsage{
			UserID:       r.UserID,
			UserName:     r.UserName,
			AgentType:    r.AgentType,
			ProjectName:  r.ProjectName,
			SessionCount: int(r.SessionCount),
			MessageCount: int(r.TotalMessages),
		})
	}
	return results, nil
}

// heatmapRow is the scan target for ActivityHeatmap queries.
type heatmapRow struct {
	DayOfWeek    uint8  `ch:"dow"`
	Hour         uint8  `ch:"hour"`
	SessionCount uint64 `ch:"session_count"`
}

// ActivityHeatmap returns session counts grouped by day-of-week and hour.
func (s *ClickHouseStore) ActivityHeatmap(ctx context.Context, orgID string, dateFrom, dateTo string) ([]HeatmapCell, error) {
	var conditions []string
	var args []interface{}

	conditions = append(conditions, "s.org_id = ?")
	args = append(args, orgID)
	conditions = append(conditions, "s.parent_session_id = ''")
	conditions = append(conditions, "s.user_message_count > 0")

	if dateFrom != "" {
		conditions = append(conditions, "s.started_at >= ?")
		args = append(args, dateFrom)
	}
	if dateTo != "" {
		conditions = append(conditions, "s.started_at < toDate(?) + 1")
		args = append(args, dateTo)
	}

	q := fmt.Sprintf(`SELECT toDayOfWeek(s.started_at) AS dow,
		toHour(s.started_at) AS hour,
		count() AS session_count
		FROM sessions AS s FINAL
		%s
		GROUP BY dow, hour
		ORDER BY dow, hour`, chWhereClause(conditions))

	var rows []heatmapRow
	if err := s.conn.Select(ctx, &rows, q, args...); err != nil {
		return nil, fmt.Errorf("activity heatmap: %w", err)
	}

	cells := make([]HeatmapCell, 0, len(rows))
	for _, r := range rows {
		cells = append(cells, HeatmapCell{
			DayOfWeek:    int(r.DayOfWeek),
			Hour:         int(r.Hour),
			SessionCount: int(r.SessionCount),
		})
	}
	return cells, nil
}

// toolUsageRow is the scan target for ToolUsageDistribution queries.
type toolUsageRow struct {
	ToolName   string `ch:"tool_name"`
	Category   string `ch:"tool_category"`
	UsageCount uint64 `ch:"usage_count"`
}

// ToolUsageDistribution returns the top 20 tool name/category pairs by usage count.
func (s *ClickHouseStore) ToolUsageDistribution(ctx context.Context, orgID string, dateFrom, dateTo string) ([]ToolUsageStat, error) {
	var conditions []string
	var args []interface{}

	conditions = append(conditions, "tc.org_id = ?")
	args = append(args, orgID)
	conditions = append(conditions, "s.parent_session_id = ''")
	conditions = append(conditions, "s.user_message_count > 0")

	if dateFrom != "" {
		conditions = append(conditions, "s.started_at >= ?")
		args = append(args, dateFrom)
	}
	if dateTo != "" {
		conditions = append(conditions, "s.started_at < toDate(?) + 1")
		args = append(args, dateTo)
	}

	q := fmt.Sprintf(`SELECT tc.tool_name, tc.tool_category, count() AS usage_count
		FROM tool_calls AS tc FINAL
		JOIN sessions AS s FINAL ON s.id = tc.session_id AND s.org_id = tc.org_id
		%s
		GROUP BY tc.tool_name, tc.tool_category
		ORDER BY usage_count DESC
		LIMIT 20`, chWhereClause(conditions))

	var rows []toolUsageRow
	if err := s.conn.Select(ctx, &rows, q, args...); err != nil {
		return nil, fmt.Errorf("tool usage distribution: %w", err)
	}

	stats := make([]ToolUsageStat, 0, len(rows))
	for _, r := range rows {
		stats = append(stats, ToolUsageStat{
			ToolName:   r.ToolName,
			Category:   r.Category,
			UsageCount: int(r.UsageCount),
		})
	}
	return stats, nil
}
