<!-- ABOUTME: Session and message count bar charts grouped by developer. -->
<!-- ABOUTME: Stacked bars color-coded by agent type with a legend. -->
<script lang="ts">
  import { analytics } from "../../stores/analytics.svelte.js";
  import BarChart from "./BarChart.svelte";

  const agentColors: Record<string, string> = {};
  const palette = [
    "var(--accent-blue)",
    "var(--accent-green)",
    "var(--accent-purple)",
    "var(--accent-amber)",
    "var(--accent-rose)",
    "var(--accent-coral)",
    "var(--accent-teal)",
    "var(--accent-black)",
  ];

  function getAgentColor(agent: string): string {
    if (!agentColors[agent]) {
      agentColors[agent] = palette[Object.keys(agentColors).length % palette.length];
    }
    return agentColors[agent];
  }

  interface BarData {
    label: string;
    segments: { value: number; color: string; label: string }[];
    total: number;
  }

  const sessionBars = $derived.by(() => {
    const byUser = new Map<string, { name: string; agents: Map<string, number> }>();
    for (const u of analytics.usage) {
      if (!byUser.has(u.user_id)) {
        byUser.set(u.user_id, { name: u.user_name || u.user_id, agents: new Map() });
      }
      const entry = byUser.get(u.user_id)!;
      entry.agents.set(u.agent_type, (entry.agents.get(u.agent_type) || 0) + u.session_count);
    }

    const allAgents = [...new Set(analytics.usage.map(u => u.agent_type))].sort();
    allAgents.forEach(a => getAgentColor(a));

    const bars: BarData[] = [];
    for (const [, user] of byUser) {
      const segments = allAgents.map(agent => ({
        value: user.agents.get(agent) || 0,
        color: getAgentColor(agent),
        label: agent || "(unknown)",
      }));
      bars.push({
        label: user.name,
        segments,
        total: segments.reduce((s, seg) => s + seg.value, 0),
      });
    }
    return bars.sort((a, b) => b.total - a.total);
  });

  const messageBars = $derived.by(() => {
    const byUser = new Map<string, { name: string; agents: Map<string, number> }>();
    for (const u of analytics.usage) {
      if (!byUser.has(u.user_id)) {
        byUser.set(u.user_id, { name: u.user_name || u.user_id, agents: new Map() });
      }
      const entry = byUser.get(u.user_id)!;
      entry.agents.set(u.agent_type, (entry.agents.get(u.agent_type) || 0) + u.message_count);
    }

    const allAgents = [...new Set(analytics.usage.map(u => u.agent_type))].sort();

    const bars: BarData[] = [];
    for (const [, user] of byUser) {
      const segments = allAgents.map(agent => ({
        value: user.agents.get(agent) || 0,
        color: getAgentColor(agent),
        label: agent || "(unknown)",
      }));
      bars.push({
        label: user.name,
        segments,
        total: segments.reduce((s, seg) => s + seg.value, 0),
      });
    }
    return bars.sort((a, b) => b.total - a.total);
  });

  const agentLegend = $derived([...new Set(analytics.usage.map(u => u.agent_type))].sort());
</script>

<div class="usage-overview card">
  <h2>Usage Overview</h2>
  {#if analytics.usage.length === 0}
    <div class="empty">No usage data for this period</div>
  {:else}
    <div class="charts">
      <div class="chart-section">
        <h3>Sessions</h3>
        <BarChart data={sessionBars} />
      </div>
      <div class="chart-section">
        <h3>Messages</h3>
        <BarChart data={messageBars} />
      </div>
    </div>
    <div class="legend">
      {#each agentLegend as agent}
        <span class="legend-item">
          <span class="swatch" style="background: {getAgentColor(agent)}"></span>
          {agent || "(unknown)"}
        </span>
      {/each}
    </div>
  {/if}
</div>

<style>
  .card {
    background: var(--bg-surface);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-lg);
    padding: 16px;
  }

  h2 {
    font-size: 13px;
    font-weight: 650;
    color: var(--text-primary);
    margin-bottom: 12px;
  }

  h3 {
    font-size: 11px;
    font-weight: 600;
    color: var(--text-secondary);
    margin-bottom: 8px;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .charts {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 20px;
  }

  .empty {
    padding: 20px;
    text-align: center;
    color: var(--text-muted);
    font-size: 12px;
  }

  .legend {
    display: flex;
    gap: 12px;
    margin-top: 12px;
    flex-wrap: wrap;
  }

  .legend-item {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 11px;
    color: var(--text-secondary);
  }

  .swatch {
    width: 10px;
    height: 10px;
    border-radius: 2px;
    flex-shrink: 0;
  }

  @media (max-width: 700px) {
    .charts {
      grid-template-columns: 1fr;
    }
  }
</style>
