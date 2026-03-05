// ABOUTME: URL-based routing state management using the History API.
// ABOUTME: Svelte 5 runes-based singleton store that maps URL paths to application routes.

export type Route =
  | { page: "browser"; sessionId: null }
  | { page: "browser"; sessionId: string; messageOrdinal?: number }
  | { page: "analytics" };

export function parseUrl(pathname: string, hash: string): Route {
  if (pathname === "/analytics") {
    return { page: "analytics" };
  }

  const match = pathname.match(/^\/sessions\/(.+)$/);
  if (match) {
    const sessionId = match[1];
    const route: Route = { page: "browser", sessionId };
    const hashMatch = hash.match(/^#msg-(\d+)$/);
    if (hashMatch) {
      route.messageOrdinal = parseInt(hashMatch[1], 10);
    }
    return route;
  }

  return { page: "browser", sessionId: null };
}

export function buildUrl(route: Route): string {
  if (route.page === "analytics") {
    return "/analytics";
  }

  if (route.sessionId === null) {
    return "/";
  }

  let url = `/sessions/${route.sessionId}`;
  if (route.messageOrdinal != null) {
    url += `#msg-${route.messageOrdinal}`;
  }
  return url;
}

class RouterStore {
  route = $state<Route>(parseUrl(window.location.pathname, window.location.hash));

  page = $derived(this.route.page);

  sessionId = $derived(
    this.route.page === "browser" ? this.route.sessionId : null,
  );

  messageOrdinal = $derived(
    this.route.page === "browser" && this.route.sessionId !== null
      ? (this.route as { messageOrdinal?: number }).messageOrdinal ?? null
      : null,
  );

  private onPopState = () => {
    this.route = parseUrl(window.location.pathname, window.location.hash);
  };

  constructor() {
    window.addEventListener("popstate", this.onPopState);
  }

  navigate(route: Route): void {
    const url = buildUrl(route);
    history.pushState({}, "", url);
    this.route = route;
  }

  replace(route: Route): void {
    const url = buildUrl(route);
    history.replaceState({}, "", url);
    this.route = route;
  }

  destroy(): void {
    window.removeEventListener("popstate", this.onPopState);
  }
}

export const router = new RouterStore();
