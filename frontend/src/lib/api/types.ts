// ABOUTME: TypeScript interfaces matching Go API JSON responses.
// ABOUTME: Uses snake_case field names to match Go JSON tags exactly.

export interface Session {
  id: string;
  user_id: string;
  user_name: string;
  project_id: string;
  project_name: string;
  project_path: string;
  machine: string;
  agent_type: string;
  first_message: string | null;
  started_at: string | null;
  ended_at: string | null;
  message_count: number;
  user_message_count: number;
  parent_session_id: string;
  relationship_type: string;
  commit_count: number;
}

export interface SessionPage {
  sessions: Session[];
  next_cursor: string;
  total: number;
}

export interface MessageWithToolCalls {
  session_id: string;
  ordinal: number;
  role: string;
  content: string;
  timestamp: string | null;
  has_thinking: boolean;
  has_tool_use: boolean;
  content_length: number;
  tool_calls: ToolCall[];
}

export interface ToolCall {
  message_ordinal: number;
  session_id: string;
  tool_name: string;
  tool_category: string;
  tool_use_id: string;
  input_json: string;
  skill_name: string;
  result_content_length: number | null;
  result_content: string;
  subagent_session_id: string;
}

export interface UserInfo {
  id: string;
  name: string;
}

export interface ProjectInfo {
  id: string;
  name: string;
  path: string;
}

export interface Filters {
  user_id?: string;
  project_id?: string;
  project_name?: string;
  agent_type?: string;
  date_from?: string;
  date_to?: string;
  cursor?: string;
  limit?: number;
}

export interface Highlight {
  start: number;
  end: number;
}

export interface SearchResult {
  session_id: string;
  ordinal: number;
  role: string;
  user_id: string;
  user_name: string;
  project_name: string;
  agent_type: string;
  started_at: string | null;
  first_message: string | null;
  snippet: string;
  highlights: Highlight[];
}

export interface SearchPage {
  results: SearchResult[];
  total: number;
}

export interface GitLinkResult {
  session_id: string;
  user_name: string;
  user_id: string;
  project_id: string;
  project_name: string;
  agent_type: string;
  started_at: string | null;
  first_message: string | null;
  commit_sha: string;
  pr_url: string;
  link_type: string;
  confidence: string;
  message_ordinal: number;
}

export interface UserUsage {
  user_id: string;
  user_name: string;
  agent_type: string;
  project_name: string;
  session_count: number;
  message_count: number;
}

export interface HeatmapCell {
  day_of_week: number;
  hour: number;
  session_count: number;
}

export interface ToolUsageStat {
  tool_name: string;
  category: string;
  usage_count: number;
}
