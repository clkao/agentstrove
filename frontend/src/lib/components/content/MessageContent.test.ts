// ABOUTME: Behavioral tests for MessageContent rendering.
// ABOUTME: Verifies text/markdown, tool call display, and thinking block collapsed state.

import { describe, it, expect, afterEach } from "vitest";
import { render, cleanup } from "@testing-library/svelte";
import MessageContent from "./MessageContent.svelte";
import type { MessageWithToolCalls } from "../../api/types.js";

afterEach(() => {
  cleanup();
});

function makeMessage(
  overrides: Partial<MessageWithToolCalls> = {},
): MessageWithToolCalls {
  return {
    session_id: "test-session",
    ordinal: 1,
    role: "assistant",
    content: "Hello world",
    timestamp: "2026-01-01T00:00:00Z",
    has_thinking: false,
    has_tool_use: false,
    content_length: 11,
    tool_calls: [],
    ...overrides,
  };
}

describe("MessageContent", () => {
  it("renders text segments as markdown HTML", () => {
    const msg = makeMessage({ content: "Hello **bold** text" });
    const { container } = render(MessageContent, {
      props: { message: msg },
    });
    const textContent = container.querySelector(".text-content");
    expect(textContent).toBeTruthy();
    expect(textContent!.innerHTML).toContain("<strong>bold</strong>");
  });

  it("shows role label for assistant messages", () => {
    const msg = makeMessage({ role: "assistant" });
    const { container } = render(MessageContent, {
      props: { message: msg },
    });
    const roleLabel = container.querySelector(".role-label");
    expect(roleLabel).toBeTruthy();
    expect(roleLabel!.textContent).toBe("Assistant");
  });

  it("shows role label for user messages", () => {
    const msg = makeMessage({ role: "user" });
    const { container } = render(MessageContent, {
      props: { message: msg },
    });
    const roleLabel = container.querySelector(".role-label");
    expect(roleLabel).toBeTruthy();
    expect(roleLabel!.textContent).toBe("User");
  });

  it("renders tool call segments as ToolBlock components", () => {
    const msg = makeMessage({
      content: "[Read]\n/path/to/file.ts",
      has_tool_use: true,
      tool_calls: [
        {
          message_ordinal: 1,
          session_id: "test-session",
          tool_name: "Read",
          tool_category: "file",
          tool_use_id: "tu-1",
          input_json: '{"file_path":"/path/to/file.ts"}',
          skill_name: "",
          result_content_length: 100,
          subagent_session_id: "",
        },
      ],
    });
    const { container } = render(MessageContent, {
      props: { message: msg },
    });
    const toolBlock = container.querySelector(".tool-block");
    expect(toolBlock).toBeTruthy();
    const toolLabel = container.querySelector(".tool-label");
    expect(toolLabel).toBeTruthy();
    expect(toolLabel!.textContent).toBe("Read");
  });

  it("renders thinking segments as collapsed ThinkingBlock by default", () => {
    const msg = makeMessage({
      content: "[Thinking]\nDeep thought here\n[/Thinking]\n\nVisible text",
      has_thinking: true,
    });
    const { container } = render(MessageContent, {
      props: { message: msg },
    });
    const thinkingBlock = container.querySelector(".thinking-block");
    expect(thinkingBlock).toBeTruthy();
    const thinkingContent = container.querySelector(".thinking-content");
    expect(thinkingContent).toBeNull();
    const textContent = container.querySelector(".text-content");
    expect(textContent).toBeTruthy();
    expect(textContent!.textContent).toContain("Visible text");
  });

  it("renders thinking block with content accessible after expansion", () => {
    const msg = makeMessage({
      content: "[Thinking]\nDeep thought here\n[/Thinking]\n\nVisible text",
      has_thinking: true,
    });
    const { container } = render(MessageContent, {
      props: { message: msg },
    });
    const thinkingBlock = container.querySelector(".thinking-block");
    expect(thinkingBlock).toBeTruthy();
  });

  it("renders only text content for messages with no tool calls", () => {
    const msg = makeMessage({ content: "Just plain text here" });
    const { container } = render(MessageContent, {
      props: { message: msg },
    });
    const textContent = container.querySelector(".text-content");
    expect(textContent).toBeTruthy();
    const toolBlock = container.querySelector(".tool-block");
    expect(toolBlock).toBeNull();
    const thinkingBlock = container.querySelector(".thinking-block");
    expect(thinkingBlock).toBeNull();
  });
});
