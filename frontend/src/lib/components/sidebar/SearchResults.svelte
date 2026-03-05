<!-- ABOUTME: Scrollable list of search results when a search query is active. -->
<!-- ABOUTME: Shows loading, empty, or result list states with total count header. -->
<script lang="ts">
  import { search } from "../../stores/search.svelte.js";
  import SearchResultItem from "./SearchResultItem.svelte";
</script>

<div class="search-results">
  {#if search.loading}
    <div class="status-message">Searching...</div>
  {:else if search.results.length === 0}
    <div class="status-message">No results found</div>
  {:else}
    <div class="results-header">{search.total} result{search.total === 1 ? "" : "s"}</div>
    <div class="results-list">
      {#each search.results as result (result.session_id + "-" + result.ordinal)}
        <SearchResultItem {result} />
      {/each}
    </div>
  {/if}
</div>

<style>
  .search-results {
    flex: 1;
    overflow-y: auto;
  }

  .status-message {
    color: var(--text-muted);
    font-size: 13px;
    text-align: center;
    padding: 24px 12px;
  }

  .results-header {
    padding: 8px 12px;
    font-size: 11px;
    font-weight: 500;
    color: var(--text-muted);
    border-bottom: 1px solid var(--border-muted);
    background: var(--bg-surface);
  }

  .results-list {
    display: flex;
    flex-direction: column;
  }
</style>
