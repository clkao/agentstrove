<!-- ABOUTME: Input field for looking up conversations by commit SHA or PR URL. -->
<!-- ABOUTME: Displays results with confidence badges and enables navigation to the linked conversation. -->
<script lang="ts">
  import { gitlinks } from "../../stores/gitlinks.svelte.js";
  import { sessions } from "../../stores/sessions.svelte.js";
  import { messages } from "../../stores/messages.svelte.js";
  import { truncate, formatRelativeTime } from "../../utils/format.js";

  function handleKeydown(event: KeyboardEvent): void {
    if (event.key === "Enter") {
      gitlinks.lookup();
    }
  }

  function handleLookup(): void {
    gitlinks.lookup();
  }

  function handleClear(): void {
    gitlinks.clear();
  }

  function selectResult(sessionId: string, messageOrdinal: number): void {
    sessions.selectSession(sessionId);
    messages.load(sessionId);
    messages.targetOrdinal = messageOrdinal;
    gitlinks.clear();
  }
</script>

<div class="gitlink-lookup">
  <div class="input-row">
    <input
      type="text"
      class="lookup-input"
      placeholder="Find commit/PR..."
      bind:value={gitlinks.query}
      onkeydown={handleKeydown}
      aria-label="Look up commit SHA or PR URL"
    />
    {#if gitlinks.query.trim()}
      <button class="clear-btn" onclick={handleClear} aria-label="Clear lookup">x</button>
    {/if}
    <button class="lookup-btn" onclick={handleLookup} aria-label="Look up" disabled={!gitlinks.query.trim()}>Go</button>
  </div>

  {#if gitlinks.loading}
    <div class="lookup-status">Searching...</div>
  {/if}

  {#if gitlinks.error && !gitlinks.loading}
    <div class="lookup-error">{gitlinks.error}</div>
  {/if}

  {#if gitlinks.results.length > 0}
    <ul class="lookup-results">
      {#each gitlinks.results as result}
        <li>
          <button
            class="result-item"
            onclick={() => selectResult(result.session_id, result.message_ordinal)}
            type="button"
          >
            <div class="result-title">{truncate(result.first_message ?? "Untitled", 60)}</div>
            <div class="result-meta">
              <span class="result-ref">{result.commit_sha ? truncate(result.commit_sha, 10) : truncate(result.pr_url, 30)}</span>
              <span class="confidence-badge" class:high={result.confidence === "high"} class:medium={result.confidence === "medium"}>{result.confidence === "high" ? "High" : "Med"}</span>
            </div>
            <div class="result-meta">
              <span class="result-dev">{result.user_name}</span>
              <span class="result-time">{formatRelativeTime(result.started_at)}</span>
            </div>
          </button>
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .gitlink-lookup {
    padding: 10px 12px;
    border-bottom: 1px solid var(--border-muted);
    background: var(--bg-surface);
  }

  .input-row {
    display: flex;
    gap: 4px;
    position: relative;
  }

  .lookup-input {
    flex: 1;
    padding: 6px 24px 6px 10px;
    background: var(--bg-inset);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-sm);
    font-size: 13px;
    color: var(--text-primary);
    outline: none;
    transition: border-color 0.15s;
  }

  .lookup-input::placeholder {
    color: var(--text-muted);
  }

  .lookup-input:focus {
    border-color: var(--accent-blue);
  }

  .clear-btn {
    position: absolute;
    right: 36px;
    top: 50%;
    transform: translateY(-50%);
    width: 16px;
    height: 16px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    background: var(--text-muted);
    color: var(--bg-surface);
    font-size: 10px;
    font-weight: 600;
    cursor: pointer;
    line-height: 1;
    border: none;
    padding: 0;
  }

  .clear-btn:hover {
    background: var(--text-secondary);
  }

  .lookup-btn {
    padding: 6px 10px;
    background: var(--accent-blue);
    color: #fff;
    border: none;
    border-radius: var(--radius-sm);
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
    white-space: nowrap;
  }

  .lookup-btn:disabled {
    opacity: 0.5;
    cursor: default;
  }

  .lookup-btn:hover:not(:disabled) {
    opacity: 0.9;
  }

  .lookup-status {
    font-size: 12px;
    color: var(--text-muted);
    padding: 6px 0 0;
  }

  .lookup-error {
    font-size: 12px;
    color: var(--danger-text, #e5534b);
    padding: 6px 0 0;
  }

  .lookup-results {
    list-style: none;
    margin: 6px 0 0;
    padding: 0;
  }

  .result-item {
    display: block;
    width: 100%;
    text-align: left;
    padding: 6px 8px;
    border-radius: var(--radius-sm);
    transition: background 0.1s;
    border: none;
    background: none;
    cursor: pointer;
    color: inherit;
  }

  .result-item:hover {
    background: var(--bg-surface-hover);
  }

  .result-title {
    font-size: 12px;
    font-weight: 500;
    color: var(--text-primary);
    line-height: 1.3;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .result-meta {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 11px;
    color: var(--text-muted);
    margin-top: 2px;
  }

  .result-ref {
    font-family: var(--font-mono, monospace);
    font-size: 10px;
  }

  .confidence-badge {
    display: inline-block;
    padding: 0 4px;
    border-radius: 3px;
    font-size: 10px;
    font-weight: 600;
    line-height: 1.6;
  }

  .confidence-badge.high {
    background: var(--accent-green, #2ea04370);
    color: var(--accent-green-text, #3fb950);
  }

  .confidence-badge.medium {
    background: transparent;
    border: 1px solid var(--text-muted);
    color: var(--text-muted);
  }

  .result-dev {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .result-time {
    flex-shrink: 0;
    margin-left: 8px;
  }
</style>
