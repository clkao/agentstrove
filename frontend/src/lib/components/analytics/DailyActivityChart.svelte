<!-- ABOUTME: Vertical bar chart showing daily session counts over time. -->
<!-- ABOUTME: SVG bars with hover tooltips, sparse date labels, responsive width. -->
<script lang="ts">
  import { analytics } from "../../stores/analytics.svelte.js";

  let containerWidth = $state(400);

  const padding = { top: 8, right: 16, bottom: 24, left: 32 };
  const chartHeight = 120;

  const data = $derived(analytics.daily);
  const maxCount = $derived(Math.max(...data.map(d => d.session_count), 1));

  const chartWidth = $derived(Math.max(containerWidth - padding.left - padding.right, 100));
  const barWidth = $derived(data.length > 0 ? Math.max(chartWidth / data.length - 1, 2) : 0);
  const barGap = $derived(data.length > 0 ? Math.max((chartWidth - barWidth * data.length) / data.length, 1) : 0);

  // Show roughly 6-8 date labels, spaced evenly
  const labelStep = $derived(Math.max(1, Math.ceil(data.length / 7)));

  function formatDate(dateStr: string): string {
    const parts = dateStr.split("-");
    return `${parts[1]}/${parts[2]}`;
  }
</script>

<div class="daily-chart card">
  <h2>Daily Activity</h2>
  {#if data.length === 0}
    <div class="empty">No daily activity data</div>
  {:else}
    <div class="chart-wrapper" bind:clientWidth={containerWidth}>
      <svg width="100%" height={chartHeight + padding.top + padding.bottom}>
        <!-- Y-axis max label -->
        <text
          x={padding.left - 4}
          y={padding.top + 4}
          text-anchor="end"
          font-size="9"
          fill="var(--text-muted)"
        >
          {maxCount}
        </text>

        <!-- Bars -->
        {#each data as day, i}
          {@const barH = (day.session_count / maxCount) * chartHeight}
          {@const x = padding.left + i * (barWidth + barGap)}
          {@const y = padding.top + chartHeight - barH}
          <rect
            {x}
            {y}
            width={barWidth}
            height={barH}
            fill="var(--accent-blue)"
            opacity="0.8"
            rx="1"
          >
            <title>{day.date}: {day.session_count} sessions, {day.message_count} messages</title>
          </rect>
        {/each}

        <!-- X-axis date labels -->
        {#each data as day, i}
          {#if i % labelStep === 0}
            <text
              x={padding.left + i * (barWidth + barGap) + barWidth / 2}
              y={padding.top + chartHeight + 14}
              text-anchor="middle"
              font-size="9"
              fill="var(--text-muted)"
            >
              {formatDate(day.date)}
            </text>
          {/if}
        {/each}
      </svg>
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

  .chart-wrapper {
    width: 100%;
  }

  .empty {
    padding: 20px;
    text-align: center;
    color: var(--text-muted);
    font-size: 12px;
  }
</style>
