<!-- ABOUTME: Compact rendering of consecutive tool-only assistant messages. -->
<!-- ABOUTME: Groups multiple tool calls with minimal spacing to save vertical space. -->
<script lang="ts">
  import type { MessageWithToolCalls } from "../../api/types.js";
  import type { ContentSegment } from "../../utils/content-parser.js";
  import {
    parseContent,
    enrichSegments,
  } from "../../utils/content-parser.js";
  import { formatTimestamp } from "../../utils/format.js";
  import ToolBlock from "./ToolBlock.svelte";

  interface Props {
    messages: MessageWithToolCalls[];
    timestamp: string | null;
    expandTools?: boolean;
  }

  let { messages, timestamp, expandTools = false }: Props = $props();

  let toolSegments = $derived.by(() => {
    const segments: ContentSegment[] = [];
    for (const msg of messages) {
      const parsed = parseContent(msg.content, msg.has_tool_use);
      const enriched = enrichSegments(parsed, msg.tool_calls);
      for (const seg of enriched) {
        if (seg.type === "tool") {
          segments.push(seg);
        }
      }
    }
    return segments;
  });
</script>

<div class="tool-call-group">
  {#if timestamp}
    <span class="group-timestamp">
      {formatTimestamp(timestamp)}
    </span>
  {/if}
  <div class="group-tools">
    {#each toolSegments as segment}
      <ToolBlock
        content={segment.content}
        label={segment.label}
        toolCall={segment.toolCall}
        startExpanded={expandTools}
      />
    {/each}
  </div>
</div>

<style>
  .tool-call-group {
    border-left: 3px solid var(--accent-amber);
    background: var(--tool-bg);
    border-radius: 0 var(--radius-md) var(--radius-md) 0;
    padding: 4px 0;
    position: relative;
  }

  .group-timestamp {
    position: absolute;
    top: 4px;
    right: 10px;
    font-size: 11px;
    color: var(--text-muted);
  }

  .group-tools {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .tool-call-group :global(.tool-block) {
    border-left: none;
    background: transparent;
    border-radius: 0;
    margin: 0;
  }
</style>
