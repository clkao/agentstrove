// ABOUTME: Configuration loading, user identity detection, and project identity resolution.
// ABOUTME: Reads JSON config file with defaults, auto-detects git identity with config override.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Config holds all daemon configuration fields.
type Config struct {
	ClickHouseAddr     string `json:"clickhouse_addr"`
	ClickHouseDatabase string `json:"clickhouse_database"`
	ClickHouseSecure   bool   `json:"clickhouse_secure"`
	ClickHouseUser     string `json:"clickhouse_user"`
	ClickHousePassword string `json:"clickhouse_password"`
	UserName           string `json:"user_name"`
	UserID             string `json:"user_id"`
	AgentsviewDBPath   string `json:"agentsview_db_path"`
	DataDir            string `json:"data_dir"`
	ServerPort         int    `json:"server_port"`
}

// DefaultAgentsviewDBPath returns the standard agentsview DB location if it exists.
// Checks ~/.agentsview/sessions.db first, then ~/.claude/agentsview/sessions.db.
func DefaultAgentsviewDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	for _, rel := range []string{
		filepath.Join(".agentsview", "sessions.db"),
		filepath.Join(".claude", "agentsview", "sessions.db"),
	} {
		p := filepath.Join(home, rel)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// DefaultDataDir returns ~/.config/agentlore/data as the default data directory.
func DefaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "agentlore", "data")
}

// Load reads configuration from a JSON file at the given path.
// Returns sensible defaults when the file does not exist.
// Environment variables override JSON values when set.
func Load(path string) (*Config, error) {
	cfg := &Config{}

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	applyEnvOverrides(cfg)
	return cfg, nil
}

// applyEnvOverrides sets config fields from environment variables.
// Env vars take precedence over JSON config values.
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("CLICKHOUSE_ADDR"); v != "" {
		cfg.ClickHouseAddr = v
	}
	if v := os.Getenv("CLICKHOUSE_DATABASE"); v != "" {
		cfg.ClickHouseDatabase = v
	}
	if v := os.Getenv("CLICKHOUSE_USER"); v != "" {
		cfg.ClickHouseUser = v
	}
	if v := os.Getenv("CLICKHOUSE_PASSWORD"); v != "" {
		cfg.ClickHousePassword = v
	}
	if v := os.Getenv("AGENTSVIEW_DB_PATH"); v != "" {
		cfg.AgentsviewDBPath = v
	}
	if v := os.Getenv("AGENTLORE_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("SERVER_PORT"); v != "" {
		var port int
		if _, err := fmt.Sscanf(v, "%d", &port); err == nil {
			cfg.ServerPort = port
		}
	}
}

// DetectUserIdentity runs git config to auto-detect the user's name and email.
// Returns userName, userID (email), error.
func DetectUserIdentity() (userName string, userID string, err error) {
	nameOut, err := exec.Command("git", "config", "user.name").Output()
	if err != nil {
		return "", "", err
	}

	emailOut, err := exec.Command("git", "config", "user.email").Output()
	if err != nil {
		return "", "", err
	}

	return strings.TrimSpace(string(nameOut)), strings.TrimSpace(string(emailOut)), nil
}

// ResolvedUserIdentity returns the user identity, preferring config overrides
// and falling back to git auto-detection. Returns userID (email), userName.
func (c *Config) ResolvedUserIdentity() (userID string, userName string) {
	userID = c.UserID
	userName = c.UserName

	if userID == "" || userName == "" {
		gitName, gitEmail, err := DetectUserIdentity()
		if err == nil {
			if userID == "" {
				userID = gitEmail
			}
			if userName == "" {
				userName = gitName
			}
		}
	}

	return userID, userName
}

// ResolveProjectIdentity returns the project ID and name for a given project path.
// projectID is the git remote origin URL; projectName is derived from the last path
// component of the remote URL without .git, or the directory basename as fallback.
// If the path is not a git repo or has no remote, returns ("", basename).
func ResolveProjectIdentity(projectPath string) (projectID string, projectName string) {
	basename := filepath.Base(projectPath)

	out, err := exec.Command("git", "-C", projectPath, "remote", "get-url", "origin").Output()
	if err != nil {
		return "", basename
	}

	remoteURL := strings.TrimSpace(string(out))
	if remoteURL == "" {
		return "", basename
	}

	projectID = remoteURL

	// Extract project name from last path component of the URL, stripping .git suffix.
	// Works for both https://github.com/org/repo.git and git@github.com:org/repo.git forms.
	last := remoteURL
	if i := strings.LastIndexAny(remoteURL, "/:"); i >= 0 {
		last = remoteURL[i+1:]
	}
	projectName = strings.TrimSuffix(last, ".git")
	if projectName == "" {
		projectName = basename
	}

	return projectID, projectName
}
