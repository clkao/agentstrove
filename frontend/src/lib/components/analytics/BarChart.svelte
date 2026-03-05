<!-- ABOUTME: Reusable horizontal bar chart with stacked colored segments. -->
<!-- ABOUTME: Pure SVG rendering for session/message counts by developer. -->
<script lang="ts">
  interface Segment {
    value: number;
    color: string;
    label: string;
  }

  interface Bar {
    label: string;
    segments: Segment[];
    total: number;
  }

  let { data = [], maxValue = 0 }: { data: Bar[]; maxValue?: number } = $props();

  const barHeight = 28;
  const labelWidth = 120;
  const gap = 4;

  let containerWidth = $state(400);

  const computedMax = $derived(maxValue > 0 ? maxValue : Math.max(...data.map(d => d.total), 1));
  const chartWidth = $derived(Math.max(containerWidth - labelWidth - 60, 100));
</script>

{#if data.length === 0}
  <div class="empty">No data</div>
{:else}
  <div class="bar-chart" bind:clientWidth={containerWidth}>
    <svg width="100%" height={data.length * (barHeight + gap) + gap}>
      {#each data as bar, i}
        <text
          x={labelWidth - 8}
          y={i * (barHeight + gap) + gap + barHeight / 2}
          dy="0.35em"
          text-anchor="end"
          class="label"
          font-size="11"
          fill="var(--text-secondary)"
        >
          {bar.label.length > 14 ? bar.label.slice(0, 13) + '\u2026' : bar.label}
        </text>
        {#each bar.segments as seg, j}
          {@const offset = bar.segments.slice(0, j).reduce((sum, s) => sum + s.value, 0)}
          <rect
            x={labelWidth + (offset / computedMax) * chartWidth}
            y={i * (barHeight + gap) + gap}
            width={Math.max((seg.value / computedMax) * chartWidth, seg.value > 0 ? 2 : 0)}
            height={barHeight}
            fill={seg.color}
            rx="2"
          >
            <title>{seg.label}: {seg.value}</title>
          </rect>
        {/each}
        <text
          x={labelWidth + (bar.total / computedMax) * chartWidth + 6}
          y={i * (barHeight + gap) + gap + barHeight / 2}
          dy="0.35em"
          font-size="11"
          fill="var(--text-muted)"
        >
          {bar.total}
        </text>
      {/each}
    </svg>
  </div>
{/if}

<style>
  .bar-chart {
    width: 100%;
  }

  .empty {
    padding: 20px;
    text-align: center;
    color: var(--text-muted);
    font-size: 12px;
  }
</style>
