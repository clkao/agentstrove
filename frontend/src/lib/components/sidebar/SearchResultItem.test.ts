// ABOUTME: Tests for SearchResultItem snippet highlight rendering and metadata display.
// ABOUTME: Verifies <mark> elements for highlights and user/time metadata.

import { describe, it, expect, vi, afterEach } from "vitest";
import { render, cleanup } from "@testing-library/svelte";
import SearchResultItem from "./SearchResultItem.svelte";
import type { SearchResult } from "../../api/types.js";

vi.mock("../../stores/sessions.svelte.js", () => ({
  sessions: {
    selectSession: vi.fn(),
  },
}));

vi.mock("../../stores/messages.svelte.js", () => ({
  messages: {
    load: vi.fn(),
    targetOrdinal: null,
  },
}));

afterEach(() => {
  cleanup();
});

function makeResult(overrides: Partial<SearchResult> = {}): SearchResult {
  return {
    session_id: "s1",
    ordinal: 5,
    role: "assistant",
    user_id: "alice@example.com",
    user_name: "Alice",
    project_name: "proj-a",
    agent_type: "claude",
    started_at: "2025-01-01T00:00:00Z",
    first_message: "Hello world",
    snippet: "before matching after",
    highlights: [{ start: 7, length: 8 }],
    ...overrides,
  };
}

describe("SearchResultItem", () => {
  it("renders snippet with <mark> elements for highlighted portions", () => {
    const result = makeResult({
      snippet: "before matching after",
      highlights: [{ start: 7, length: 8 }],
    });

    const { container } = render(SearchResultItem, { props: { result } });

    const marks = container.querySelectorAll("mark");
    expect(marks).toHaveLength(1);
    expect(marks[0].textContent).toBe("matching");
  });

  it("renders snippet as plain text when highlights array is empty", () => {
    const result = makeResult({
      snippet: "no highlights here",
      highlights: [],
    });

    const { container } = render(SearchResultItem, { props: { result } });

    const marks = container.querySelectorAll("mark");
    expect(marks).toHaveLength(0);
    expect(container.textContent).toContain("no highlights here");
  });

  it("renders multiple highlights correctly", () => {
    const result = makeResult({
      snippet: "first match and second match here",
      highlights: [
        { start: 6, length: 5 },
        { start: 23, length: 5 },
      ],
    });

    const { container } = render(SearchResultItem, { props: { result } });

    const marks = container.querySelectorAll("mark");
    expect(marks).toHaveLength(2);
    expect(marks[0].textContent).toBe("match");
    expect(marks[1].textContent).toBe("match");
  });

  it("renders user name in the output", () => {
    const result = makeResult({ user_name: "Bob" });

    const { container } = render(SearchResultItem, { props: { result } });

    expect(container.textContent).toContain("Bob");
  });

  it("renders project in the output", () => {
    const result = makeResult({ project_name: "my-project" });

    const { container } = render(SearchResultItem, { props: { result } });

    expect(container.textContent).toContain("my-project");
  });

  it("renders relative time from started_at", () => {
    const result = makeResult({ started_at: "2025-01-01T00:00:00Z" });

    const { container } = render(SearchResultItem, { props: { result } });

    // The relative time will be formatted (e.g., "Jan 1" or "Xd ago")
    // Just verify the meta section exists and has some text content
    const meta = container.querySelector(".result-meta");
    expect(meta).toBeTruthy();
    expect(meta!.textContent!.trim().length).toBeGreaterThan(0);
  });
});
