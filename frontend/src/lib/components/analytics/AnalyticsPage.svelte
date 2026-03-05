<!-- ABOUTME: Dashboard page assembling all analytics chart components. -->
<!-- ABOUTME: Full-width layout with time range controls, usage overview, heatmap, and tool usage. -->
<script lang="ts">
  import { analytics } from "../../stores/analytics.svelte.js";
  import TimeRangeControl from "./TimeRangeControl.svelte";
  import UsageOverview from "./UsageOverview.svelte";
  import ActivityHeatmap from "./ActivityHeatmap.svelte";
  import ToolUsage from "./ToolUsage.svelte";
</script>

<div class="analytics-page">
  <div class="analytics-header">
    <h1>Team Analytics</h1>
    <TimeRangeControl />
  </div>

  {#if analytics.loading}
    <div class="loading">Loading analytics...</div>
  {:else}
    <div class="analytics-content">
      <UsageOverview />
      <div class="analytics-bottom">
        <ActivityHeatmap />
        <ToolUsage />
      </div>
    </div>
  {/if}
</div>

<style>
  .analytics-page {
    flex: 1;
    overflow-y: auto;
    padding: 20px 24px;
  }

  .analytics-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 20px;
  }

  h1 {
    font-size: 16px;
    font-weight: 650;
    color: var(--text-primary);
    letter-spacing: -0.01em;
  }

  .loading {
    padding: 40px;
    text-align: center;
    color: var(--text-muted);
    font-size: 13px;
  }

  .analytics-content {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .analytics-bottom {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 20px;
  }

  @media (max-width: 900px) {
    .analytics-bottom {
      grid-template-columns: 1fr;
    }
  }
</style>
