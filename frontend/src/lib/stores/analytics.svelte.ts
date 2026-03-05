// ABOUTME: Analytics data state management for the team dashboard.
// ABOUTME: Loads usage, heatmap, and tool data in parallel from 3 API endpoints.

import type { UserUsage, HeatmapCell, ToolUsageStat } from "../api/types.js";
import { fetchUsageOverview, fetchActivityHeatmap, fetchToolUsage } from "../api/client.js";

function defaultDateFrom(): string {
  const d = new Date();
  d.setDate(d.getDate() - 30);
  return d.toISOString().slice(0, 10);
}

function today(): string {
  return new Date().toISOString().slice(0, 10);
}

class AnalyticsStore {
  usage = $state<UserUsage[]>([]);
  heatmap = $state<HeatmapCell[]>([]);
  toolUsage = $state<ToolUsageStat[]>([]);
  loading = $state(false);
  dateFrom = $state(defaultDateFrom());
  dateTo = $state(today());

  async load(): Promise<void> {
    this.loading = true;
    try {
      const [usage, heatmap, tools] = await Promise.all([
        fetchUsageOverview(this.dateFrom, this.dateTo),
        fetchActivityHeatmap(this.dateFrom, this.dateTo),
        fetchToolUsage(this.dateFrom, this.dateTo),
      ]);
      this.usage = usage;
      this.heatmap = heatmap;
      this.toolUsage = tools;
    } finally {
      this.loading = false;
    }
  }
}

export const analytics = new AnalyticsStore();
