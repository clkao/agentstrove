# Session Permalinks & Deep Links

## Status: In Progress

## Goal

Make every session and message in the UI directly linkable via URL. Users can share a conversation link, bookmark it, or deep-link to a specific message. Browser back/forward navigation works as expected.

## URL Scheme

| URL | View |
|---|---|
| `/` | Session browser (no session selected) |
| `/sessions/{id}` | Session selected, messages loaded |
| `/sessions/{id}#msg-{ordinal}` | Session + scroll to message |
| `/analytics` | Analytics dashboard |

### Design Decisions

- **History API routing** — Clean URLs, no hash prefix. The Go backend SPA fallback already rewrites non-file, non-API paths to `index.html`.
- **Session ID only in path** — Session IDs are globally unique. No org/project in the URL (fragile on rename, empty in self-hosted mode). The session detail header shows all context.
- **Fragment for message anchors** — `#msg-{ordinal}` is semantically correct (scroll-to-element). No query params or path segments needed.
- **No filters in URL** — Permalinks encode the session, not the sidebar state. Filter state is ephemeral and belongs to the viewer.
- **No search in URL** — Search is interactive and ephemeral. Can be added later if needed.
- **No router library** — The routing surface is small (2 pages + optional session + optional fragment). Hand-rolled parser is simpler and dependency-free.

## Implementation

### 1. Router module (`frontend/src/lib/stores/router.svelte.ts`)

Reactive store that owns the URL ↔ state contract:

```typescript
type Route =
  | { page: "browser"; sessionId: null }
  | { page: "browser"; sessionId: string; messageOrdinal?: number }
  | { page: "analytics" };
```

- `parseUrl()`: `window.location.pathname` + `hash` → `Route`
- `buildUrl(route)`: `Route` → URL string
- `navigate(route)`: calls `history.pushState` + updates reactive state
- Listens to `popstate` for back/forward
- Exported as singleton `router`

### 2. App.svelte integration

- On mount: parse URL → set initial route
- Route determines which page component renders
- Replaces current `hashchange` listener with `popstate`

### 3. Session selection via links

`SessionItem` and `SearchResultItem` become `<a>` tags with `href` attributes:
- `SessionItem`: `href="/sessions/{id}"`
- `SearchResultItem`: `href="/sessions/{id}#msg-{ordinal}"`
- `onclick` calls `event.preventDefault()` + `router.navigate(...)` for SPA behavior
- Right-click → Copy Link, middle-click → new tab work for free

### 4. Deep link loading

When navigating to `/sessions/{id}` (fresh page load or SPA navigation):

1. Router parses URL → `{ page: "browser", sessionId: "abc123" }`
2. `sessions.selectSession(id)` updates active session
3. `messages.load(id)` fetches messages
4. If session not in sidebar list, fetch it via `GET /api/v1/sessions/{id}` so the detail panel header has metadata
5. If `#msg-{ordinal}` fragment present, set `messages.targetOrdinal` before load — existing `$effect` in DetailPanel handles scroll

### 5. Navigation.svelte update

Replace `<a href="#/">` and `<a href="#/analytics">` with History API links:
- `<a href="/">` and `<a href="/analytics">`
- `onclick` calls `router.navigate(...)` for SPA navigation

### 6. Browser back/forward

- `router` listens to `popstate` event
- On popstate: re-parse URL → update route state → trigger appropriate loads
- `router.navigate()` uses `pushState` (not `replaceState`) so each navigation creates a history entry

## Files Changed

- `frontend/src/lib/stores/router.svelte.ts` — **new**: URL ↔ state router
- `frontend/src/App.svelte` — replace hash routing with router
- `frontend/src/lib/components/layout/Navigation.svelte` — update links
- `frontend/src/lib/components/sidebar/SessionItem.svelte` — `<a>` with href
- `frontend/src/lib/components/sidebar/SearchResultItem.svelte` — `<a>` with href + ordinal fragment
- `frontend/src/lib/stores/sessions.svelte.ts` — handle deep-linked session not in list
- `frontend/src/lib/components/layout/DetailPanel.svelte` — handle fragment-based scroll target

## Not In Scope

- Filter state in URLs
- Search state in URLs
- Org/project hierarchy in URLs
- Analytics deep links (specific chart state)
