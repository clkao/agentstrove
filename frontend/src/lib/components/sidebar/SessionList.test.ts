// ABOUTME: Tests for SessionList component rendering states.
// ABOUTME: Verifies session items, empty state, loading indicator, and load-more button.

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, cleanup } from "@testing-library/svelte";
import SessionList from "./SessionList.svelte";
import type { Session } from "../../api/types.js";

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

vi.mock("../../stores/sessions.svelte.js", () => {
  let _sessions: Session[] = [];
  let _loading = false;
  let _nextCursor = "";
  let _activeSessionId: string | null = null;

  return {
    sessions: {
      get sessions() {
        return _sessions;
      },
      set sessions(v: Session[]) {
        _sessions = v;
      },
      get loading() {
        return _loading;
      },
      set loading(v: boolean) {
        _loading = v;
      },
      get nextCursor() {
        return _nextCursor;
      },
      set nextCursor(v: string) {
        _nextCursor = v;
      },
      get activeSessionId() {
        return _activeSessionId;
      },
      set activeSessionId(v: string | null) {
        _activeSessionId = v;
      },
      selectSession: vi.fn(),
      loadMore: vi.fn(),
    },
  };
});

vi.mock("../../stores/messages.svelte.js", () => ({
  messages: {
    load: vi.fn(),
  },
}));

import { sessions } from "../../stores/sessions.svelte.js";

describe("SessionList", () => {
  afterEach(() => {
    cleanup();
  });

  beforeEach(() => {
    vi.clearAllMocks();
    sessions.sessions = [];
    sessions.loading = false;
    sessions.nextCursor = "";
  });

  it("renders session items when sessions exist", () => {
    sessions.sessions = [
      makeSession({ id: "s1", first_message: "First task" }),
      makeSession({ id: "s2", first_message: "Second task" }),
    ];

    render(SessionList);

    expect(screen.getByText("First task")).toBeTruthy();
    expect(screen.getByText("Second task")).toBeTruthy();
  });

  it("shows empty state when sessions array is empty", () => {
    sessions.sessions = [];
    sessions.loading = false;

    render(SessionList);

    expect(screen.getByText("No conversations found")).toBeTruthy();
  });

  it("shows loading indicator when loading is true and no sessions", () => {
    sessions.loading = true;
    sessions.sessions = [];

    render(SessionList);

    expect(screen.getByText("Loading sessions...")).toBeTruthy();
  });

  it("shows load more button when nextCursor is non-empty", () => {
    sessions.sessions = [makeSession()];
    sessions.nextCursor = "cursor-abc";

    render(SessionList);

    expect(screen.getByText("Load more")).toBeTruthy();
  });

  it("hides load more button when nextCursor is empty", () => {
    sessions.sessions = [makeSession()];
    sessions.nextCursor = "";

    render(SessionList);

    expect(screen.queryByText("Load more")).toBeNull();
  });
});
