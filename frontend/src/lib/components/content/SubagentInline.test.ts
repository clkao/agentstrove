// ABOUTME: Tests that SubagentInline groups consecutive tool-only messages into compact displays.
// ABOUTME: Verifies tool-call-group rendering via mocked API responses on expand.

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, fireEvent, waitFor, cleanup } from "@testing-library/svelte";
import type { MessageWithToolCalls } from "../../api/types.js";
import SubagentInline from "./SubagentInline.svelte";

function makeMessage(
  overrides: Partial<MessageWithToolCalls> & { content: string; ordinal: number },
): MessageWithToolCalls {
  return {
    session_id: "sub-session-1",
    role: "assistant",
    has_tool_use: false,
    has_thinking: false,
    content_length: overrides.content.length,
    timestamp: "2024-01-01T00:00:00Z",
    tool_calls: [],
    ...overrides,
  };
}

vi.mock("../../api/client.js", () => ({
  getSessionMessages: vi.fn(),
}));

import { getSessionMessages } from "../../api/client.js";
const mockGetSessionMessages = vi.mocked(getSessionMessages);

describe("SubagentInline", () => {
  afterEach(() => {
    cleanup();
  });

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders consecutive tool-only messages as a tool-call-group", async () => {
    const messages: MessageWithToolCalls[] = [
      makeMessage({ content: "[Bash]\necho hi", ordinal: 1, has_tool_use: true }),
      makeMessage({ content: "[Read]\nfile.ts", ordinal: 2, has_tool_use: true }),
      makeMessage({ content: "[Edit]\nchanges", ordinal: 3, has_tool_use: true }),
    ];
    mockGetSessionMessages.mockResolvedValue(messages);

    const { getByText, container } = render(SubagentInline, {
      props: { sessionId: "sub-session-1" },
    });

    const toggle = getByText("Subagent session");
    await fireEvent.click(toggle);

    await waitFor(() => {
      const groups = container.querySelectorAll(".tool-call-group");
      expect(groups.length).toBeGreaterThan(0);
    });
  });

  it("renders non-tool messages individually with MessageContent", async () => {
    const messages: MessageWithToolCalls[] = [
      makeMessage({ content: "Here is my analysis", ordinal: 1 }),
      makeMessage({ content: "[Bash]\necho hi", ordinal: 2, has_tool_use: true }),
      makeMessage({ content: "[Read]\nfile.ts", ordinal: 3, has_tool_use: true }),
      makeMessage({ content: "All done!", ordinal: 4 }),
    ];
    mockGetSessionMessages.mockResolvedValue(messages);

    const { getByText, container } = render(SubagentInline, {
      props: { sessionId: "sub-session-1" },
    });

    const toggle = getByText("Subagent session");
    await fireEvent.click(toggle);

    await waitFor(() => {
      // Non-tool messages should render as individual .message elements
      const messageElements = container.querySelectorAll(".message");
      expect(messageElements.length).toBe(2);

      // Consecutive tool-only messages should be grouped
      const groups = container.querySelectorAll(".tool-call-group");
      expect(groups.length).toBe(1);
    });
  });
});
