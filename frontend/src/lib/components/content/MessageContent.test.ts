// ABOUTME: Behavioral tests for MessageContent rendering.
// ABOUTME: Verifies text/markdown, tool call display, thinking blocks, model badge, and token info.

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
    model: "",
    token_usage: "",
    context_tokens: 0,
    output_tokens: 0,
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

  it("shows model badge on assistant messages with a model name", () => {
    const msg = makeMessage({
      role: "assistant",
      model: "claude-opus-4-20250514",
    });
    const { container } = render(MessageContent, {
      props: { message: msg },
    });
    const badge = container.querySelector(".model-badge");
    expect(badge).toBeTruthy();
    expect(badge!.textContent).toBe("opus-4");
  });

  it("does not show model badge when model is empty", () => {
    const msg = makeMessage({ role: "assistant", model: "" });
    const { container } = render(MessageContent, {
      props: { message: msg },
    });
    const badge = container.querySelector(".model-badge");
    expect(badge).toBeNull();
  });

  it("does not show model badge on user messages", () => {
    const msg = makeMessage({
      role: "user",
      model: "claude-opus-4-20250514",
    });
    const { container } = render(MessageContent, {
      props: { message: msg },
    });
    const badge = container.querySelector(".model-badge");
    expect(badge).toBeNull();
  });

  it("shows token info on assistant messages with token counts", () => {
    const msg = makeMessage({
      role: "assistant",
      context_tokens: 45000,
      output_tokens: 1200,
    });
    const { container } = render(MessageContent, {
      props: { message: msg },
    });
    const tokenInfo = container.querySelector(".token-info");
    expect(tokenInfo).toBeTruthy();
    expect(tokenInfo!.textContent).toContain("ctx: 45k");
    expect(tokenInfo!.textContent).toContain("out: 1.2k");
  });

  it("does not show token info when both counts are zero", () => {
    const msg = makeMessage({
      role: "assistant",
      context_tokens: 0,
      output_tokens: 0,
    });
    const { container } = render(MessageContent, {
      props: { message: msg },
    });
    const tokenInfo = container.querySelector(".token-info");
    expect(tokenInfo).toBeNull();
  });

  it("shows token info with only context tokens", () => {
    const msg = makeMessage({
      role: "assistant",
      context_tokens: 8000,
      output_tokens: 0,
    });
    const { container } = render(MessageContent, {
      props: { message: msg },
    });
    const tokenInfo = container.querySelector(".token-info");
    expect(tokenInfo).toBeTruthy();
    expect(tokenInfo!.textContent).toContain("ctx: 8k");
    expect(tokenInfo!.textContent).not.toContain("out:");
  });

  it("shows token info with only output tokens", () => {
    const msg = makeMessage({
      role: "assistant",
      context_tokens: 0,
      output_tokens: 500,
    });
    const { container } = render(MessageContent, {
      props: { message: msg },
    });
    const tokenInfo = container.querySelector(".token-info");
    expect(tokenInfo).toBeTruthy();
    expect(tokenInfo!.textContent).not.toContain("ctx:");
    expect(tokenInfo!.textContent).toContain("out: 500");
  });

  it("does not show token info on user messages", () => {
    const msg = makeMessage({
      role: "user",
      context_tokens: 45000,
      output_tokens: 1200,
    });
    const { container } = render(MessageContent, {
      props: { message: msg },
    });
    const tokenInfo = container.querySelector(".token-info");
    expect(tokenInfo).toBeNull();
  });
});
