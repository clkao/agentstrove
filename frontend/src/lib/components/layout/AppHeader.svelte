<!-- ABOUTME: Top header bar with app name, project filter, and theme toggle. -->
<!-- ABOUTME: Mirrors agentsview header layout with project filter in the header line. -->
<script lang="ts">
  import { onMount } from "svelte";
  import { ui } from "../../stores/ui.svelte.js";
  import { sessions } from "../../stores/sessions.svelte.js";
  import { search } from "../../stores/search.svelte.js";
  import { listProjects } from "../../api/client.js";
  import type { ProjectInfo } from "../../api/types.js";

  let projects = $state<ProjectInfo[]>([]);
  let selectedProject = $state("");

  onMount(async () => {
    projects = await listProjects();
  });

  function onProjectChange(): void {
    const filters = {
      project_name: selectedProject || undefined,
    };
    sessions.updateFilters(filters);
    if (search.active) {
      search.search(filters);
    }
  }
</script>

<header class="header">
  <div class="header-left">
    <span class="header-title">Agentstrove</span>

    <select
      class="project-select"
      bind:value={selectedProject}
      onchange={onProjectChange}
      aria-label="Filter by project"
    >
      <option value="">All projects</option>
      {#each projects as proj}
        <option value={proj.name}>{proj.name}</option>
      {/each}
    </select>
  </div>

  <div class="header-right">
    <button
      class="header-btn"
      onclick={() => ui.toggleTheme()}
      title="Toggle theme"
      aria-label="Toggle theme"
    >
      {#if ui.theme === "light"}
        <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor">
          <path d="M6 .278a.768.768 0 01.08.858 7.208 7.208 0 00-.878 3.46c0 4.021 3.278 7.277 7.318 7.277.527 0 1.04-.055 1.533-.16a.787.787 0 01.81.316.733.733 0 01-.031.893A8.349 8.349 0 018.344 16C3.734 16 0 12.286 0 7.71 0 4.266 2.114 1.312 5.124.06A.752.752 0 016 .278z"/>
        </svg>
      {:else}
        <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor">
          <path d="M8 12a4 4 0 100-8 4 4 0 000 8zM8 0a.5.5 0 01.5.5v2a.5.5 0 01-1 0v-2A.5.5 0 018 0zm0 13a.5.5 0 01.5.5v2a.5.5 0 01-1 0v-2A.5.5 0 018 13zm8-5a.5.5 0 01-.5.5h-2a.5.5 0 010-1h2A.5.5 0 0116 8zM3 8a.5.5 0 01-.5.5h-2a.5.5 0 010-1h2A.5.5 0 013 8zm10.657-5.657a.5.5 0 010 .707l-1.414 1.414a.5.5 0 11-.707-.707l1.414-1.414a.5.5 0 01.707 0zm-9.193 9.193a.5.5 0 010 .707L3.05 13.657a.5.5 0 01-.707-.707l1.414-1.414a.5.5 0 01.707 0zm9.193 2.121a.5.5 0 01-.707 0l-1.414-1.414a.5.5 0 01.707-.707l1.414 1.414a.5.5 0 010 .707zM4.464 4.465a.5.5 0 01-.707 0L2.343 3.05a.5.5 0 01.707-.707l1.414 1.414a.5.5 0 010 .708z"/>
        </svg>
      {/if}
    </button>
  </div>
</header>

<style>
  .header {
    height: 40px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 14px;
    background: var(--bg-surface);
    border-bottom: 1px solid var(--border-default);
    flex-shrink: 0;
    gap: 10px;
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: 12px;
    min-width: 0;
  }

  .header-title {
    font-size: 12px;
    font-weight: 650;
    color: var(--text-primary);
    white-space: nowrap;
    letter-spacing: -0.01em;
  }

  .project-select {
    height: 26px;
    padding: 0 8px;
    background: var(--bg-inset);
    border: 1px solid var(--border-muted);
    border-radius: var(--radius-sm);
    font-size: 11px;
    color: var(--text-primary);
    min-width: 140px;
    max-width: 260px;
  }

  .header-right {
    display: flex;
    align-items: center;
    gap: 2px;
  }

  .header-btn {
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: var(--radius-sm);
    color: var(--text-muted);
    font-size: 12px;
    font-weight: 600;
    transition: background 0.12s, color 0.12s;
  }

  .header-btn:hover {
    background: var(--bg-surface-hover);
    color: var(--text-secondary);
  }
</style>
