<!-- ABOUTME: Detail panel showing session metadata header and rendered messages. -->
<!-- ABOUTME: Wires MessageList to messages store with scroll-to-message support. -->
<script lang="ts">
  import { sessions } from "../../stores/sessions.svelte.js";
  import { messages } from "../../stores/messages.svelte.js";
  import { formatTimestamp, formatAgentName, formatNumber } from "../../utils/format.js";
  import MessageList from "../content/MessageList.svelte";

  $effect(() => {
    if (
      !messages.loading &&
      messages.messages.length > 0 &&
      messages.targetOrdinal !== null
    ) {
      const ordinal = messages.targetOrdinal;
      messages.targetOrdinal = null;
      requestAnimationFrame(() => {
        const el = document.querySelector(`[data-ordinal="${ordinal}"]`);
        el?.scrollIntoView({ behavior: "smooth", block: "center" });
      });
    }
  });
</script>

<main class="detail-panel">
  {#if sessions.activeSession}
    {@const s = sessions.activeSession}
    <header class="session-header">
      <h2 class="session-title">{s.first_message ?? "Untitled conversation"}</h2>
      <div class="session-meta">
        <span class="meta-item" title="User">{s.user_name}</span>
        <span class="meta-sep"></span>
        <span class="meta-item" title="User ID">{s.user_id}</span>
        <span class="meta-sep"></span>
        <span class="meta-item" title="Project">{s.project_name}</span>
        <span class="meta-sep"></span>
        <span class="meta-item" title="Agent">{formatAgentName(s.agent_type)}</span>
      </div>
      <div class="session-meta">
        <span class="meta-item" title="Started">{formatTimestamp(s.started_at)}</span>
        <span class="meta-sep"></span>
        <span class="meta-item" title="Ended">{formatTimestamp(s.ended_at)}</span>
        <span class="meta-sep"></span>
        <span class="meta-item" title="Messages">{formatNumber(s.message_count)} messages</span>
      </div>
    </header>
    <section class="message-area">
      {#if messages.loading}
        <div class="loading-placeholder">Loading messages...</div>
      {:else if messages.messages.length > 0}
        <MessageList messages={messages.messages} developerName={s.user_name} agentName={formatAgentName(s.agent_type)} />
      {:else}
        <div class="loading-placeholder">No messages found.</div>
      {/if}
    </section>
  {:else}
    <div class="empty-state">
      <p>Select a conversation to view</p>
    </div>
  {/if}
</main>

<style>
  .detail-panel {
    flex: 1;
    height: 100%;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    background: var(--bg-primary);
  }

  .session-header {
    padding: 16px 20px;
    border-bottom: 1px solid var(--border-default);
    background: var(--bg-surface);
  }

  .session-title {
    font-size: 15px;
    font-weight: 600;
    color: var(--text-primary);
    line-height: 1.3;
    margin-bottom: 6px;
  }

  .session-meta {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: var(--text-secondary);
    margin-top: 4px;
  }

  .meta-sep::after {
    content: "\00b7";
    color: var(--text-muted);
  }

  .message-area {
    flex: 1;
    overflow-y: auto;
    padding: 16px 20px;
  }

  .loading-placeholder {
    color: var(--text-muted);
    font-size: 13px;
    text-align: center;
    padding: 40px 0;
  }

  .empty-state {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-muted);
    font-size: 14px;
  }
</style>
