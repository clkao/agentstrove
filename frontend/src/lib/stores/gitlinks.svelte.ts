// ABOUTME: Git link lookup state management with API integration.
// ABOUTME: Svelte 5 runes-based store for commit/PR lookup query and results.

import type { GitLinkResult } from "../api/types.js";
import { lookupGitLinks } from "../api/client.js";

class GitLinksStore {
  query = $state("");
  results = $state<GitLinkResult[]>([]);
  loading = $state(false);
  error = $state<string | null>(null);

  async lookup(): Promise<void> {
    const q = this.query.trim();
    if (!q) {
      this.results = [];
      return;
    }
    this.loading = true;
    this.error = null;
    try {
      const isPR = q.startsWith("https://github.com/") && q.includes("/pull/");
      const results = isPR
        ? await lookupGitLinks(undefined, q)
        : await lookupGitLinks(q, undefined);
      this.results = results;
      if (results.length === 0) {
        this.error = "No conversations found for this commit/PR";
      }
    } catch (e) {
      this.error = e instanceof Error ? e.message : "Lookup failed";
      this.results = [];
    } finally {
      this.loading = false;
    }
  }

  clear(): void {
    this.query = "";
    this.results = [];
    this.error = null;
  }
}

export const gitlinks = new GitLinksStore();
