// ABOUTME: Typed fetch wrappers for all 6 API endpoints.
// ABOUTME: Uses relative paths so the Vite dev proxy and production embed both work.

import type {
  SessionPage,
  Session,
  MessageWithToolCalls,
  UserInfo,
  ProjectInfo,
  Filters,
  SearchPage,
  GitLinkResult,
  UserUsage,
  HeatmapCell,
  ToolUsageStat,
  DailyActivity,
} from "./types.js";

class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

async function fetchJSON<T>(path: string): Promise<T> {
  const res = await fetch(path);
  if (!res.ok) {
    let message = res.statusText;
    try {
      const body = await res.json();
      if (body.error) message = body.error;
    } catch {
      /* use statusText */
    }
    throw new ApiError(res.status, message);
  }
  return res.json() as Promise<T>;
}

function buildQuery(filters: Filters): string {
  const params = new URLSearchParams();
  if (filters.user_id) params.set("user_id", filters.user_id);
  if (filters.project_id) params.set("project_id", filters.project_id);
  if (filters.project_name) params.set("project_name", filters.project_name);
  if (filters.agent_type) params.set("agent_type", filters.agent_type);
  if (filters.date_from) params.set("date_from", filters.date_from);
  if (filters.date_to) params.set("date_to", filters.date_to);
  if (filters.cursor) params.set("cursor", filters.cursor);
  if (filters.limit) params.set("limit", String(filters.limit));
  const qs = params.toString();
  return qs ? `?${qs}` : "";
}

export function listSessions(filters: Filters = {}): Promise<SessionPage> {
  return fetchJSON<SessionPage>(`/api/v1/sessions${buildQuery(filters)}`);
}

export function getSession(id: string): Promise<Session> {
  return fetchJSON<Session>(`/api/v1/sessions/${encodeURIComponent(id)}`);
}

export function getSessionMessages(id: string): Promise<MessageWithToolCalls[]> {
  return fetchJSON<MessageWithToolCalls[]>(
    `/api/v1/sessions/${encodeURIComponent(id)}/messages`,
  );
}

export function listUsers(): Promise<UserInfo[]> {
  return fetchJSON<UserInfo[]>("/api/v1/users");
}

export function listProjects(): Promise<ProjectInfo[]> {
  return fetchJSON<ProjectInfo[]>("/api/v1/projects");
}

export function listAgents(): Promise<string[]> {
  return fetchJSON<string[]>("/api/v1/agents");
}

export function lookupGitLinks(sha?: string, pr?: string): Promise<GitLinkResult[]> {
  const params = new URLSearchParams();
  if (sha) params.set("sha", sha);
  if (pr) params.set("pr", pr);
  return fetchJSON<GitLinkResult[]>(`/api/v1/gitlinks?${params.toString()}`);
}

export function searchMessages(query: string, filters: Filters = {}): Promise<SearchPage> {
  const params = new URLSearchParams();
  params.set("q", query);
  if (filters.user_id) params.set("user_id", filters.user_id);
  if (filters.project_id) params.set("project_id", filters.project_id);
  if (filters.project_name) params.set("project_name", filters.project_name);
  if (filters.agent_type) params.set("agent_type", filters.agent_type);
  if (filters.date_from) params.set("date_from", filters.date_from);
  if (filters.date_to) params.set("date_to", filters.date_to);
  if (filters.limit) params.set("limit", String(filters.limit));
  return fetchJSON<SearchPage>(`/api/v1/search?${params.toString()}`);
}

export function fetchUsageOverview(dateFrom?: string, dateTo?: string, projectName?: string): Promise<UserUsage[]> {
  const params = new URLSearchParams();
  if (dateFrom) params.set("date_from", dateFrom);
  if (dateTo) params.set("date_to", dateTo);
  if (projectName) params.set("project_name", projectName);
  const qs = params.toString();
  return fetchJSON<UserUsage[]>(`/api/v1/analytics/usage${qs ? `?${qs}` : ""}`);
}

export function fetchActivityHeatmap(dateFrom?: string, dateTo?: string, projectName?: string): Promise<HeatmapCell[]> {
  const params = new URLSearchParams();
  if (dateFrom) params.set("date_from", dateFrom);
  if (dateTo) params.set("date_to", dateTo);
  if (projectName) params.set("project_name", projectName);
  const qs = params.toString();
  return fetchJSON<HeatmapCell[]>(`/api/v1/analytics/heatmap${qs ? `?${qs}` : ""}`);
}

export function fetchToolUsage(dateFrom?: string, dateTo?: string, projectName?: string): Promise<ToolUsageStat[]> {
  const params = new URLSearchParams();
  if (dateFrom) params.set("date_from", dateFrom);
  if (dateTo) params.set("date_to", dateTo);
  if (projectName) params.set("project_name", projectName);
  const qs = params.toString();
  return fetchJSON<ToolUsageStat[]>(`/api/v1/analytics/tools${qs ? `?${qs}` : ""}`);
}

export function fetchDailyActivity(dateFrom?: string, dateTo?: string, projectName?: string): Promise<DailyActivity[]> {
  const params = new URLSearchParams();
  if (dateFrom) params.set("date_from", dateFrom);
  if (dateTo) params.set("date_to", dateTo);
  if (projectName) params.set("project_name", projectName);
  const qs = params.toString();
  return fetchJSON<DailyActivity[]>(`/api/v1/analytics/daily${qs ? `?${qs}` : ""}`);
}
