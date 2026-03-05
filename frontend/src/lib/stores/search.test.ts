// ABOUTME: Tests for SearchStore: search, clear, and active derived state.
// ABOUTME: Mocks the searchMessages API function to verify store state transitions.

import { describe, it, expect, vi, beforeEach } from "vitest";
import type { SearchPage } from "../api/types.js";

vi.mock("../api/client.js", () => ({
  searchMessages: vi.fn(),
}));

import { searchMessages } from "../api/client.js";

const mockSearchMessages = vi.mocked(searchMessages);

function makePage(
  results: SearchPage["results"] = [],
  total = results.length,
): SearchPage {
  return { results, total };
}

function makeResult(overrides: Partial<SearchPage["results"][0]> = {}) {
  return {
    session_id: "s1",
    ordinal: 3,
    role: "assistant",
    user_id: "alice@example.com",
    user_name: "Alice",
    project_name: "proj-a",
    agent_type: "claude",
    started_at: "2025-01-01T00:00:00Z",
    first_message: "Hello world",
    snippet: "...matching text here...",
    highlights: [{ start: 3, length: 8 }],
    ...overrides,
  };
}

describe("SearchStore", () => {
  let search: typeof import("./search.svelte.js").search;

  beforeEach(async () => {
    vi.clearAllMocks();
    vi.resetModules();
    const mod = await import("./search.svelte.js");
    search = mod.search;
  });

  describe("search", () => {
    it("calls searchMessages API and populates results when query is non-empty", async () => {
      const page = makePage([makeResult(), makeResult({ session_id: "s2" })], 2);
      mockSearchMessages.mockResolvedValueOnce(page);

      search.query = "matching";
      await search.search({});

      expect(mockSearchMessages).toHaveBeenCalledWith("matching", {});
      expect(search.results).toHaveLength(2);
      expect(search.total).toBe(2);
      expect(search.loading).toBe(false);
    });

    it("clears results without API call when query is empty", async () => {
      search.query = "";
      await search.search({});

      expect(mockSearchMessages).not.toHaveBeenCalled();
      expect(search.results).toHaveLength(0);
      expect(search.total).toBe(0);
    });

    it("clears results without API call when query is whitespace", async () => {
      search.query = "   ";
      await search.search({});

      expect(mockSearchMessages).not.toHaveBeenCalled();
      expect(search.results).toHaveLength(0);
    });

    it("sets loading to false after API completes", async () => {
      mockSearchMessages.mockResolvedValueOnce(makePage([]));
      search.query = "test";
      await search.search({});
      expect(search.loading).toBe(false);
    });

    it("sets loading to false even on API error", async () => {
      mockSearchMessages.mockRejectedValueOnce(new Error("network error"));
      search.query = "test";
      await search.search({}).catch(() => {});
      expect(search.loading).toBe(false);
    });

    it("passes filters to API", async () => {
      mockSearchMessages.mockResolvedValueOnce(makePage([]));
      search.query = "test";
      await search.search({ user_id: "alice@example.com", project_id: "proj-a" });
      expect(mockSearchMessages).toHaveBeenCalledWith("test", {
        user_id: "alice@example.com",
        project_id: "proj-a",
      });
    });
  });

  describe("clear", () => {
    it("resets query, results, and total", async () => {
      const page = makePage([makeResult()], 1);
      mockSearchMessages.mockResolvedValueOnce(page);

      search.query = "something";
      await search.search({});
      expect(search.results).toHaveLength(1);

      search.clear();

      expect(search.query).toBe("");
      expect(search.results).toHaveLength(0);
      expect(search.total).toBe(0);
    });
  });

  describe("active", () => {
    it("is true when query is non-empty", () => {
      search.query = "hello";
      expect(search.active).toBe(true);
    });

    it("is false when query is empty", () => {
      search.query = "";
      expect(search.active).toBe(false);
    });

    it("is false when query is only whitespace", () => {
      search.query = "   ";
      expect(search.active).toBe(false);
    });
  });
});
