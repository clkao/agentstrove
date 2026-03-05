// ABOUTME: Search state management with API integration.
// ABOUTME: Svelte 5 runes-based store for search query, results, and active/loading state.

import type { SearchResult, Filters } from "../api/types.js";
import { searchMessages } from "../api/client.js";

class SearchStore {
  query = $state("");
  results = $state<SearchResult[]>([]);
  loading = $state(false);
  total = $state(0);

  active = $derived(this.query.trim().length > 0);

  async search(filters: Filters): Promise<void> {
    if (!this.query.trim()) {
      this.results = [];
      this.total = 0;
      return;
    }
    this.loading = true;
    try {
      const page = await searchMessages(this.query, filters);
      this.results = page.results;
      this.total = page.total;
    } finally {
      this.loading = false;
    }
  }

  clear(): void {
    this.query = "";
    this.results = [];
    this.total = 0;
  }
}

export const search = new SearchStore();
