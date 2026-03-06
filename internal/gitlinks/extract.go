// ABOUTME: Extracts git commit SHAs and PR URLs from Bash tool call results.
// ABOUTME: Pure function taking tool calls, returning discovered git links with confidence scores.
package gitlinks

import (
	"github.com/clkao/agentlore/internal/store"
)

// ExtractGitLinks scans Bash tool calls' result content for git commit SHAs
// and PR URLs, returning discovered links with confidence scores.
func ExtractGitLinks(toolCalls []store.ToolCall, messages []store.Message) []store.GitLink {
	var links []store.GitLink
	seen := make(map[string]bool)

	for _, tc := range toolCalls {
		if tc.ToolName != "Bash" || tc.ResultContent == "" {
			continue
		}

		isCommit := isGitCommitCommand(tc.InputJSON)
		isPRCreate := isGHPRCreateCommand(tc.InputJSON)

		// Extract commit SHA from git commit output
		if isCommit {
			matches := commitOutputRE.FindStringSubmatch(tc.ResultContent)
			if len(matches) >= 2 {
				sha := matches[1]
				if !seen[sha] {
					seen[sha] = true
					links = append(links, store.GitLink{
						SessionID:      tc.SessionID,
						MessageOrdinal: tc.MessageOrdinal,
						CommitSHA:      sha,
						LinkType:       "commit",
						Confidence:     "high",
					})
				}
			}
		}

		// Extract PR URL from gh pr create output (high confidence)
		if isPRCreate {
			prURL := prURLRE.FindString(tc.ResultContent)
			if prURL != "" && !seen[prURL] {
				seen[prURL] = true
				links = append(links, store.GitLink{
					SessionID:      tc.SessionID,
					MessageOrdinal: tc.MessageOrdinal,
					PRURL:          prURL,
					LinkType:       "pr",
					Confidence:     "high",
				})
			}
		}

		// Scan for PR URLs in non-gh-pr-create Bash output (medium confidence)
		if !isPRCreate {
			prURL := prURLRE.FindString(tc.ResultContent)
			if prURL != "" && !seen[prURL] {
				seen[prURL] = true
				links = append(links, store.GitLink{
					SessionID:      tc.SessionID,
					MessageOrdinal: tc.MessageOrdinal,
					PRURL:          prURL,
					LinkType:       "pr",
					Confidence:     "medium",
				})
			}
		}
	}

	return links
}
