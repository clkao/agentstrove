// ABOUTME: Regex patterns for detecting git commit SHAs and PR URLs in tool call output.
// ABOUTME: Provides command classification helpers for Bash tool call input_json parsing.
package gitlinks

import (
	"encoding/json"
	"regexp"
	"strings"
)

// commitOutputRE matches git commit output: [branch SHA], [branch (root-commit) SHA],
// [detached HEAD SHA], and similar variants. The branch portion allows spaces
// to handle "detached HEAD" and similar multi-word refs.
var commitOutputRE = regexp.MustCompile(
	`\[[\w/.:\- ]+(?:\([^)]+\)\s+)?([0-9a-f]{7,40})\]`,
)

// prURLRE matches GitHub PR URLs in the format https://github.com/owner/repo/pull/123.
var prURLRE = regexp.MustCompile(
	`https://github\.com/[\w.-]+/[\w.-]+/pull/\d+`,
)

// bashInput represents the JSON structure of a Bash tool call's input_json field.
type bashInput struct {
	Command string `json:"command"`
}

// isGitCommitCommand checks whether the tool call's input_json contains a git commit command.
func isGitCommitCommand(inputJSON string) bool {
	var input bashInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return false
	}
	cmd := strings.TrimSpace(input.Command)
	return strings.HasPrefix(cmd, "git commit") ||
		strings.Contains(cmd, "&& git commit") ||
		strings.Contains(cmd, "; git commit")
}

// isGHPRCreateCommand checks whether the tool call's input_json contains a gh pr create command.
func isGHPRCreateCommand(inputJSON string) bool {
	var input bashInput
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return false
	}
	cmd := strings.TrimSpace(input.Command)
	return strings.Contains(cmd, "gh pr create")
}
