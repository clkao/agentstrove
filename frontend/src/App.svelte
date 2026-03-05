<!-- ABOUTME: Root application component with URL-based routing. -->
<!-- ABOUTME: Switches between conversation browser and analytics dashboard views. -->
<script lang="ts">
  import AppHeader from "./lib/components/layout/AppHeader.svelte";
  import Navigation from "./lib/components/layout/Navigation.svelte";
  import Sidebar from "./lib/components/layout/Sidebar.svelte";
  import DetailPanel from "./lib/components/layout/DetailPanel.svelte";
  import AnalyticsPage from "./lib/components/analytics/AnalyticsPage.svelte";
  import { router } from "./lib/stores/router.svelte.js";
  import { sessions } from "./lib/stores/sessions.svelte.js";
  import { messages } from "./lib/stores/messages.svelte.js";
  import { analytics } from "./lib/stores/analytics.svelte.js";

  $effect(() => {
    if (router.page === "browser") {
      sessions.load();
    } else if (router.page === "analytics") {
      analytics.load();
    }
  });

  $effect(() => {
    const sessionId = router.sessionId;
    if (sessionId) {
      sessions.selectSession(sessionId);
      messages.load(sessionId);
    } else {
      sessions.selectSession(null);
    }
  });

  $effect(() => {
    const ordinal = router.messageOrdinal;
    if (ordinal !== null) {
      messages.targetOrdinal = ordinal;
    }
  });
</script>

<AppHeader />
<Navigation />
{#if router.page === "analytics"}
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
