<!-- ABOUTME: Detail panel showing session metadata header and rendered messages. -->
<!-- ABOUTME: Wires MessageList to messages store with scroll-to-message support. -->
<script lang="ts">
  import type { GitLink } from "../../api/types.js";
  import { getSessionGitLinks } from "../../api/client.js";
  import { sessions } from "../../stores/sessions.svelte.js";
  import { messages } from "../../stores/messages.svelte.js";
  import { formatTimestamp, formatAgentName, formatNumber } from "../../utils/format.js";
  import MessageList from "../content/MessageList.svelte";

  let highlightOrdinal = $state<number | null>(null);
  let gitLinks = $state<GitLink[]>([]);
  let gitLinksOpen = $state(false);

  $effect(() => {
    if (
      !messages.loading &&
      messages.messages.length > 0 &&
      messages.targetOrdinal !== null
    ) {
      const ordinal = messages.targetOrdinal;
      messages.targetOrdinal = null;
      highlightOrdinal = ordinal;
      requestAnimationFrame(() => {
        const el = document.querySelector(`[data-ordinal="${ordinal}"]`);
        el?.scrollIntoView({ behavior: "smooth", block: "center" });
      });
    }
  });

  $effect(() => {
    const session = sessions.activeSession;
    highlightOrdinal = null;
    gitLinks = [];
    gitLinksOpen = false;

    if (session && session.commit_count > 0) {
      getSessionGitLinks(session.id).then((links) => {
        if (sessions.activeSessionId === session.id) {
          gitLinks = links;
        }
      });
    }
  });

  function scrollToMessage(ordinal: number): void {
    messages.targetOrdinal = ordinal;
    gitLinksOpen = false;
  }

  function formatLinkLabel(link: GitLink): string {
    if (link.commit_sha) {
      return link.commit_sha.slice(0, 7);
    }
    return link.pr_url;
  }
</script>

<main class="detail-panel">
  {#if sessions.activeSession}
    {@const s = sessions.activeSession}
    <header class="session-header">
      <div class="session-meta">
        <span class="meta-item" title="User">{s.user_name}</span>
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
        {#if s.commit_count > 0}
          <span class="meta-sep"></span>
          <span class="gitlinks-container">
            <button
              class="commit-badge"
              title="{s.commit_count} git commit{s.commit_count === 1 ? '' : 's'} linked"
              onclick={() => gitLinksOpen = !gitLinksOpen}
            >
              &#x2022; {s.commit_count} commit{s.commit_count === 1 ? '' : 's'}
            </button>
            {#if gitLinksOpen && gitLinks.length > 0}
              <div class="gitlinks-dropdown">
                {#each gitLinks as link}
                  <button
                    class="gitlink-item"
                    onclick={() => scrollToMessage(link.message_ordinal)}
                  >
                    <span class="gitlink-label">{formatLinkLabel(link)}</span>
                    <span class="gitlink-type">{link.link_type}</span>
                    {#if link.confidence !== "high"}
                      <span class="gitlink-confidence">{link.confidence}</span>
                    {/if}
                  </button>
                {/each}
              </div>
            {/if}
          </span>
        {/if}
      </div>
    </header>
    <section class="message-area">
      {#if messages.loading}
        <div class="loading-placeholder">Loading messages...</div>
      {:else if messages.messages.length > 0}
        <MessageList messages={messages.messages} developerName={s.user_name} agentName={formatAgentName(s.agent_type)} {highlightOrdinal} />
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

  .gitlinks-container {
    position: relative;
  }

  .commit-badge {
    font-size: 10px;
    padding: 1px 5px;
    border-radius: 3px;
    background: var(--bg-inset);
    color: var(--text-secondary);
    border: none;
    cursor: pointer;
    font-family: inherit;
  }

  .commit-badge:hover {
    background: var(--bg-surface-hover);
  }

  .gitlinks-dropdown {
    position: absolute;
    top: 100%;
    left: 0;
    margin-top: 4px;
    background: var(--bg-surface);
    border: 1px solid var(--border-default);
    border-radius: 6px;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
    z-index: 10;
    min-width: 200px;
    max-height: 240px;
    overflow-y: auto;
  }

  .gitlink-item {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    padding: 6px 10px;
    border: none;
    background: none;
    color: var(--text-primary);
    font-size: 12px;
    font-family: monospace;
    cursor: pointer;
    text-align: left;
  }

  .gitlink-item:hover {
    background: var(--bg-surface-hover);
  }

  .gitlink-type {
    font-family: inherit;
    font-size: 10px;
    padding: 0 4px;
    border-radius: 3px;
    background: var(--bg-inset);
    color: var(--text-muted);
  }

  .gitlink-confidence {
    font-family: inherit;
    font-size: 10px;
    padding: 0 4px;
    border-radius: 3px;
    background: var(--bg-inset);
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
