// ABOUTME: Tests for SessionsStore: load, filter, select, and pagination.
// ABOUTME: Mocks the API client to verify store state transitions.

import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session, SessionPage } from "../api/types.js";

vi.mock("../api/client.js", () => ({
  listSessions: vi.fn(),
  getSession: vi.fn(),
}));

import { listSessions, getSession } from "../api/client.js";

const mockListSessions = vi.mocked(listSessions);
const mockGetSession = vi.mocked(getSession);

function makeSession(overrides: Partial<Session> = {}): Session {
  return {
    id: "s1",
    user_name: "Alice",
    user_id: "alice@example.com",
    project_id: "proj-a",
    project_name: "proj-a",
    project_path: "/path/to/proj-a",
    machine: "mac-1",
    agent_type: "claude",
    first_message: "Hello world",
    started_at: "2025-01-01T00:00:00Z",
    ended_at: "2025-01-01T01:00:00Z",
    message_count: 10,
    user_message_count: 3,
    parent_session_id: "",
    relationship_type: "",
    commit_count: 0,
    ...overrides,
  };
}

function makePage(
  sessions: Session[],
  nextCursor = "",
  total = sessions.length,
): SessionPage {
  return { sessions, next_cursor: nextCursor, total };
}

describe("SessionsStore", () => {
  let sessions: typeof import("./sessions.svelte.js").sessions;

  beforeEach(async () => {
    vi.clearAllMocks();
    vi.resetModules();
    const mod = await import("./sessions.svelte.js");
    sessions = mod.sessions;
  });

  describe("load", () => {
    it("fetches sessions from API and populates state", async () => {
      const page = makePage([makeSession(), makeSession({ id: "s2" })], "", 2);
      mockListSessions.mockResolvedValueOnce(page);

      await sessions.load();

      expect(mockListSessions).toHaveBeenCalledWith({});
      expect(sessions.sessions).toHaveLength(2);
      expect(sessions.total).toBe(2);
      expect(sessions.nextCursor).toBe("");
      expect(sessions.loading).toBe(false);
    });

    it("sets loading to false after fetch completes", async () => {
      mockListSessions.mockResolvedValueOnce(makePage([]));
      await sessions.load();
      expect(sessions.loading).toBe(false);
    });

    it("sets loading to false even on API error", async () => {
      mockListSessions.mockRejectedValueOnce(new Error("network error"));
      await sessions.load().catch(() => {});
      expect(sessions.loading).toBe(false);
    });

    it("passes current filters to API", async () => {
      mockListSessions.mockResolvedValue(makePage([]));
      sessions.filters = { user_id: "alice@example.com", project_id: "proj-a" };
      await sessions.load();
      expect(mockListSessions).toHaveBeenCalledWith({
        user_id: "alice@example.com",
        project_id: "proj-a",
      });
    });
  });

  describe("updateFilters", () => {
    it("merges partial filters and triggers reload", async () => {
      mockListSessions.mockResolvedValue(makePage([]));
      await sessions.updateFilters({ user_id: "bob@example.com" });
      expect(sessions.filters.user_id).toBe("bob@example.com");
      expect(mockListSessions).toHaveBeenCalled();
    });

    it("resets active session on filter change", async () => {
      mockListSessions.mockResolvedValue(
        makePage([makeSession()]),
      );
      await sessions.load();
      sessions.selectSession("s1");
      expect(sessions.activeSessionId).toBe("s1");

      mockListSessions.mockResolvedValue(makePage([]));
      await sessions.updateFilters({ project_id: "other" });
      expect(sessions.activeSessionId).toBeNull();
    });
  });

  describe("selectSession", () => {
    it("sets activeSessionId", async () => {
      mockListSessions.mockResolvedValue(
        makePage([makeSession({ id: "s1" }), makeSession({ id: "s2" })]),
      );
      await sessions.load();
      sessions.selectSession("s2");
      expect(sessions.activeSessionId).toBe("s2");
    });

    it("derives activeSession from sessions list", async () => {
      const s = makeSession({ id: "s3", user_name: "Charlie" });
      mockListSessions.mockResolvedValue(makePage([s]));
      await sessions.load();
      sessions.selectSession("s3");
      expect(sessions.activeSession).toEqual(s);
    });

    it("returns null for activeSession when id not in list (before fetch)", async () => {
      mockListSessions.mockResolvedValue(makePage([makeSession()]));
      mockGetSession.mockResolvedValue(makeSession({ id: "nonexistent" }));
      await sessions.load();
      sessions.selectSession("nonexistent");
      // Synchronously null before fetch resolves
      expect(sessions.activeSession).toBeNull();
    });

    it("fetches session from API when id not in list", async () => {
      const fetched = makeSession({ id: "remote-1", user_name: "Remote" });
      mockListSessions.mockResolvedValue(makePage([makeSession()]));
      mockGetSession.mockResolvedValue(fetched);
      await sessions.load();
      sessions.selectSession("remote-1");
      expect(mockGetSession).toHaveBeenCalledWith("remote-1");
      await vi.waitFor(() => {
        expect(sessions.activeSession).toEqual(fetched);
      });
    });
  });

  describe("loadMore", () => {
    it("appends next page to existing sessions", async () => {
      const page1 = makePage([makeSession({ id: "s1" })], "cursor-1", 3);
      mockListSessions.mockResolvedValueOnce(page1);
      await sessions.load();
      expect(sessions.sessions).toHaveLength(1);

      const page2 = makePage(
        [makeSession({ id: "s2" }), makeSession({ id: "s3" })],
        "",
        3,
      );
      mockListSessions.mockResolvedValueOnce(page2);
      await sessions.loadMore();

      expect(sessions.sessions).toHaveLength(3);
      expect(sessions.sessions.map((s) => s.id)).toEqual(["s1", "s2", "s3"]);
      expect(sessions.nextCursor).toBe("");
    });

    it("passes cursor in the API call", async () => {
      const page1 = makePage([makeSession()], "abc123", 2);
      mockListSessions.mockResolvedValueOnce(page1);
      await sessions.load();

      mockListSessions.mockResolvedValueOnce(makePage([], "", 2));
      await sessions.loadMore();

      expect(mockListSessions).toHaveBeenLastCalledWith(
        expect.objectContaining({ cursor: "abc123" }),
      );
    });

    it("does nothing when no nextCursor", async () => {
      mockListSessions.mockResolvedValue(makePage([]));
      await sessions.load();
      await sessions.loadMore();
      expect(mockListSessions).toHaveBeenCalledTimes(1);
    });
  });
});
