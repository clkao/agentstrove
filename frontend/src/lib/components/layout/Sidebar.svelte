<!-- ABOUTME: Sidebar layout with search bar, filters, and conditional session/search results. -->
<!-- ABOUTME: Swaps between session list and search results based on active search query. -->
<script lang="ts">
  import { search } from "../../stores/search.svelte.js";
  import GitLinkLookup from "../sidebar/GitLinkLookup.svelte";
  import SearchBar from "../sidebar/SearchBar.svelte";
  import FilterBar from "../sidebar/FilterBar.svelte";
  import SessionList from "../sidebar/SessionList.svelte";
  import SearchResults from "../sidebar/SearchResults.svelte";
</script>

<aside class="sidebar">
  <GitLinkLookup />
  <SearchBar />
  <FilterBar />
  {#if search.active}
    <SearchResults />
  {:else}
    <SessionList />
  {/if}
</aside>

<style>
  .sidebar {
    width: var(--sidebar-width);
    height: 100%;
    display: flex;
    flex-direction: column;
    background: var(--bg-surface);
    border-right: 1px solid var(--border-default);
    overflow: hidden;
  }
</style>
