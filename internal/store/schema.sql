-- ABOUTME: ClickHouse schema for agentlore
-- ABOUTME: Canonical DDL for sessions, messages, tool_calls, git_links tables

CREATE TABLE IF NOT EXISTS sessions (
    org_id              String DEFAULT '',
    id                  String,
    user_id             String DEFAULT '',
    user_name           String DEFAULT '',
    project_id          String DEFAULT '',
    project_name        String DEFAULT '',
    project_path        String DEFAULT '',
    agent_type          String DEFAULT '',
    first_message       String DEFAULT '',
    started_at          Nullable(DateTime64(3)),
    ended_at            Nullable(DateTime64(3)),
    message_count       UInt32 DEFAULT 0,
    user_message_count  UInt32 DEFAULT 0,
    parent_session_id   String DEFAULT '',
    relationship_type   String DEFAULT '',
    machine             String DEFAULT '',
    source_created_at   String DEFAULT '',
    display_name        String DEFAULT '',
    total_output_tokens UInt32 DEFAULT 0,
    peak_context_tokens UInt32 DEFAULT 0,
    _version            UInt64 DEFAULT toUnixTimestamp64Milli(now64(3))
) ENGINE = ReplacingMergeTree(_version)
PARTITION BY org_id
ORDER BY (org_id, id);

CREATE TABLE IF NOT EXISTS messages (
    org_id          String DEFAULT '',
    session_id      String,
    ordinal         UInt32,
    role            String,
    content         String DEFAULT '',
    timestamp       Nullable(DateTime64(3)),
    has_thinking    Bool DEFAULT false,
    has_tool_use    Bool DEFAULT false,
    content_length  UInt32 DEFAULT 0,
    model           String DEFAULT '',
    token_usage     String DEFAULT '',
    context_tokens  UInt32 DEFAULT 0,
    output_tokens   UInt32 DEFAULT 0,
    _version        UInt64 DEFAULT toUnixTimestamp64Milli(now64(3))
) ENGINE = ReplacingMergeTree(_version)
PARTITION BY org_id
ORDER BY (org_id, session_id, ordinal);

CREATE TABLE IF NOT EXISTS tool_calls (
    org_id              String DEFAULT '',
    session_id          String,
    message_ordinal     UInt32,
    tool_use_id         String DEFAULT '',
    tool_name           String DEFAULT '',
    tool_category       String DEFAULT '',
    input_json          String DEFAULT '',
    skill_name          String DEFAULT '',
    result_content      String DEFAULT '',
    result_content_length Nullable(UInt32),
    subagent_session_id String DEFAULT '',
    _version            UInt64 DEFAULT toUnixTimestamp64Milli(now64(3))
) ENGINE = ReplacingMergeTree(_version)
PARTITION BY org_id
ORDER BY (org_id, session_id, message_ordinal, tool_use_id);

CREATE TABLE IF NOT EXISTS git_links (
    org_id              String DEFAULT '',
    session_id          String,
    user_id             String DEFAULT '',
    message_ordinal     UInt32 DEFAULT 0,
    commit_sha          String DEFAULT '',
    pr_url              String DEFAULT '',
    link_type           String DEFAULT '',
    confidence          String DEFAULT '',
    detected_at         DateTime64(3) DEFAULT now64(3),
    _version            UInt64 DEFAULT toUnixTimestamp64Milli(now64(3))
) ENGINE = ReplacingMergeTree(_version)
PARTITION BY org_id
ORDER BY (org_id, session_id, commit_sha, pr_url);

CREATE TABLE IF NOT EXISTS session_stars (
    org_id      String DEFAULT '',
    session_id  String,
    user_id     String DEFAULT '',
    created_at  DateTime64(3) DEFAULT now64(3),
    _version    UInt64 DEFAULT toUnixTimestamp64Milli(now64(3))
) ENGINE = ReplacingMergeTree(_version)
PARTITION BY org_id
ORDER BY (org_id, session_id, user_id);

CREATE TABLE IF NOT EXISTS message_pins (
    org_id          String DEFAULT '',
    session_id      String,
    message_ordinal UInt32,
    user_id         String DEFAULT '',
    note            String DEFAULT '',
    created_at      DateTime64(3) DEFAULT now64(3),
    _version        UInt64 DEFAULT toUnixTimestamp64Milli(now64(3))
) ENGINE = ReplacingMergeTree(_version)
PARTITION BY org_id
ORDER BY (org_id, session_id, message_ordinal, user_id);

CREATE TABLE IF NOT EXISTS session_deletes (
    org_id      String DEFAULT '',
    session_id  String,
    user_id     String DEFAULT '',
    created_at  DateTime64(3) DEFAULT now64(3),
    _version    UInt64 DEFAULT toUnixTimestamp64Milli(now64(3))
) ENGINE = ReplacingMergeTree(_version)
PARTITION BY org_id
ORDER BY (org_id, session_id, user_id);
