// ABOUTME: Tests for git link extraction from Bash tool calls in synced conversations.
// ABOUTME: Covers commit SHA extraction, PR URL detection, confidence scoring, and edge cases.
package gitlinks

import (
	"testing"

	"github.com/clkao/agentstrove/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeToolCall(sessionID string, ordinal int, category, inputJSON, resultContent string) store.ToolCall {
	return store.ToolCall{
		SessionID:      sessionID,
		MessageOrdinal: ordinal,
		ToolName:       "Bash",
		Category:       category,
		InputJSON:      inputJSON,
		ResultContent:  resultContent,
	}
}

func makeMessage(sessionID string, ordinal int, role, content string) store.Message {
	return store.Message{
		SessionID:     sessionID,
		Ordinal:       ordinal,
		Role:          role,
		Content:       content,
		ContentLength: len(content),
	}
}

func TestExtractGitLinks_CommitSHA(t *testing.T) {
	tcs := []store.ToolCall{
		makeToolCall("sess-1", 1, "Bash",
			`{"command":"git commit -m \"fix: something\""}`,
			"[main abc1234] fix: something\n 1 file changed, 2 insertions(+)\n"),
	}
	msgs := []store.Message{
		makeMessage("sess-1", 1, "assistant",
			"[Bash] $ git commit -m \"fix: something\""),
	}

	links := ExtractGitLinks(tcs, msgs)
	require.Len(t, links, 1)
	assert.Equal(t, "abc1234", links[0].CommitSHA)
	assert.Equal(t, "commit", links[0].LinkType)
	assert.Equal(t, "high", links[0].Confidence)
	assert.Equal(t, "sess-1", links[0].SessionID)
	assert.Equal(t, 1, links[0].MessageOrdinal)
}

func TestExtractGitLinks_RootCommit(t *testing.T) {
	tcs := []store.ToolCall{
		makeToolCall("sess-1", 1, "Bash",
			`{"command":"git commit -m \"initial\""}`,
			"[main (root-commit) abc1234] initial\n 1 file changed\n"),
	}
	msgs := []store.Message{}

	links := ExtractGitLinks(tcs, msgs)
	require.Len(t, links, 1)
	assert.Equal(t, "abc1234", links[0].CommitSHA)
}

func TestExtractGitLinks_DetachedHEAD(t *testing.T) {
	tcs := []store.ToolCall{
		makeToolCall("sess-1", 1, "Bash",
			`{"command":"git commit -m \"detached\""}`,
			"[detached HEAD abc1234] detached\n"),
	}
	msgs := []store.Message{}

	links := ExtractGitLinks(tcs, msgs)
	require.Len(t, links, 1)
	assert.Equal(t, "abc1234", links[0].CommitSHA)
}

func TestExtractGitLinks_GHPRCreate(t *testing.T) {
	tcs := []store.ToolCall{
		makeToolCall("sess-1", 2, "Bash",
			`{"command":"gh pr create --title \"feat: new\" --body \"description\""}`,
			"Creating pull request for feat/new into main...\nhttps://github.com/owner/repo/pull/42\n"),
	}
	msgs := []store.Message{}

	links := ExtractGitLinks(tcs, msgs)
	require.Len(t, links, 1)
	assert.Equal(t, "https://github.com/owner/repo/pull/42", links[0].PRURL)
	assert.Equal(t, "pr", links[0].LinkType)
	assert.Equal(t, "high", links[0].Confidence)
}

func TestExtractGitLinks_PRURLInNonGHPRCreate_MediumConfidence(t *testing.T) {
	tcs := []store.ToolCall{
		makeToolCall("sess-1", 3, "Bash",
			`{"command":"git push origin feat/new"}`,
			"remote: Create a pull request for 'feat/new' on GitHub by visiting:\nremote:   https://github.com/owner/repo/pull/43\n"),
	}
	msgs := []store.Message{}

	links := ExtractGitLinks(tcs, msgs)
	require.Len(t, links, 1)
	assert.Equal(t, "https://github.com/owner/repo/pull/43", links[0].PRURL)
	assert.Equal(t, "pr", links[0].LinkType)
	assert.Equal(t, "medium", links[0].Confidence)
}

