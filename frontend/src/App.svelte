<!-- ABOUTME: Root application component with sidebar + detail panel layout. -->
<!-- ABOUTME: Triggers initial session load on mount using CSS grid layout. -->
<script lang="ts">
  import { onMount } from "svelte";
  import Sidebar from "./lib/components/layout/Sidebar.svelte";
  import DetailPanel from "./lib/components/layout/DetailPanel.svelte";
  import { sessions } from "./lib/stores/sessions.svelte.js";

  onMount(() => {
    sessions.load();
  });
</script>

<div class="app-layout">
  <Sidebar />
  <DetailPanel />
</div>

<style>
  .app-layout {
    height: 100%;
    display: flex;
  }
</style>
