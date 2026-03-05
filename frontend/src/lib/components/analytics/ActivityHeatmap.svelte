<!-- ABOUTME: Activity heatmap showing session frequency by day-of-week and hour. -->
<!-- ABOUTME: CSS grid with color-mix intensity scaling, labeled as UTC. -->
<script lang="ts">
  import { analytics } from "../../stores/analytics.svelte.js";

  const dayLabels = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];

  // Build full 7x24 grid with zero-filled defaults
  const grid = $derived.by(() => {
    const cells: { dow: number; hour: number; count: number }[] = [];
    const lookup = new Map<string, number>();
    for (const cell of analytics.heatmap) {
      lookup.set(`${cell.day_of_week}-${cell.hour}`, cell.session_count);
    }
    for (let dow = 1; dow <= 7; dow++) {
      for (let hour = 0; hour < 24; hour++) {
        cells.push({
          dow,
          hour,
          count: lookup.get(`${dow}-${hour}`) || 0,
        });
      }
    }
    return cells;
  });

  const maxCount = $derived(Math.max(...grid.map(c => c.count), 1));
</script>

<div class="heatmap-card card">
  <h2>Activity Heatmap <span class="tz">(UTC)</span></h2>
  <div class="heatmap-wrapper">
    <div class="hour-labels">
      {#each Array(24) as _, h}
        <span class="hour-label">{h}</span>
      {/each}
    </div>
    <div class="grid-container">
      <div class="day-labels">
        {#each dayLabels as day}
          <span class="day-label">{day}</span>
        {/each}
      </div>
      <div class="heatmap-grid">
        {#each grid as cell}
          <div
            class="cell"
            style="--intensity: {cell.count / maxCount}"
            title="{dayLabels[cell.dow - 1]} {cell.hour}:00 — {cell.count} sessions"
          ></div>
        {/each}
      </div>
    </div>
  </div>
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

  .tz {
    font-weight: 400;
    color: var(--text-muted);
    font-size: 11px;
  }

  .heatmap-wrapper {
    display: flex;
    gap: 4px;
  }

  .hour-labels {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding-top: 18px;
  }

  .hour-label {
    height: 14px;
    display: flex;
    align-items: center;
    justify-content: flex-end;
    font-size: 9px;
    color: var(--text-muted);
    width: 16px;
  }

  .grid-container {
    flex: 1;
    min-width: 0;
  }

  .day-labels {
    display: grid;
    grid-template-columns: repeat(7, 1fr);
    gap: 2px;
    margin-bottom: 2px;
  }

  .day-label {
    text-align: center;
    font-size: 10px;
    color: var(--text-muted);
    font-weight: 500;
  }

  .heatmap-grid {
    display: grid;
    grid-template-columns: repeat(7, 1fr);
    grid-template-rows: repeat(24, 14px);
    gap: 2px;
    grid-auto-flow: column;
  }

  .cell {
    border-radius: 2px;
    background: color-mix(in oklch, var(--accent-green) calc(var(--intensity) * 100%), var(--bg-inset));
    min-width: 0;
  }
</style>
