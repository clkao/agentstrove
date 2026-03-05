<!-- ABOUTME: Tool usage distribution shown as ranked horizontal bars. -->
<!-- ABOUTME: Displays top tools by usage count with category color coding. -->
<script lang="ts">
  import { analytics } from "../../stores/analytics.svelte.js";
  import BarChart from "./BarChart.svelte";

  const toolColors: Record<string, string> = {
    Read: "#3b82f6",     // blue
    Edit: "#f59e0b",     // amber
    Write: "#10b981",    // green
    Bash: "#ef4444",     // red
    Grep: "#8b5cf6",     // purple
    Glob: "#06b6d4",     // cyan
    Task: "#ec4899",     // pink
  };
  const defaultColor = "#6b7280"; // gray

  function getToolColor(name: string): string {
    return toolColors[name] || defaultColor;
  }

  const toolBars = $derived(
    analytics.toolUsage.map(t => ({
      label: t.tool_name,
      segments: [{ value: t.usage_count, color: getToolColor(t.tool_name), label: t.tool_name }],
      total: t.usage_count,
    }))
  );

  const legendTools = $derived(
    analytics.toolUsage
      .slice(0, 10)
      .map(t => ({ name: t.tool_name, color: getToolColor(t.tool_name) }))
      .filter((item, i, arr) => arr.findIndex(x => x.name === item.name) === i)
  );
</script>

<div class="tool-usage card">
  <h2>Tool Usage</h2>
  {#if analytics.toolUsage.length === 0}
    <div class="empty">No tool usage data</div>
  {:else}
    <BarChart data={toolBars} />
    {#if legendTools.length > 1}
      <div class="legend">
        {#each legendTools as tool}
          <span class="legend-item">
            <span class="swatch" style="background: {tool.color}"></span>
            {tool.name}
          </span>
        {/each}
      </div>
    {/if}
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

  .empty {
    padding: 20px;
    text-align: center;
    color: var(--text-muted);
    font-size: 12px;
  }

  .legend {
    display: flex;
    gap: 12px;
    margin-top: 8px;
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
</style>
