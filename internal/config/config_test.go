// ABOUTME: Tests for config loading, user identity resolution, and project identity resolution.
// ABOUTME: Uses temp files and temp git repos to exercise all code paths.
package config

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := &Config{
		ClickHouseAddr:   "localhost:9440",
		UserName:         "Alice",
		UserID:           "alice@example.com",
		AgentsviewDBPath: "/tmp/sessions.db",
		DataDir:          "/tmp/data",
		ServerPort:       8080,
	}

	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	loaded, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, "localhost:9440", loaded.ClickHouseAddr)
	assert.Equal(t, "Alice", loaded.UserName)
	assert.Equal(t, "alice@example.com", loaded.UserID)
	assert.Equal(t, "/tmp/sessions.db", loaded.AgentsviewDBPath)
	assert.Equal(t, "/tmp/data", loaded.DataDir)
	assert.Equal(t, 8080, loaded.ServerPort)
}

func TestLoadMissing(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.json")
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "", cfg.ClickHouseAddr)
	assert.Equal(t, 0, cfg.ServerPort)
}

func TestLoadInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0o644))

	_, err := Load(path)
	assert.Error(t, err)
}

func TestResolvedUserIdentity_FromConfig(t *testing.T) {
	cfg := &Config{
		UserID:   "bob@example.com",
		UserName: "Bob",
	}

	userID, userName := cfg.ResolvedUserIdentity()
	assert.Equal(t, "bob@example.com", userID)
	assert.Equal(t, "Bob", userName)
}

func TestResolvedUserIdentity_FallsBackToGit(t *testing.T) {
	cfg := &Config{}

	// This test relies on git being configured. If DetectUserIdentity fails,
	// we get empty strings — which is acceptable behaviour.
	userID, userName := cfg.ResolvedUserIdentity()

	gitName, gitEmail, err := DetectUserIdentity()
	if err != nil {
		// git not configured — both should be empty
		assert.Equal(t, "", userID)
		assert.Equal(t, "", userName)
	} else {
		assert.Equal(t, gitEmail, userID)
		assert.Equal(t, gitName, userName)
	}
}

func TestResolvedUserIdentity_PartialOverride(t *testing.T) {
	cfg := &Config{
		UserID: "carol@example.com",
		// UserName intentionally left blank — should fall back to git
	}

	userID, _ := cfg.ResolvedUserIdentity()
	assert.Equal(t, "carol@example.com", userID)
}

func initTempGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmds := [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@example.com"},
		{"git", "-C", dir, "config", "user.name", "Test User"},
	}
	for _, args := range cmds {
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		require.NoError(t, err, "command %v failed: %s", args, out)
	}
}

func TestResolveProjectIdentity_WithRemote(t *testing.T) {
	dir := t.TempDir()
	initTempGitRepo(t, dir)

	out, err := exec.Command("git", "-C", dir, "remote", "add", "origin", "https://github.com/org/myrepo.git").CombinedOutput()
	require.NoError(t, err, string(out))

	projectID, projectName := ResolveProjectIdentity(dir)
	assert.Equal(t, "https://github.com/org/myrepo.git", projectID)
	assert.Equal(t, "myrepo", projectName)
}

func TestResolveProjectIdentity_SSHRemote(t *testing.T) {
	dir := t.TempDir()
	initTempGitRepo(t, dir)

	out, err := exec.Command("git", "-C", dir, "remote", "add", "origin", "git@github.com:org/myrepo.git").CombinedOutput()
	require.NoError(t, err, string(out))

	projectID, projectName := ResolveProjectIdentity(dir)
	assert.Equal(t, "git@github.com:org/myrepo.git", projectID)
	assert.Equal(t, "myrepo", projectName)
}

func TestResolveProjectIdentity_NoRemote(t *testing.T) {
	dir := t.TempDir()
	initTempGitRepo(t, dir)

	// No remote added
	projectID, projectName := ResolveProjectIdentity(dir)
	assert.Equal(t, "", projectID)
	assert.Equal(t, filepath.Base(dir), projectName)
}

func TestResolveProjectIdentity_NotAGitRepo(t *testing.T) {
	dir := t.TempDir()

	projectID, projectName := ResolveProjectIdentity(dir)
	assert.Equal(t, "", projectID)
	assert.Equal(t, filepath.Base(dir), projectName)
}
