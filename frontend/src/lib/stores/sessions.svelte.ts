// ABOUTME: Session list state management with API integration.
// ABOUTME: Svelte 5 runes-based store for sessions, filters, pagination, and active selection.

import type { Session, SessionPage, Filters } from "../api/types.js";
import { listSessions, getSession } from "../api/client.js";

class SessionsStore {
  sessions = $state<Session[]>([]);
  activeSessionId = $state<string | null>(null);
  loading = $state(false);
  filters = $state<Filters>({});
  nextCursor = $state("");
  total = $state(0);

  activeSession = $derived(
    this.sessions.find((s) => s.id === this.activeSessionId) ?? null,
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
  }

  async ensureSession(id: string): Promise<void> {
    this.activeSessionId = id;
    if (this.sessions.find((s) => s.id === id)) return;
    const session = await getSession(id);
    if (!this.sessions.find((s) => s.id === id)) {
      this.sessions = [session, ...this.sessions];
    }
  }

  async updateFilters(partial: Partial<Filters>): Promise<void> {
    this.filters = { ...this.filters, ...partial };
    this.activeSessionId = null;
    await this.load();
  }
}

export const sessions = new SessionsStore();
