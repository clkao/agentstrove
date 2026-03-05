<!-- ABOUTME: Compact row for a single session in the sidebar list. -->
<!-- ABOUTME: Shows truncated first message, user name, and relative time. -->
<script lang="ts">
  import type { Session } from "../../api/types.js";
  import { sessions } from "../../stores/sessions.svelte.js";
  import { messages } from "../../stores/messages.svelte.js";
  import { formatRelativeTime, truncate } from "../../utils/format.js";

  let { session }: { session: Session } = $props();

  let active = $derived(sessions.activeSessionId === session.id);

  function handleClick(): void {
    sessions.selectSession(session.id);
    messages.load(session.id);
  }
</script>

<button
  class="session-item"
  class:active
  onclick={handleClick}
  type="button"
>
  <div class="session-title">{truncate(session.first_message ?? "Untitled", 80)}</div>
  <div class="session-info">
    <span class="developer">{session.user_name}</span>
    {#if session.commit_count > 0}
      <span class="commit-badge" title="{session.commit_count} git commit{session.commit_count === 1 ? '' : 's'} linked">&#x2022; {session.commit_count}</span>
    {/if}
    <span class="time">{formatRelativeTime(session.started_at)}</span>
  </div>
</button>

<style>
  .session-item {
    display: block;
    width: 100%;
    padding: 10px 12px;
    text-align: left;
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

  .commit-badge {
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
