<!-- ABOUTME: Search input with debounced query execution placed above FilterBar. -->
<!-- ABOUTME: Triggers search store on input, clears on empty, shows clear button when active. -->
<script lang="ts">
  import { search } from "../../stores/search.svelte.js";
  import { sessions } from "../../stores/sessions.svelte.js";

  let debounceTimer: ReturnType<typeof setTimeout>;

  function handleInput(event: Event): void {
    const value = (event.target as HTMLInputElement).value;
    search.query = value;
    clearTimeout(debounceTimer);
    if (!value.trim()) {
      search.clear();
      return;
    }
    debounceTimer = setTimeout(() => {
      search.search(sessions.filters);
    }, 300);
  }

  function handleClear(): void {
    search.clear();
  }
</script>

<div class="search-bar">
  <input
    type="search"
    class="search-input"
    placeholder="Search conversations..."
    value={search.query}
    oninput={handleInput}
    aria-label="Search conversations"
  />
  {#if search.active}
    <button class="clear-btn" onclick={handleClear} aria-label="Clear search">x</button>
  {/if}
</div>

<style>
  .search-bar {
    padding: 10px 12px;
    border-bottom: 1px solid var(--border-muted);
    background: var(--bg-surface);
    position: relative;
  }

  .search-input {
    width: 100%;
    padding: 6px 28px 6px 10px;
    background: var(--bg-inset);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-sm);
    font-size: 13px;
    color: var(--text-primary);
    outline: none;
    transition: border-color 0.15s;
  }

  .search-input::placeholder {
    color: var(--text-muted);
  }

  .search-input:focus {
    border-color: var(--accent-blue);
  }

  .clear-btn {
    position: absolute;
    right: 18px;
    top: 50%;
    transform: translateY(-50%);
    width: 18px;
    height: 18px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 50%;
    background: var(--text-muted);
    color: var(--bg-surface);
    font-size: 11px;
    font-weight: 600;
    cursor: pointer;
    line-height: 1;
    border: none;
    padding: 0;
  }

  .clear-btn:hover {
    background: var(--text-secondary);
  }
</style>
