// ABOUTME: Secret detection and masking pipeline stage for sync pipeline.
// ABOUTME: Detects credentials/tokens via regex and replaces with [REDACTED:pattern_name].
package secrets

import (
	"regexp"
)

// MaskResult holds the output of secret masking: masked text, count, and pattern names.
type MaskResult struct {
	Masked      string
	SecretCount int
	Patterns    []string
}

type secretPattern struct {
	Name    string
	Pattern *regexp.Regexp
}

// Patterns derived from gitleaks. High-confidence patterns that prioritize
// catching secrets over avoiding false positives (locked decision).
var secretPatterns = []secretPattern{
	{"aws-access-key", regexp.MustCompile(`\b((?:A3T[A-Z0-9]|AKIA|ASIA|ABIA|ACCA)[A-Z2-7]{16})\b`)},
	{"github-pat", regexp.MustCompile(`ghp_[0-9a-zA-Z]{36}`)},
	{"github-oauth", regexp.MustCompile(`gho_[0-9a-zA-Z]{36}`)},
	{"github-app", regexp.MustCompile(`(?:ghu|ghs)_[0-9a-zA-Z]{36}`)},
	{"github-refresh", regexp.MustCompile(`ghr_[0-9a-zA-Z]{36}`)},
	{"slack-token", regexp.MustCompile(`xox[pboa]-[0-9]{12}-[0-9]{12}-[0-9a-zA-Z]{24}`)},
	{"jwt", regexp.MustCompile(`\b(ey[a-zA-Z0-9]{17,}\.ey[a-zA-Z0-9/\\_-]{17,}\.(?:[a-zA-Z0-9/\\_-]{10,}={0,2})?)`)},
	{"private-key", regexp.MustCompile(`-----BEGIN (?:RSA |DSA |EC |PGP )?PRIVATE KEY`)},
	{"generic-api-key", regexp.MustCompile(`(?i)(?:api[_-]?key|apikey)\s*[:=]\s*['"]([a-zA-Z0-9/+]{32,})['"]`)},
	{"generic-secret", regexp.MustCompile(`(?i)(?:secret|token|password|passwd|pwd)\s*[:=]\s*['"]([^\s'"]{16,})['"]`)},
	{"openai-key", regexp.MustCompile(`sk-[a-zA-Z0-9]{20}T3BlbkFJ[a-zA-Z0-9]{20}`)},
	{"anthropic-key", regexp.MustCompile(`sk-ant-[a-zA-Z0-9-]{40,}`)},
	{"db-connection", regexp.MustCompile(`(?i)(?:postgres|mysql|mongodb)://[^:]+:[^@]+@[^\s]+`)},
}

// MaskSecrets applies all secret patterns to the content, replacing matches
// with [REDACTED:pattern_name]. Returns the masked text, count of secrets found,
// and a deduplicated list of which pattern names matched.
func MaskSecrets(content string) MaskResult {
	result := MaskResult{Masked: content}
	seen := make(map[string]bool)

	for _, sp := range secretPatterns {
		matches := sp.Pattern.FindAllStringIndex(result.Masked, -1)
		if len(matches) == 0 {
			continue
		}

		result.SecretCount += len(matches)
		if !seen[sp.Name] {
			seen[sp.Name] = true
			result.Patterns = append(result.Patterns, sp.Name)
		}

		result.Masked = sp.Pattern.ReplaceAllString(result.Masked, "[REDACTED:"+sp.Name+"]")
	}

	return result
}
