// ABOUTME: Domain types and storage interfaces for agentstrove.
// ABOUTME: Store handles writes; ReadStore handles reads — they are separate interfaces.

package store

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrInvalidCursor = errors.New("invalid cursor")
)

type Session struct {
	OrgID            string     `json:"org_id"`
	ID               string     `json:"id"`
	UserID           string     `json:"user_id"`
	UserName         string     `json:"user_name"`
	ProjectID        string     `json:"project_id"`
	ProjectName      string     `json:"project_name"`
	ProjectPath      string     `json:"project_path"`
	Machine          string     `json:"machine"`
	AgentType        string     `json:"agent_type"`
	FirstMessage     string     `json:"first_message"`
	StartedAt        *time.Time `json:"started_at"`
	EndedAt          *time.Time `json:"ended_at"`
	MessageCount     int        `json:"message_count"`
	UserMessageCount int        `json:"user_message_count"`
	ParentSessionID  string     `json:"parent_session_id"`
	RelationshipType string     `json:"relationship_type"`
	SourceCreatedAt  string     `json:"source_created_at"`
	CommitCount      int        `json:"commit_count"`
}

type Message struct {
	OrgID         string     `json:"org_id"`
	SessionID     string     `json:"session_id"`
	Ordinal       int        `json:"ordinal"`
	Role          string     `json:"role"`
	Content       string     `json:"content"`
	Timestamp     *time.Time `json:"timestamp"`
	HasThinking   bool       `json:"has_thinking"`
	HasToolUse    bool       `json:"has_tool_use"`
	ContentLength int        `json:"content_length"`
}

type ToolCall struct {
	OrgID               string `json:"org_id"`
	MessageOrdinal      int    `json:"message_ordinal"`
	SessionID           string `json:"session_id"`
	ToolName            string `json:"tool_name"`
	Category            string `json:"tool_category"`
	ToolUseID           string `json:"tool_use_id"`
	InputJSON           string `json:"input_json"`
	SkillName           string `json:"skill_name"`
	ResultContentLength *int   `json:"result_content_length"`
	ResultContent       string `json:"result_content"`
	SubagentSessionID   string `json:"subagent_session_id"`
}

type GitLink struct {
	OrgID          string `json:"org_id"`
	SessionID      string `json:"session_id"`
	UserID         string `json:"user_id"`
	MessageOrdinal int    `json:"message_ordinal"`
	CommitSHA      string `json:"commit_sha"`
	PRURL          string `json:"pr_url"`
	LinkType       string `json:"link_type"`
	Confidence     string `json:"confidence"`
}

type GitLinkResult struct {
	SessionID      string     `json:"session_id"`
	UserID         string     `json:"user_id"`
	UserName       string     `json:"user_name"`
	ProjectID      string     `json:"project_id"`
	ProjectName    string     `json:"project_name"`
	AgentType      string     `json:"agent_type"`
	StartedAt      *time.Time `json:"started_at"`
	FirstMessage   string     `json:"first_message"`
	CommitSHA      string     `json:"commit_sha"`
	PRURL          string     `json:"pr_url"`
	LinkType       string     `json:"link_type"`
	Confidence     string     `json:"confidence"`
	MessageOrdinal int        `json:"message_ordinal"`
}

type SessionFilter struct {
	UserID           string
	ProjectID        string
	ProjectName      string
	AgentType        string
	DateFrom         string
	DateTo           string
	Cursor           string
	Limit            int
	IncludeSubagents bool
}

type SessionPage struct {
	Sessions   []Session `json:"sessions"`
	NextCursor string    `json:"next_cursor"`
	Total      int64     `json:"total"`
}

type UserInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ProjectInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

type SearchQuery struct {
	Query       string
	UserID      string
	ProjectID   string
	ProjectName string
	AgentType   string
	DateFrom  string
	DateTo    string
	Limit     int
}

type Highlight struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type SearchResult struct {
	SessionID    string     `json:"session_id"`
	Ordinal      int        `json:"ordinal"`
	Role         string     `json:"role"`
	UserID       string     `json:"user_id"`
	UserName     string     `json:"user_name"`
	ProjectName  string     `json:"project_name"`
	AgentType    string     `json:"agent_type"`
	StartedAt    *time.Time `json:"started_at"`
	FirstMessage string     `json:"first_message"`
	Snippet      string     `json:"snippet"`
	Highlights   []Highlight `json:"highlights"`
}

type SearchPage struct {
	Results []SearchResult `json:"results"`
	Total   int            `json:"total"`
}

type UserUsage struct {
	UserID       string `json:"user_id"`
	UserName     string `json:"user_name"`
	AgentType    string `json:"agent_type"`
	ProjectName  string `json:"project_name"`
	SessionCount int    `json:"session_count"`
	MessageCount int    `json:"message_count"`
	CommitCount  int    `json:"commit_count"`
}

type HeatmapCell struct {
	DayOfWeek    int `json:"day_of_week"`
	Hour         int `json:"hour"`
	SessionCount int `json:"session_count"`
}

type ToolUsageStat struct {
	ToolName   string `json:"tool_name"`
	Category   string `json:"category"`
	UsageCount int    `json:"usage_count"`
}

type DailyActivity struct {
	Date         string `json:"date"`
	SessionCount int    `json:"session_count"`
	MessageCount int    `json:"message_count"`
}

// Store handles write operations for agent session data.
type Store interface {
	EnsureSchema(ctx context.Context) error
	WriteSession(ctx context.Context, orgID string, session Session, messages []Message, toolCalls []ToolCall) error
	WriteBatch(ctx context.Context, orgID string, sessions []Session, messages []Message, toolCalls []ToolCall) error
	WriteGitLinks(ctx context.Context, orgID string, links []GitLink) error
	Close() error
}

// ReadStore handles read operations for agent session data.
type ReadStore interface {
	ListSessions(ctx context.Context, orgID string, filter SessionFilter) (*SessionPage, error)
	GetSession(ctx context.Context, orgID string, id string) (*Session, error)
	GetSessionMessages(ctx context.Context, orgID string, sessionID string) ([]Message, error)
	GetSessionToolCalls(ctx context.Context, orgID string, sessionID string) ([]ToolCall, error)
	ListUsers(ctx context.Context, orgID string) ([]UserInfo, error)
	ListProjects(ctx context.Context, orgID string) ([]ProjectInfo, error)
	ListAgents(ctx context.Context, orgID string) ([]string, error)
	Search(ctx context.Context, orgID string, query SearchQuery) (*SearchPage, error)
	LookupGitLinks(ctx context.Context, orgID string, sha string, prURL string) ([]GitLinkResult, error)
	UsageByUser(ctx context.Context, orgID string, dateFrom, dateTo string) ([]UserUsage, error)
	ActivityHeatmap(ctx context.Context, orgID string, dateFrom, dateTo string) ([]HeatmapCell, error)
	ToolUsageDistribution(ctx context.Context, orgID string, dateFrom, dateTo string) ([]ToolUsageStat, error)
	DailyActivity(ctx context.Context, orgID string, dateFrom, dateTo string) ([]DailyActivity, error)
	Close() error
}
