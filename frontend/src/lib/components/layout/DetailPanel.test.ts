// ABOUTME: Tests for DetailPanel git links dropdown rendering and interaction.
// ABOUTME: Verifies commit badge visibility, dropdown items, and scroll-to-message behavior.

import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import { render, screen, cleanup, fireEvent } from "@testing-library/svelte";
import DetailPanel from "./DetailPanel.svelte";
import type { Session, GitLink } from "../../api/types.js";

function makeSession(overrides: Partial<Session> = {}): Session {
  return {
    id: "s1",
    user_name: "Alice",
    user_id: "alice@example.com",
    project_id: "proj-a",
    project_name: "proj-a",
    project_path: "/path/to/proj-a",
    machine: "mac-1",
    agent_type: "claude-code",
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

const mockGitLinks: GitLink[] = [
  {
    session_id: "s1",
    commit_sha: "abc1234def5678",
    pr_url: "",
    link_type: "commit",
    confidence: "high",
    message_ordinal: 3,
  },
  {
    session_id: "s1",
    commit_sha: "",
    pr_url: "https://github.com/org/repo/pull/42",
    link_type: "pr",
    confidence: "medium",
    message_ordinal: 5,
  },
];

let mockActiveSession: Session | null = null;
let mockActiveSessionId: string | null = null;
let mockTargetOrdinal: number | null = null;

vi.mock("../../stores/sessions.svelte.js", () => ({
  sessions: {
    get activeSession() {
      return mockActiveSession;
    },
    get activeSessionId() {
      return mockActiveSessionId;
    },
  },
}));

vi.mock("../../stores/messages.svelte.js", () => ({
  messages: {
    loading: false,
    messages: [],
    get targetOrdinal() {
      return mockTargetOrdinal;
    },
    set targetOrdinal(v: number | null) {
      mockTargetOrdinal = v;
    },
  },
}));

const mockGetSessionGitLinks = vi.fn<(id: string) => Promise<GitLink[]>>();

vi.mock("../../api/client.js", () => ({
  getSessionGitLinks: (...args: [string]) => mockGetSessionGitLinks(...args),
}));

describe("DetailPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockActiveSession = null;
    mockActiveSessionId = null;
    mockTargetOrdinal = null;
    mockGetSessionGitLinks.mockResolvedValue([]);
  });

  afterEach(() => {
    cleanup();
  });

  it("shows empty state when no session is active", () => {
    render(DetailPanel);
    expect(screen.getByText("Select a conversation to view")).toBeTruthy();
  });

  it("does not show commit badge when commit_count is 0", () => {
    mockActiveSession = makeSession({ commit_count: 0 });
    mockActiveSessionId = "s1";

    render(DetailPanel);

    expect(screen.queryByText(/commit/)).toBeNull();
  });

  it("shows commit badge when commit_count > 0", async () => {
    mockActiveSession = makeSession({ id: "s1", commit_count: 2 });
    mockActiveSessionId = "s1";
    mockGetSessionGitLinks.mockResolvedValue(mockGitLinks);

    render(DetailPanel);

    const badge = screen.getByTitle("2 git commits linked");
    expect(badge).toBeTruthy();
    expect(badge.textContent).toContain("2 commits");
  });

  it("shows singular 'commit' for count of 1", () => {
    mockActiveSession = makeSession({ id: "s1", commit_count: 1 });
    mockActiveSessionId = "s1";
    mockGetSessionGitLinks.mockResolvedValue([mockGitLinks[0]]);

    render(DetailPanel);

    const badge = screen.getByTitle("1 git commit linked");
    expect(badge).toBeTruthy();
    expect(badge.textContent).toContain("1 commit");
    expect(badge.textContent).not.toContain("commits");
  });

  it("shows dropdown with git links when badge is clicked", async () => {
    mockActiveSession = makeSession({ id: "s1", commit_count: 2 });
    mockActiveSessionId = "s1";
    mockGetSessionGitLinks.mockResolvedValue(mockGitLinks);

    render(DetailPanel);

    // Wait for git links to load
    await vi.waitFor(() => {
      expect(mockGetSessionGitLinks).toHaveBeenCalledWith("s1");
    });

    // Flush the resolved promise
    await new Promise((r) => setTimeout(r, 0));

    const badge = screen.getByTitle("2 git commits linked");
    await fireEvent.click(badge);

    // Should show truncated SHA
    expect(screen.getByText("abc1234")).toBeTruthy();
    // Should show PR URL
    expect(screen.getByText("https://github.com/org/repo/pull/42")).toBeTruthy();
    // Should show link types
    expect(screen.getByText("commit")).toBeTruthy();
    expect(screen.getByText("pr")).toBeTruthy();
    // Should show non-high confidence
    expect(screen.getByText("medium")).toBeTruthy();
  });

  it("sets targetOrdinal when a git link item is clicked", async () => {
    mockActiveSession = makeSession({ id: "s1", commit_count: 2 });
    mockActiveSessionId = "s1";
    mockGetSessionGitLinks.mockResolvedValue(mockGitLinks);

    render(DetailPanel);

    await vi.waitFor(() => {
      expect(mockGetSessionGitLinks).toHaveBeenCalledWith("s1");
    });
    await new Promise((r) => setTimeout(r, 0));

    const badge = screen.getByTitle("2 git commits linked");
    await fireEvent.click(badge);

    const shaLink = screen.getByText("abc1234");
    await fireEvent.click(shaLink.closest("button")!);

    expect(mockTargetOrdinal).toBe(3);
  });
});
