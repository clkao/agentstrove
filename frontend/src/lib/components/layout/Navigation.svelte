<!-- ABOUTME: Top-level navigation bar for switching between Conversations and Analytics. -->
<!-- ABOUTME: Highlights the active page based on the current route. -->
<script lang="ts">
  import { router } from "../../stores/router.svelte.js";

  function navigateTo(event: MouseEvent, page: "browser" | "analytics") {
    event.preventDefault();
    if (page === "browser") {
      router.navigate({ page: "browser", sessionId: null });
    } else {
      router.navigate({ page: "analytics" });
    }
  }
</script>

<nav class="navigation">
  <a href="/" class:active={router.page === "browser"} onclick={(e) => navigateTo(e, "browser")}>Conversations</a>
  <a href="/analytics" class:active={router.page === "analytics"} onclick={(e) => navigateTo(e, "analytics")}>Analytics</a>
</nav>

<style>
  .navigation {
    display: flex;
    gap: 2px;
    padding: 0 14px;
    height: 32px;
    align-items: center;
    background: var(--bg-surface);
    border-bottom: 1px solid var(--border-muted);
    flex-shrink: 0;
  }

  a {
    padding: 4px 10px;
    font-size: 12px;
    font-weight: 500;
    color: var(--text-muted);
    text-decoration: none;
    border-radius: var(--radius-sm);
    transition: color 0.12s, background 0.12s;
  }

  a:hover {
    color: var(--text-secondary);
    background: var(--bg-surface-hover);
  }

  a.active {
    color: var(--text-primary);
    background: var(--bg-inset);
    font-weight: 600;
  }
</style>
