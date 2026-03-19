<!-- ABOUTME: Token usage breakdown by model shown as horizontal bars. -->
<!-- ABOUTME: Displays output tokens, context tokens, and message count per model. -->
<script lang="ts">
  import { analytics } from "../../stores/analytics.svelte.js";
  import { formatTokenCount, formatModelName } from "../../utils/format.js";
  import BarChart from "./BarChart.svelte";

  const bars = $derived(
    analytics.modelTokens.map(t => ({
      label: formatModelName(t.model),
      segments: [
        { value: t.output_tokens, color: "var(--accent-purple)", label: "output" },
        { value: t.context_tokens, color: "var(--accent-teal)", label: "context" },
      ],
      total: t.output_tokens + t.context_tokens,
    }))
  );
</script>

<div class="model-tokens card">
  <h2>Token Usage by Model</h2>
  {#if analytics.modelTokens.length === 0}
    <div class="empty">No token usage data</div>
  {:else}
    <BarChart data={bars} formatValue={formatTokenCount} />
    <div class="legend">
      <span class="legend-item">
        <span class="swatch" style="background: var(--accent-purple)"></span>
        Output
      </span>
      <span class="legend-item">
        <span class="swatch" style="background: var(--accent-teal)"></span>
        Context
      </span>
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
