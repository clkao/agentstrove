<!-- ABOUTME: Filter controls for user, project, agent type, and date range. -->
<!-- ABOUTME: Populates dropdown options from metadata API endpoints on mount. -->
<script lang="ts">
  import { onMount } from "svelte";
  import { sessions } from "../../stores/sessions.svelte.js";
  import { search } from "../../stores/search.svelte.js";
  import { listUsers, listProjects, listAgents } from "../../api/client.js";
  import type { UserInfo } from "../../api/types.js";

  let users = $state<UserInfo[]>([]);
  let projects = $state<string[]>([]);
  let agents = $state<string[]>([]);

  let selectedUser = $state("");
  let selectedProject = $state("");
  let selectedAgent = $state("");
  let dateFrom = $state("");
  let dateTo = $state("");

  onMount(async () => {
    const [usrs, projs, agts] = await Promise.all([
      listUsers(),
      listProjects(),
      listAgents(),
    ]);
    users = usrs;
    projects = projs;
    agents = agts;
  });

  function applyFilters(): void {
    const filters = {
      user_id: selectedUser || undefined,
      project_id: selectedProject || undefined,
      agent_type: selectedAgent || undefined,
      date_from: dateFrom || undefined,
      date_to: dateTo || undefined,
    };
    sessions.updateFilters(filters);
    if (search.active) {
      search.search(filters);
    }
  }
</script>

<div class="filter-bar">
  <select
    class="filter-select"
    bind:value={selectedUser}
    onchange={applyFilters}
    aria-label="Filter by user"
  >
    <option value="">All users</option>
    {#each users as user}
      <option value={user.id}>{user.name}</option>
    {/each}
  </select>

  <select
    class="filter-select"
    bind:value={selectedProject}
    onchange={applyFilters}
    aria-label="Filter by project"
  >
    <option value="">All projects</option>
    {#each projects as proj}
      <option value={proj}>{proj}</option>
    {/each}
  </select>

  <select
    class="filter-select"
    bind:value={selectedAgent}
    onchange={applyFilters}
    aria-label="Filter by agent"
  >
    <option value="">All agents</option>
    {#each agents as agent}
      <option value={agent}>{agent}</option>
    {/each}
  </select>

  <div class="date-range">
    <input
      type="date"
      class="date-input"
      bind:value={dateFrom}
      onchange={applyFilters}
      aria-label="Date from"
    />
    <span class="date-sep">to</span>
    <input
      type="date"
      class="date-input"
      bind:value={dateTo}
      onchange={applyFilters}
      aria-label="Date to"
    />
  </div>
</div>

<style>
  .filter-bar {
    padding: 10px 12px;
    display: flex;
    flex-direction: column;
    gap: 6px;
    border-bottom: 1px solid var(--border-muted);
    background: var(--bg-surface);
  }

  .filter-select {
    width: 100%;
    padding: 5px 8px;
    background: var(--bg-inset);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-sm);
    font-size: 12px;
    color: var(--text-primary);
  }

  .date-range {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .date-input {
    flex: 1;
    padding: 5px 8px;
    background: var(--bg-inset);
    border: 1px solid var(--border-default);
    border-radius: var(--radius-sm);
    font-size: 12px;
    color: var(--text-primary);
  }

  .date-sep {
    font-size: 11px;
    color: var(--text-muted);
  }
</style>
