<!-- ABOUTME: Root application component with sidebar + detail panel layout. -->
<!-- ABOUTME: Triggers initial session load on mount using CSS grid layout. -->
<script lang="ts">
  import { onMount } from "svelte";
  import AppHeader from "./lib/components/layout/AppHeader.svelte";
  import Sidebar from "./lib/components/layout/Sidebar.svelte";
  import DetailPanel from "./lib/components/layout/DetailPanel.svelte";
  import { sessions } from "./lib/stores/sessions.svelte.js";

  onMount(() => {
    sessions.load();
  });
</script>

<AppHeader />
<div class="app-layout">
  <Sidebar />
  <DetailPanel />
</div>

<style>
  .app-layout {
    flex: 1;
    display: flex;
    overflow: hidden;
  }
</style>
