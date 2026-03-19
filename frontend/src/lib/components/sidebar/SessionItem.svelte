<!-- ABOUTME: Compact row for a single session in the sidebar list. -->
<!-- ABOUTME: Renders as an <a> tag with permalink href for native link behavior. -->
<script lang="ts">
  import type { Session } from "../../api/types.js";
  import { router } from "../../stores/router.svelte.js";
  import { formatRelativeTime, formatTokenCount, truncate } from "../../utils/format.js";

  let { session }: { session: Session } = $props();

  let active = $derived(router.sessionId === session.id);

  function handleClick(event: MouseEvent): void {
    event.preventDefault();
    router.navigate({ page: "browser", sessionId: session.id });
  }
</script>

<a
  class="session-item"
  class:active
  href="/sessions/{session.id}"
  onclick={handleClick}
>
  <div class="session-title">{truncate(session.display_name || session.first_message || session.id, 80)}</div>
  <div class="session-info">
    <span class="developer">{session.user_name}</span>
    {#if session.commit_count > 0}
      <span class="commit-badge" title="{session.commit_count} git commit{session.commit_count === 1 ? '' : 's'} linked">&#x2022; {session.commit_count}</span>
    {/if}
    {#if session.total_output_tokens > 0}
      <span class="token-badge" title="{session.total_output_tokens.toLocaleString()} output tokens">{formatTokenCount(session.total_output_tokens)} tok</span>
    {/if}
    <span class="time">{formatRelativeTime(session.started_at)}</span>
  </div>
</a>

<style>
  .session-item {
    display: block;
    width: 100%;
    padding: 10px 12px;
    text-align: left;
    text-decoration: none;
    color: inherit;
    border-bottom: 1px solid var(--border-muted);
    transition: background 0.1s;
  }

  .session-item:hover {
    background: var(--bg-surface-hover);
  }

  .session-item.active {
    background: var(--bg-inset);
    border-left: 2px solid var(--accent-blue);
    padding-left: 10px;
  }

  .session-title {
    font-size: 13px;
    font-weight: 500;
    color: var(--text-primary);
    line-height: 1.3;
    margin-bottom: 3px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .session-info {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 11px;
    color: var(--text-muted);
  }

  .developer {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .commit-badge,
  .token-badge {
    flex-shrink: 0;
    font-size: 10px;
    padding: 1px 5px;
    border-radius: 3px;
    background: var(--bg-inset);
    color: var(--text-secondary);
  }

  .time {
    flex-shrink: 0;
    margin-left: auto;
  }
</style>
