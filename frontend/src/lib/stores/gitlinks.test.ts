// ABOUTME: Tests for GitLinksStore: lookup, clear, error handling, and PR URL detection.
// ABOUTME: Mocks the lookupGitLinks API function to verify store state transitions.

import { describe, it, expect, vi, beforeEach } from "vitest";
import type { GitLinkResult } from "../api/types.js";

vi.mock("../api/client.js", () => ({
  lookupGitLinks: vi.fn(),
}));

import { lookupGitLinks } from "../api/client.js";

const mockLookupGitLinks = vi.mocked(lookupGitLinks);

function makeResult(overrides: Partial<GitLinkResult> = {}): GitLinkResult {
  return {
    session_id: "s1",
    user_name: "Alice",
    user_id: "alice@example.com",
    project_id: "myapp",
    project_name: "myapp",
    agent_type: "claude-code",
    started_at: "2026-03-04T10:00:00Z",
    first_message: "Fix the login bug",
    commit_sha: "abc1234def5678",
    pr_url: "",
    link_type: "commit",
    confidence: "high",
    message_ordinal: 5,
    ...overrides,
  };
}

describe("GitLinksStore", () => {
  let gitlinks: typeof import("./gitlinks.svelte.js").gitlinks;

  beforeEach(async () => {
    vi.clearAllMocks();
    vi.resetModules();
    const mod = await import("./gitlinks.svelte.js");
    gitlinks = mod.gitlinks;
  });

  describe("lookup", () => {
    it("calls lookupGitLinks with SHA and populates results", async () => {
      const results = [makeResult(), makeResult({ session_id: "s2" })];
      mockLookupGitLinks.mockResolvedValueOnce(results);

      gitlinks.query = "abc1234";
      await gitlinks.lookup();

      expect(mockLookupGitLinks).toHaveBeenCalledWith("abc1234", undefined);
      expect(gitlinks.results).toHaveLength(2);
      expect(gitlinks.loading).toBe(false);
      expect(gitlinks.error).toBeNull();
    });

    it("detects PR URL and passes it as pr parameter", async () => {
      mockLookupGitLinks.mockResolvedValueOnce([makeResult({ link_type: "pr" })]);

      gitlinks.query = "https://github.com/org/repo/pull/42";
      await gitlinks.lookup();

      expect(mockLookupGitLinks).toHaveBeenCalledWith(
        undefined,
        "https://github.com/org/repo/pull/42",
      );
      expect(gitlinks.results).toHaveLength(1);
    });

    it("sets error when no results found", async () => {
      mockLookupGitLinks.mockResolvedValueOnce([]);

      gitlinks.query = "deadbeef";
      await gitlinks.lookup();

      expect(gitlinks.results).toHaveLength(0);
      expect(gitlinks.error).toBe("No conversations found for this commit/PR");
    });

    it("sets error on API failure", async () => {
      mockLookupGitLinks.mockRejectedValueOnce(new Error("network error"));

      gitlinks.query = "abc1234";
      await gitlinks.lookup();

      expect(gitlinks.results).toHaveLength(0);
      expect(gitlinks.error).toBe("network error");
      expect(gitlinks.loading).toBe(false);
    });

    it("clears results without API call when query is empty", async () => {
      gitlinks.query = "";
      await gitlinks.lookup();

      expect(mockLookupGitLinks).not.toHaveBeenCalled();
      expect(gitlinks.results).toHaveLength(0);
    });

    it("clears results without API call when query is whitespace", async () => {
      gitlinks.query = "   ";
      await gitlinks.lookup();

      expect(mockLookupGitLinks).not.toHaveBeenCalled();
      expect(gitlinks.results).toHaveLength(0);
    });
  });

  describe("clear", () => {
    it("resets query, results, and error", async () => {
      mockLookupGitLinks.mockResolvedValueOnce([makeResult()]);
      gitlinks.query = "abc1234";
      await gitlinks.lookup();
      expect(gitlinks.results).toHaveLength(1);

      gitlinks.clear();

      expect(gitlinks.query).toBe("");
      expect(gitlinks.results).toHaveLength(0);
      expect(gitlinks.error).toBeNull();
    });
  });
});
