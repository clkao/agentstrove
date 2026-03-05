<!-- ABOUTME: Scrollable list of session items with loading and empty states. -->
<!-- ABOUTME: Renders SessionItem for each session and shows load-more when paginated. -->
<script lang="ts">
  import { sessions } from "../../stores/sessions.svelte.js";
  import SessionItem from "./SessionItem.svelte";
</script>

<div class="session-list">
  {#if sessions.loading && sessions.sessions.length === 0}
    <div class="loading-state">Loading sessions...</div>
  {:else if sessions.sessions.length === 0}
    <div class="empty-state">No conversations found</div>
  {:else}
    <div class="session-items">
      {#each sessions.sessions as session (session.id)}
        <SessionItem {session} />
      {/each}
    </div>
    {#if sessions.nextCursor}
      <button
        class="load-more"
        onclick={() => sessions.loadMore()}
        disabled={sessions.loading}
      >
        {sessions.loading ? "Loading..." : "Load more"}
      </button>
    {/if}
  {/if}
</div>

<style>
  .session-list {
    flex: 1;
    overflow-y: auto;
  }

  .session-items {
    display: flex;
    flex-direction: column;
  }

  .loading-state,
  .empty-state {
    padding: 32px 16px;
    text-align: center;
    color: var(--text-muted);
    font-size: 13px;
  }

  .load-more {
    display: block;
    width: 100%;
    padding: 10px;
    text-align: center;
    font-size: 12px;
    color: var(--accent-blue);
    border-top: 1px solid var(--border-muted);
  }

  .load-more:hover:not(:disabled) {
    background: var(--bg-surface-hover);
  }
</style>
