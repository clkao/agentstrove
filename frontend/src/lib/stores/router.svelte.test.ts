// ABOUTME: Tests for RouterStore: URL parsing, URL building, navigate, and replace.
// ABOUTME: Verifies route state transitions and History API interactions.

import { describe, it, expect, vi, beforeEach } from "vitest";
import { parseUrl, buildUrl } from "./router.svelte.js";
import type { Route } from "./router.svelte.js";

describe("parseUrl", () => {
  it("parses / as browser with no session", () => {
    expect(parseUrl("/", "")).toEqual({ page: "browser", sessionId: null });
  });

  it("parses /sessions/ as browser with no session", () => {
    expect(parseUrl("/sessions/", "")).toEqual({ page: "browser", sessionId: null });
  });

  it("parses /sessions as browser with no session", () => {
    expect(parseUrl("/sessions", "")).toEqual({ page: "browser", sessionId: null });
  });

  it("parses /sessions/{id} as browser with session", () => {
    expect(parseUrl("/sessions/abc-123", "")).toEqual({
      page: "browser",
      sessionId: "abc-123",
    });
  });

  it("parses /sessions/{id}#msg-{ordinal} with messageOrdinal", () => {
    expect(parseUrl("/sessions/abc-123", "#msg-42")).toEqual({
      page: "browser",
      sessionId: "abc-123",
      messageOrdinal: 42,
    });
  });

  it("ignores non-msg hash fragments", () => {
    expect(parseUrl("/sessions/abc-123", "#other")).toEqual({
      page: "browser",
      sessionId: "abc-123",
    });
  });

  it("parses /analytics as analytics page", () => {
    expect(parseUrl("/analytics", "")).toEqual({ page: "analytics" });
  });

  it("falls back to browser with no session for unknown paths", () => {
    expect(parseUrl("/unknown/path", "")).toEqual({
      page: "browser",
      sessionId: null,
    });
  });

  it("parses session IDs with dots and colons", () => {
    expect(parseUrl("/sessions/ses.2025:abc", "")).toEqual({
      page: "browser",
      sessionId: "ses.2025:abc",
    });
  });

  it("parses session IDs with encoded slashes", () => {
    expect(parseUrl("/sessions/id%2Fwith%2Fslashes", "")).toEqual({
      page: "browser",
      sessionId: "id%2Fwith%2Fslashes",
    });
  });

  it("includes trailing slash in session ID for /sessions/{id}/", () => {
    expect(parseUrl("/sessions/abc-123/", "")).toEqual({
      page: "browser",
      sessionId: "abc-123/",
    });
  });

  it("does not match /analytics/ with trailing slash", () => {
    expect(parseUrl("/analytics/", "")).toEqual({
      page: "browser",
      sessionId: null,
    });
  });

  it("parses /analytics ignoring hash fragment", () => {
    expect(parseUrl("/analytics", "#something")).toEqual({
      page: "analytics",
    });
  });

  it("falls back to browser with no session for empty pathname", () => {
    expect(parseUrl("", "")).toEqual({
      page: "browser",
      sessionId: null,
    });
  });

  it("parses #msg-0 as messageOrdinal 0", () => {
    expect(parseUrl("/sessions/s1", "#msg-0")).toEqual({
      page: "browser",
      sessionId: "s1",
      messageOrdinal: 0,
    });
  });
});

describe("buildUrl", () => {
  it("builds / for browser with no session", () => {
    expect(buildUrl({ page: "browser", sessionId: null })).toBe("/");
  });

  it("builds /sessions/{id} for browser with session", () => {
    expect(buildUrl({ page: "browser", sessionId: "abc-123" })).toBe(
      "/sessions/abc-123",
    );
  });

  it("builds /sessions/{id}#msg-{ordinal} with messageOrdinal", () => {
    expect(
      buildUrl({ page: "browser", sessionId: "abc-123", messageOrdinal: 42 }),
    ).toBe("/sessions/abc-123#msg-42");
  });

  it("builds /analytics for analytics page", () => {
    expect(buildUrl({ page: "analytics" })).toBe("/analytics");
  });

  it("builds URL with special characters in session ID", () => {
    expect(buildUrl({ page: "browser", sessionId: "ses.2025:abc" })).toBe(
      "/sessions/ses.2025:abc",
    );
  });

  it("builds #msg-0 for messageOrdinal 0", () => {
    expect(
      buildUrl({ page: "browser", sessionId: "s1", messageOrdinal: 0 }),
    ).toBe("/sessions/s1#msg-0");
  });
});

describe("RouterStore", () => {
  let router: typeof import("./router.svelte.js").router;

  beforeEach(async () => {
    vi.resetModules();

    // Reset URL to / before each test
    window.history.replaceState({}, "", "/");

    const mod = await import("./router.svelte.js");
    router = mod.router;
  });

  describe("initial state", () => {
    it("parses current URL on construction", () => {
      expect(router.route).toEqual({ page: "browser", sessionId: null });
      expect(router.page).toBe("browser");
      expect(router.sessionId).toBeNull();
      expect(router.messageOrdinal).toBeNull();
    });
  });

  describe("navigate", () => {
    it("updates route state", () => {
      router.navigate({ page: "browser", sessionId: "s1" });

      expect(router.route).toEqual({ page: "browser", sessionId: "s1" });
      expect(router.sessionId).toBe("s1");
    });

    it("pushes URL to history", () => {
      const pushSpy = vi.spyOn(history, "pushState");

      router.navigate({ page: "browser", sessionId: "s1" });

      expect(pushSpy).toHaveBeenCalledWith({}, "", "/sessions/s1");
      pushSpy.mockRestore();
    });

    it("navigates to analytics page", () => {
      router.navigate({ page: "analytics" });

      expect(router.route).toEqual({ page: "analytics" });
      expect(router.page).toBe("analytics");
      expect(router.sessionId).toBeNull();
    });

    it("navigates with messageOrdinal", () => {
      const pushSpy = vi.spyOn(history, "pushState");

      router.navigate({
        page: "browser",
        sessionId: "s1",
        messageOrdinal: 7,
      });

      expect(router.messageOrdinal).toBe(7);
      expect(pushSpy).toHaveBeenCalledWith({}, "", "/sessions/s1#msg-7");
      pushSpy.mockRestore();
    });
  });

  describe("popstate", () => {
    it("updates route when popstate fires", () => {
      router.navigate({ page: "analytics" });
      expect(router.route).toEqual({ page: "analytics" });

      // Simulate browser back: set location then fire popstate
      history.replaceState({}, "", "/sessions/s1");
      window.dispatchEvent(new PopStateEvent("popstate"));

      expect(router.route).toEqual({ page: "browser", sessionId: "s1" });
    });
  });

  describe("replace", () => {
    it("updates route state", () => {
      router.replace({ page: "browser", sessionId: "s2" });

      expect(router.route).toEqual({ page: "browser", sessionId: "s2" });
    });

    it("replaces URL without new history entry", () => {
      const replaceSpy = vi.spyOn(history, "replaceState");

      router.replace({ page: "browser", sessionId: "s2" });

      expect(replaceSpy).toHaveBeenCalledWith({}, "", "/sessions/s2");
      replaceSpy.mockRestore();
    });
  });
});
