<!-- ABOUTME: Root application component with hash-based routing. -->
<!-- ABOUTME: Switches between conversation browser and analytics dashboard views. -->
<script lang="ts">
  import { onMount } from "svelte";
  import AppHeader from "./lib/components/layout/AppHeader.svelte";
  import Navigation from "./lib/components/layout/Navigation.svelte";
  import Sidebar from "./lib/components/layout/Sidebar.svelte";
  import DetailPanel from "./lib/components/layout/DetailPanel.svelte";
  import AnalyticsPage from "./lib/components/analytics/AnalyticsPage.svelte";
  import { sessions } from "./lib/stores/sessions.svelte.js";
  import { analytics } from "./lib/stores/analytics.svelte.js";

  type Page = "browser" | "analytics";

  function getPage(): Page {
    const hash = window.location.hash;
    return hash === "#/analytics" ? "analytics" : "browser";
  }

  let page = $state<Page>(getPage());

  function handleHashChange() {
    const newPage = getPage();
    if (newPage !== page) {
      page = newPage;
      if (page === "analytics") {
        analytics.load();
      }
    }
  }

  onMount(() => {
    window.addEventListener("hashchange", handleHashChange);
    if (page === "browser") {
      sessions.load();
    } else {
      analytics.load();
    }
    return () => window.removeEventListener("hashchange", handleHashChange);
  });
</script>

<AppHeader />
<Navigation {page} />
{#if page === "analytics"}
  <AnalyticsPage />
{:else}
  <div class="app-layout">
    <Sidebar />
    <DetailPanel />
  </div>
{/if}

<style>
  .app-layout {
    flex: 1;
    display: flex;
    overflow: hidden;
  }
</style>