func TestExtractGitLinks_ChainedCommands(t *testing.T) {
	tcs := []store.ToolCall{
		makeToolCall("sess-1", 1, "Bash",
			`{"command":"git add . && git commit -m \"chained\""}`,
			"[feat/branch def5678] chained\n 2 files changed\n"),
	}
	msgs := []store.Message{}

	links := ExtractGitLinks(tcs, msgs)
	require.Len(t, links, 1)
	assert.Equal(t, "def5678", links[0].CommitSHA)
	assert.Equal(t, "high", links[0].Confidence)
}

func TestExtractGitLinks_NonBashToolCall(t *testing.T) {
	tcs := []store.ToolCall{
		makeToolCall("sess-1", 1, "Read",
			`{"path":"/tmp/file.go"}`,
			"[main abc1234] some content that looks like commit output\n"),
	}
	msgs := []store.Message{}

	links := ExtractGitLinks(tcs, msgs)
	assert.Len(t, links, 0)
}

func TestExtractGitLinks_MultipleToolCallsAcrossOrdinals(t *testing.T) {
	tcs := []store.ToolCall{
		makeToolCall("sess-1", 1, "Bash",
			`{"command":"git commit -m \"first\""}`,
			"[main aaa1111] first\n 1 file changed\n"),
		makeToolCall("sess-1", 3, "Bash",
			`{"command":"git commit -m \"second\""}`,
			"[main bbb2222] second\n 1 file changed\n"),
	}
	msgs := []store.Message{}

	links := ExtractGitLinks(tcs, msgs)
	require.Len(t, links, 2)
	assert.Equal(t, "aaa1111", links[0].CommitSHA)
	assert.Equal(t, "bbb2222", links[1].CommitSHA)
}

func TestExtractGitLinks_NoMatchingToolCalls(t *testing.T) {
	tcs := []store.ToolCall{}
	msgs := []store.Message{
		makeMessage("sess-1", 1, "assistant",
			"Just some regular conversation content"),
	}

	links := ExtractGitLinks(tcs, msgs)
	assert.Len(t, links, 0)
}

func TestExtractGitLinks_DeduplicatesSameSHA(t *testing.T) {
	tcs := []store.ToolCall{
		makeToolCall("sess-1", 1, "Bash",
			`{"command":"git commit -m \"first\""}`,
			"[main abc1234] first\n"),
		makeToolCall("sess-1", 3, "Bash",
			`{"command":"git commit --amend --no-edit"}`,
			"[main abc1234] first\n"),
	}
	msgs := []store.Message{}

	links := ExtractGitLinks(tcs, msgs)
	require.Len(t, links, 1, "duplicate SHAs should be deduplicated")
	assert.Equal(t, "abc1234", links[0].CommitSHA)
}

func TestExtractGitLinks_BashToolCallWithNoGitCommand(t *testing.T) {
	tcs := []store.ToolCall{
		makeToolCall("sess-1", 1, "Bash",
			`{"command":"ls -la"}`,
			"total 48\ndrwxr-xr-x 5 user user 4096 Mar 1 10:00 .\n"),
	}
	msgs := []store.Message{}

	links := ExtractGitLinks(tcs, msgs)
	assert.Len(t, links, 0)
}

func TestExtractGitLinks_EmptyResultContent(t *testing.T) {
	// When result_content is empty (old agentsview without result storage),
	// extraction finds nothing — this is expected behavior.
	tcs := []store.ToolCall{
		makeToolCall("sess-1", 1, "Bash",
			`{"command":"git commit -m \"fix\""}`,
			""),
	}
	msgs := []store.Message{
		makeMessage("sess-1", 1, "assistant",
			"[main abc1234] fix\n 1 file changed\n"),
	}

	links := ExtractGitLinks(tcs, msgs)
	assert.Len(t, links, 0, "empty result_content should yield no links")
}
