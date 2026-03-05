<!-- ABOUTME: Renders a collapsible tool call block with metadata tags and content. -->
<!-- ABOUTME: Shows tool name and preview when collapsed; full input/output when expanded. -->
<script lang="ts">
  import type { ToolCall } from "../../api/types.js";
  import {
    extractToolParamMeta,
    generateFallbackContent,
  } from "../../utils/tool-params.js";

  interface Props {
    content: string;
    label?: string;
    toolCall?: ToolCall;
  }

  let { content, label, toolCall }: Props = $props();
  let collapsed: boolean = $state(true);

  let previewLine = $derived(
    content.split("\n")[0]?.slice(0, 100) ?? "",
  );

  let inputParams = $derived.by(() => {
    if (!toolCall?.input_json) return null;
    try {
      return JSON.parse(toolCall.input_json);
    } catch {
      return null;
    }
  });

  let toolParamMeta = $derived.by(() => {
    if (!inputParams || !toolCall) return null;
    return extractToolParamMeta(toolCall.tool_name, inputParams);
  });

  let fallbackContent = $derived.by(() => {
    if (content || !inputParams || !toolCall) return null;
    return generateFallbackContent(
      toolCall.tool_name,
      inputParams,
    );
  });
</script>

<div class="tool-block">
  <button
    class="tool-header"
    onclick={() => {
      const sel = window.getSelection();
      if (sel && sel.toString().length > 0) return;
      collapsed = !collapsed;
    }}
  >
    <span class="tool-chevron" class:open={!collapsed}>
      &#9656;
    </span>
    {#if label}
      <span class="tool-label">{label}</span>
    {/if}
    {#if collapsed && previewLine}
      <span class="tool-preview">{previewLine}</span>
    {/if}
  </button>
  {#if !collapsed}
    {#if toolParamMeta}
      <div class="tool-meta">
        {#each toolParamMeta as { label: metaLabel, value }}
          <span class="meta-tag">
            <span class="meta-label">{metaLabel}:</span>
            {value}
          </span>
        {/each}
      </div>
    {/if}
    {#if content}
      <pre class="tool-content">{content}</pre>
    {:else if fallbackContent}
      <pre class="tool-content">{fallbackContent}</pre>
    {/if}
  {/if}
</div>

<style>
  .tool-block {
    border-left: 2px solid var(--accent-amber);
    background: var(--tool-bg);
    border-radius: 0 var(--radius-sm) var(--radius-sm) 0;
    margin: 0;
  }

  .tool-header {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 6px 10px;
    width: 100%;
    text-align: left;
    font-size: 12px;
    color: var(--text-secondary);
    min-width: 0;
    border-radius: 0 var(--radius-sm) var(--radius-sm) 0;
    transition: background 0.1s;
    user-select: text;
  }

  .tool-header:hover {
    background: var(--bg-surface-hover);
    color: var(--text-primary);
  }

  .tool-chevron {
    display: inline-block;
    font-size: 10px;
    transition: transform 0.15s;
    flex-shrink: 0;
    color: var(--text-muted);
  }

  .tool-chevron.open {
    transform: rotate(90deg);
  }

  .tool-label {
    font-family: var(--font-mono);
    font-weight: 500;
    font-size: 11px;
    color: var(--accent-amber);
    white-space: nowrap;
    flex-shrink: 0;
  }

  .tool-preview {
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }

  .tool-meta {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    padding: 6px 14px;
    border-top: 1px solid var(--border-muted);
  }

  .meta-tag {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--text-muted);
    background: var(--bg-inset);
    padding: 2px 6px;
    border-radius: var(--radius-sm);
  }

  .meta-label {
    color: var(--text-secondary);
    font-weight: 500;
  }

  .tool-content {
    padding: 8px 14px 10px;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--text-secondary);
    line-height: 1.5;
    overflow-x: auto;
    border-top: 1px solid var(--border-muted);
  }
</style>
