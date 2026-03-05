<!-- ABOUTME: Date range picker with quick preset buttons for analytics filtering. -->
<!-- ABOUTME: Updates the analytics store date range and triggers data reload on change. -->
<script lang="ts">
  import { analytics } from "../../stores/analytics.svelte.js";

  function setRange(days: number | null) {
    if (days === null) {
      analytics.dateFrom = "";
      analytics.dateTo = "";
    } else {
      const to = new Date();
      const from = new Date();
      from.setDate(from.getDate() - days);
      analytics.dateFrom = from.toISOString().slice(0, 10);
      analytics.dateTo = to.toISOString().slice(0, 10);
    }
    analytics.load();
  }

  function onDateChange() {
    analytics.load();
  }
</script>

<div class="time-range">
  <div class="presets">
    <button onclick={() => setRange(7)}>7d</button>
    <button onclick={() => setRange(30)}>30d</button>
    <button onclick={() => setRange(90)}>90d</button>
    <button onclick={() => setRange(null)}>All</button>
  </div>
  <div class="dates">
    <input type="date" bind:value={analytics.dateFrom} onchange={onDateChange} />
    <span class="separator">&ndash;</span>
    <input type="date" bind:value={analytics.dateTo} onchange={onDateChange} />
  </div>
</div>

<style>
  .time-range {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .presets {
    display: flex;
    gap: 2px;
  }

  .presets button {
    padding: 3px 8px;
    font-size: 11px;
    font-weight: 500;
    color: var(--text-secondary);
    border-radius: var(--radius-sm);
    background: var(--bg-inset);
    border: 1px solid var(--border-muted);
    transition: background 0.12s, color 0.12s;
  }

  .presets button:hover {
    background: var(--bg-surface-hover);
    color: var(--text-primary);
  }

  .dates {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .separator {
    color: var(--text-muted);
    font-size: 12px;
  }

  input[type="date"] {
    height: 26px;
    padding: 0 6px;
    background: var(--bg-inset);
    border: 1px solid var(--border-muted);
    border-radius: var(--radius-sm);
    font-size: 11px;
    color: var(--text-primary);
  }
</style>
