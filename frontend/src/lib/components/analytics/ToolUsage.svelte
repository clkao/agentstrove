<!-- ABOUTME: Tool usage distribution shown as ranked horizontal bars. -->
<!-- ABOUTME: Displays top tools by usage count with category color coding. -->
<script lang="ts">
  import { analytics } from "../../stores/analytics.svelte.js";
  import BarChart from "./BarChart.svelte";

  const categoryColors: Record<string, string> = {
    bash: "var(--accent-amber)",
    file: "var(--accent-blue)",
    search: "var(--accent-green)",
    mcp: "var(--accent-purple)",
    "": "var(--accent-teal)",
  };

  function getCategoryColor(cat: string): string {
    return categoryColors[cat] || "var(--accent-teal)";
  }

  const toolBars = $derived(
    analytics.toolUsage.map(t => ({
      label: t.tool_name,
      segments: [{ value: t.usage_count, color: getCategoryColor(t.category), label: t.category || t.tool_name }],
      total: t.usage_count,
    }))
  );

  const categories = $derived(
    [...new Set(analytics.toolUsage.map(t => t.category))].filter(Boolean).sort()
  );
</script>

<div class="tool-usage card">
  <h2>Tool Usage</h2>
  {#if analytics.toolUsage.length === 0}
    <div class="empty">No tool usage data</div>
  {:else}
    <BarChart data={toolBars} />
    {#if categories.length > 1}
      <div class="legend">
        {#each categories as cat}
          <span class="legend-item">
            <span class="swatch" style="background: {getCategoryColor(cat)}"></span>
            {cat}
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
