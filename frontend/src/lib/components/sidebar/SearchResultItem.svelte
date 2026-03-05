<!-- ABOUTME: Renders one search result with highlighted snippet and metadata. -->
<!-- ABOUTME: Renders as an <a> tag with deep-link href to the matched message. -->
<script lang="ts">
  import type { SearchResult, Highlight } from "../../api/types.js";
  import { router } from "../../stores/router.svelte.js";
  import { formatRelativeTime } from "../../utils/format.js";

  let { result }: { result: SearchResult } = $props();

  function handleClick(event: MouseEvent): void {
    event.preventDefault();
    router.navigate({ page: "browser", sessionId: result.session_id, messageOrdinal: result.ordinal });
  }

  function buildSnippetParts(
    snippet: string,
    highlights: Highlight[],
  ): { text: string; highlight: boolean }[] {
    if (!highlights || highlights.length === 0) {
      return [{ text: snippet, highlight: false }];
    }
    const sorted = [...highlights].sort((a, b) => a.start - b.start);
    const parts: { text: string; highlight: boolean }[] = [];
    let pos = 0;
    for (const h of sorted) {
      if (h.start > pos) {
        parts.push({ text: snippet.slice(pos, h.start), highlight: false });
      }
      parts.push({
        text: snippet.slice(h.start, h.end),
        highlight: true,
      });
      pos = h.end;
    }
    if (pos < snippet.length) {
      parts.push({ text: snippet.slice(pos), highlight: false });
    }
    return parts;
  }

  let parts = $derived(buildSnippetParts(result.snippet, result.highlights));
</script>

<a class="result-item" href="/sessions/{result.session_id}#msg-{result.ordinal}" onclick={handleClick}>
  <div class="result-snippet">
    {#each parts as part}
      {#if part.highlight}
        <mark>{part.text}</mark>
      {:else}
        {part.text}
      {/if}
    {/each}
  </div>
  <div class="result-meta">
    <span class="developer">{result.user_name}</span>
    <span class="project">{result.project_name}</span>
    <span class="time">{formatRelativeTime(result.started_at)}</span>
  </div>
</a>

<style>
  .result-item {
    display: block;
    width: 100%;
    padding: 10px 12px;
    text-align: left;
    text-decoration: none;
    color: inherit;
    border-bottom: 1px solid var(--border-muted);
    transition: background 0.1s;
    cursor: pointer;
  }

  .result-item:hover {
    background: var(--bg-surface-hover);
  }

  .result-snippet {
    font-size: 13px;
    color: var(--text-primary);
    line-height: 1.4;
    margin-bottom: 4px;
    overflow: hidden;
    display: -webkit-box;
    -webkit-line-clamp: 3;
    -webkit-box-orient: vertical;
  }

  .result-snippet :global(mark) {
    background: var(--accent-yellow, #fef08a);
    color: inherit;
    padding: 0 1px;
    border-radius: 2px;
  }

  .result-meta {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 11px;
    color: var(--text-muted);
    gap: 8px;
  }

  .developer {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .project {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text-secondary);
  }

  .time {
    flex-shrink: 0;
    margin-left: auto;
  }
</style>
