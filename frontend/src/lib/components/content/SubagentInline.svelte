<!-- ABOUTME: Inline expansion of a subagent session within the parent conversation. -->
<!-- ABOUTME: Lazily fetches subagent messages on first expand with green visual distinction. -->
<script lang="ts">
  import type { MessageWithToolCalls } from "../../api/types.js";
  import { getSessionMessages } from "../../api/client.js";
  import { router, buildUrl } from "../../stores/router.svelte.js";
  import MessageList from "./MessageList.svelte";

  interface Props {
    sessionId: string;
  }

  let { sessionId }: Props = $props();

  let expanded: boolean = $state(false);
  let loading: boolean = $state(false);
  let messages: MessageWithToolCalls[] | null = $state(null);
  let error: string | null = $state(null);

  async function toggleExpand() {
    expanded = !expanded;
    if (expanded && messages === null && !loading) {
      loading = true;
      error = null;
      try {
        messages = await getSessionMessages(sessionId);
      } catch (e) {
        error = e instanceof Error ? e.message : "Failed to load subagent session";
      } finally {
        loading = false;
      }
    }
  }

  function handleLinkClick(event: MouseEvent) {
    event.preventDefault();
    router.navigate({ page: "browser", sessionId });
  }

  let permalink = $derived(
    buildUrl({ page: "browser", sessionId }),
  );
</script>

<div class="subagent-inline">
  <div class="subagent-header">
    <button class="subagent-toggle" onclick={toggleExpand}>
      <span class="toggle-chevron" class:open={expanded}>&#9656;</span>
      <span class="toggle-label">Subagent session</span>
      <span class="toggle-session-id">{sessionId.slice(0, 8)}...</span>
    </button>
    <a
      class="subagent-link"
      href={permalink}
      onclick={handleLinkClick}
      title="Open subagent session"
    >&#8599;</a>
  </div>
  {#if expanded}
    {#if loading}
      <div class="subagent-loading">Loading subagent session...</div>
    {:else if error}
      <div class="subagent-error">{error}</div>
    {:else if messages}
      <div class="subagent-messages">
        <MessageList {messages} />
      </div>
    {/if}
  {/if}
</div>

<style>
  .subagent-inline {
    margin-top: 4px;
  }

  .subagent-header {
    display: flex;
    align-items: center;
  }

  .subagent-toggle {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 8px;
    font-size: 12px;
    color: var(--text-secondary);
    border-radius: var(--radius-sm);
    transition: background 0.1s;
  }

  .subagent-toggle:hover {
    color: var(--text-primary);
    background: var(--bg-surface-hover);
  }

  .toggle-chevron {
    display: inline-block;
    font-size: 10px;
    transition: transform 0.15s;
  }

  .toggle-chevron.open {
    transform: rotate(90deg);
  }

  .toggle-label {
    font-weight: 500;
    color: var(--accent-green);
  }

  .toggle-session-id {
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--text-muted);
  }

  .subagent-link {
    font-size: 11px;
    color: var(--text-muted);
    text-decoration: none;
    margin-left: 4px;
  }

  .subagent-link:hover {
    color: var(--accent-green);
  }

  .subagent-messages {
    border-left: 3px solid var(--accent-green);
    margin: 4px 0 4px 10px;
    padding: 4px 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .subagent-loading {
    padding: 8px 14px;
    font-size: 12px;
    color: var(--text-muted);
    margin-left: 10px;
  }

  .subagent-error {
    padding: 8px 14px;
    font-size: 12px;
    color: var(--accent-red);
    margin-left: 10px;
  }
</style>
