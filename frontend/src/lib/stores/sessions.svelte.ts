// ABOUTME: Session list state management with API integration.
// ABOUTME: Svelte 5 runes-based store for sessions, filters, pagination, and active selection.

import type { Session, SessionPage, Filters } from "../api/types.js";
import { listSessions, getSession } from "../api/client.js";

class SessionsStore {
  sessions = $state<Session[]>([]);
  activeSessionId = $state<string | null>(null);
  fetchedSession = $state<Session | null>(null);
  loading = $state(false);
  filters = $state<Filters>({});
  nextCursor = $state("");
  total = $state(0);

  activeSession = $derived(
    this.sessions.find((s) => s.id === this.activeSessionId) ??
      (this.fetchedSession?.id === this.activeSessionId
        ? this.fetchedSession
        : null),
  );

  async load(): Promise<void> {
    this.loading = true;
    try {
      const page: SessionPage = await listSessions(this.filters);
      this.sessions = page.sessions;
      this.nextCursor = page.next_cursor;
      this.total = page.total;
    } finally {
      this.loading = false;
    }
  }

  async loadMore(): Promise<void> {
    if (!this.nextCursor) return;
    this.loading = true;
    try {
      const page: SessionPage = await listSessions({
        ...this.filters,
        cursor: this.nextCursor,
      });
      this.sessions = [...this.sessions, ...page.sessions];
      this.nextCursor = page.next_cursor;
      this.total = page.total;
    } finally {
      this.loading = false;
    }
  }

  selectSession(id: string | null): void {
    this.activeSessionId = id;
    if (id && !this.sessions.find((s) => s.id === id)) {
      this.fetchedSession = null;
      getSession(id).then((s) => {
        this.fetchedSession = s;
      });
    }
  }

  async updateFilters(partial: Partial<Filters>): Promise<void> {
    this.filters = { ...this.filters, ...partial };
    this.activeSessionId = null;
    await this.load();
  }
}

export const sessions = new SessionsStore();
