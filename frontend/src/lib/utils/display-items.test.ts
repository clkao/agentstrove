// ABOUTME: Tests for display-items: buildDisplayItems grouping logic.
// ABOUTME: Validates consecutive tool-only messages are grouped, mixed messages are preserved.

import { describe, it, expect } from "vitest";
import type { MessageWithToolCalls } from "../api/types.js";
import { buildDisplayItems } from "./display-items.js";

function makeMsg(
  overrides: Partial<MessageWithToolCalls> & { content: string; ordinal: number },
): MessageWithToolCalls {
  const defaults: MessageWithToolCalls = {
    session_id: "s1",
    ordinal: 0,
    role: "assistant",
    content: "",
    has_tool_use: false,
    has_thinking: false,
    content_length: 0,
    timestamp: "2024-01-01T00:00:00Z",
    tool_calls: [],
  };
  return { ...defaults, ...overrides };
}

describe("buildDisplayItems", () => {
  it("returns empty array for empty input", () => {
    expect(buildDisplayItems([])).toEqual([]);
  });

  it("wraps a single text message as a message item", () => {
    const msg = makeMsg({ content: "Hello", ordinal: 1 });
    const items = buildDisplayItems([msg]);
    expect(items).toHaveLength(1);
    expect(items[0]!.kind).toBe("message");
    if (items[0]!.kind === "message") {
      expect(items[0]!.message).toBe(msg);
    }
  });

  it("wraps a single tool-only message as a tool-group", () => {
    const msg = makeMsg({
      content: "[Bash]\necho hi",
      ordinal: 1,
      has_tool_use: true,
    });
    const items = buildDisplayItems([msg]);
    expect(items).toHaveLength(1);
    expect(items[0]!.kind).toBe("tool-group");
    if (items[0]!.kind === "tool-group") {
      expect(items[0]!.messages).toHaveLength(1);
      expect(items[0]!.messages[0]).toBe(msg);
      expect(items[0]!.timestamp).toBe("2024-01-01T00:00:00Z");
    }
  });

  it("groups consecutive tool-only messages", () => {
    const m1 = makeMsg({
      content: "[Bash]\necho hi",
      ordinal: 1,
      has_tool_use: true,
    });
    const m2 = makeMsg({
      content: "[Read]\nfile.ts",
      ordinal: 2,
      has_tool_use: true,
    });
    const m3 = makeMsg({
      content: "[Edit]\nchanges",
      ordinal: 3,
      has_tool_use: true,
    });
    const items = buildDisplayItems([m1, m2, m3]);
    expect(items).toHaveLength(1);
    expect(items[0]!.kind).toBe("tool-group");
    if (items[0]!.kind === "tool-group") {
      expect(items[0]!.messages).toHaveLength(3);
      expect(items[0]!.timestamp).toBe("2024-01-01T00:00:00Z");
    }
  });

  it("breaks group when a non-tool message appears", () => {
    const m1 = makeMsg({
      content: "[Bash]\necho hi",
      ordinal: 1,
      has_tool_use: true,
    });
    const m2 = makeMsg({
      content: "Here is the result",
      ordinal: 2,
    });
    const m3 = makeMsg({
      content: "[Read]\nfile.ts",
      ordinal: 3,
      has_tool_use: true,
    });
    const items = buildDisplayItems([m1, m2, m3]);
    expect(items).toHaveLength(3);
    expect(items[0]!.kind).toBe("tool-group");
    expect(items[1]!.kind).toBe("message");
    expect(items[2]!.kind).toBe("tool-group");
  });

  it("keeps user messages as message items", () => {
    const m1 = makeMsg({
      content: "Please fix it",
      ordinal: 1,
      role: "user",
    });
    const items = buildDisplayItems([m1]);
    expect(items).toHaveLength(1);
    expect(items[0]!.kind).toBe("message");
  });

  it("does not group user messages with tool-only assistant messages", () => {
    const m1 = makeMsg({
      content: "[Bash]\necho hi",
      ordinal: 1,
      has_tool_use: true,
    });
    const m2 = makeMsg({
      content: "user input",
      ordinal: 2,
      role: "user",
    });
    const m3 = makeMsg({
      content: "[Read]\nfile.ts",
      ordinal: 3,
      has_tool_use: true,
    });
    const items = buildDisplayItems([m1, m2, m3]);
    expect(items).toHaveLength(3);
    expect(items[0]!.kind).toBe("tool-group");
    expect(items[1]!.kind).toBe("message");
    expect(items[2]!.kind).toBe("tool-group");
  });

  it("handles mixed sequence correctly", () => {
    const messages = [
      makeMsg({ content: "Let me help", ordinal: 1 }),
      makeMsg({ content: "[Bash]\necho hi", ordinal: 2, has_tool_use: true }),
      makeMsg({ content: "[Read]\nfile.ts", ordinal: 3, has_tool_use: true }),
      makeMsg({ content: "Done! Here are results", ordinal: 4 }),
      makeMsg({ content: "Thanks", ordinal: 5, role: "user" }),
      makeMsg({ content: "[Edit]\nchanges", ordinal: 6, has_tool_use: true }),
    ];
    const items = buildDisplayItems(messages);
    expect(items).toHaveLength(5);
    expect(items[0]!.kind).toBe("message"); // "Let me help"
    expect(items[1]!.kind).toBe("tool-group"); // Bash + Read
    if (items[1]!.kind === "tool-group") {
      expect(items[1]!.messages).toHaveLength(2);
    }
    expect(items[2]!.kind).toBe("message"); // "Done! Here are results"
    expect(items[3]!.kind).toBe("message"); // "Thanks" (user)
    expect(items[4]!.kind).toBe("tool-group"); // Edit
    if (items[4]!.kind === "tool-group") {
      expect(items[4]!.messages).toHaveLength(1);
    }
  });

  it("uses first message timestamp for group", () => {
    const m1 = makeMsg({
      content: "[Bash]\necho hi",
      ordinal: 1,
      has_tool_use: true,
      timestamp: "2024-01-01T10:00:00Z",
    });
    const m2 = makeMsg({
      content: "[Read]\nfile.ts",
      ordinal: 2,
      has_tool_use: true,
      timestamp: "2024-01-01T10:01:00Z",
    });
    const items = buildDisplayItems([m1, m2]);
    expect(items).toHaveLength(1);
    if (items[0]!.kind === "tool-group") {
      expect(items[0]!.timestamp).toBe("2024-01-01T10:00:00Z");
    }
  });

  it("handles tool-only with thinking as tool-group", () => {
    const msg = makeMsg({
      content: "[Thinking]\nhmm\n[Bash]\necho hi",
      ordinal: 1,
      has_tool_use: true,
    });
    const items = buildDisplayItems([msg]);
    expect(items).toHaveLength(1);
    expect(items[0]!.kind).toBe("tool-group");
  });

  it("treats assistant message with text and tools as message item", () => {
    const msg = makeMsg({
      content: "Some explanation\n[Bash]\necho hi",
      ordinal: 1,
      has_tool_use: true,
    });
    const items = buildDisplayItems([msg]);
    expect(items).toHaveLength(1);
    expect(items[0]!.kind).toBe("message");
  });
});
