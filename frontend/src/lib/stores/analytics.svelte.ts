// ABOUTME: Analytics data state management for the team dashboard.
// ABOUTME: Loads usage, heatmap, tool, daily activity, and token-by-model data in parallel.

import type { UserUsage, HeatmapCell, ToolUsageStat, DailyActivity, ModelTokenUsage } from "../api/types.js";
import { fetchUsageOverview, fetchActivityHeatmap, fetchToolUsage, fetchDailyActivity, fetchTokensByModel } from "../api/client.js";

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
  daily = $state<DailyActivity[]>([]);
  modelTokens = $state<ModelTokenUsage[]>([]);
  loading = $state(false);
  dateFrom = $state(defaultDateFrom());
  dateTo = $state(today());
  projectName = $state("");

  async load(): Promise<void> {
    this.loading = true;
    try {
      const pn = this.projectName || undefined;
      const [usage, heatmap, tools, daily, modelTokens] = await Promise.all([
        fetchUsageOverview(this.dateFrom, this.dateTo, pn),
        fetchActivityHeatmap(this.dateFrom, this.dateTo, pn),
        fetchToolUsage(this.dateFrom, this.dateTo, pn),
        fetchDailyActivity(this.dateFrom, this.dateTo, pn),
        fetchTokensByModel(this.dateFrom, this.dateTo, pn),
      ]);
      this.usage = usage;
      this.heatmap = heatmap;
      this.toolUsage = tools;
      this.daily = daily;
      this.modelTokens = modelTokens;
    } finally {
      this.loading = false;
    }
  }
}

export const analytics = new AnalyticsStore();
