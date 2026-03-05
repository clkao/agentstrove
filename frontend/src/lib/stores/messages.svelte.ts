// ABOUTME: Message list state management with API integration.
// ABOUTME: Svelte 5 runes-based store for messages belonging to the active session.

import type { MessageWithToolCalls } from "../api/types.js";
import { getSessionMessages } from "../api/client.js";

class MessagesStore {
  messages = $state<MessageWithToolCalls[]>([]);
  loading = $state(false);
  sessionId = $state<string | null>(null);
  targetOrdinal = $state<number | null>(null);

  async load(id: string): Promise<void> {
    this.sessionId = id;
    this.messages = [];
    this.loading = true;
    try {
      this.messages = await getSessionMessages(id);
    } finally {
      this.loading = false;
    }
  }
}

export const messages = new MessagesStore();
