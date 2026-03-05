// ABOUTME: Tests for secret detection and masking pipeline stage.
// ABOUTME: Covers all high-confidence secret patterns and MaskResult reporting.
package secrets

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskSecretsCleanText(t *testing.T) {
	result := MaskSecrets("This is clean text with no secrets at all")
	assert.Equal(t, 0, result.SecretCount)
	assert.Empty(t, result.Patterns)
	assert.Equal(t, "This is clean text with no secrets at all", result.Masked)
}

func TestMaskSecretsAWSAccessKey(t *testing.T) {
	input := "My AWS key is AKIAIOSFODNN7EXAMPLE right here"
	result := MaskSecrets(input)
	assert.Greater(t, result.SecretCount, 0)
	assert.Contains(t, result.Patterns, "aws-access-key")
	assert.Contains(t, result.Masked, "[REDACTED:aws-access-key]")
	assert.NotContains(t, result.Masked, "AKIAIOSFODNN7EXAMPLE")
}

func TestMaskSecretsGitHubPAT(t *testing.T) {
	input := "token: ghp_abcdefghijklmnopqrstuvwxyz0123456789"
	result := MaskSecrets(input)
	assert.Greater(t, result.SecretCount, 0)
	assert.Contains(t, result.Patterns, "github-pat")
	assert.Contains(t, result.Masked, "[REDACTED:github-pat]")
	assert.NotContains(t, result.Masked, "ghp_")
}

func TestMaskSecretsGitHubOAuth(t *testing.T) {
	input := "gho_abcdefghijklmnopqrstuvwxyz0123456789"
	result := MaskSecrets(input)
	assert.Greater(t, result.SecretCount, 0)
	assert.Contains(t, result.Patterns, "github-oauth")
	assert.Contains(t, result.Masked, "[REDACTED:github-oauth]")
}

func TestMaskSecretsGitHubApp(t *testing.T) {
	input := "ghu_abcdefghijklmnopqrstuvwxyz0123456789"
	result := MaskSecrets(input)
	assert.Greater(t, result.SecretCount, 0)
	assert.Contains(t, result.Patterns, "github-app")
	assert.Contains(t, result.Masked, "[REDACTED:github-app]")
}

func TestMaskSecretsGitHubRefresh(t *testing.T) {
	input := "ghr_abcdefghijklmnopqrstuvwxyz0123456789"
	result := MaskSecrets(input)
	assert.Greater(t, result.SecretCount, 0)
	assert.Contains(t, result.Patterns, "github-refresh")
	assert.Contains(t, result.Masked, "[REDACTED:github-refresh]")
}

func TestMaskSecretsSlackToken(t *testing.T) {
	input := "xoxb-123456789012-123456789012-abcdefghijklmnopqrstuvwx"
	result := MaskSecrets(input)
	assert.Greater(t, result.SecretCount, 0)
	assert.Contains(t, result.Patterns, "slack-token")
	assert.Contains(t, result.Masked, "[REDACTED:slack-token]")
}

func TestMaskSecretsJWT(t *testing.T) {
	// A realistic JWT structure (header.payload.signature)
	input := "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	result := MaskSecrets(input)
	assert.Greater(t, result.SecretCount, 0)
	assert.Contains(t, result.Patterns, "jwt")
	assert.Contains(t, result.Masked, "[REDACTED:jwt]")
}

func TestMaskSecretsPrivateKey(t *testing.T) {
	input := "-----BEGIN RSA PRIVATE KEY-----\nMIIBogIBAAJBALRi..."
	result := MaskSecrets(input)
	assert.Greater(t, result.SecretCount, 0)
	assert.Contains(t, result.Patterns, "private-key")
	assert.Contains(t, result.Masked, "[REDACTED:private-key]")
}

func TestMaskSecretsPrivateKeyDSA(t *testing.T) {
	input := "-----BEGIN DSA PRIVATE KEY-----\nMIIBogIBAAJBALRi..."
	result := MaskSecrets(input)
	assert.Greater(t, result.SecretCount, 0)
	assert.Contains(t, result.Patterns, "private-key")
}

func TestMaskSecretsPrivateKeyEC(t *testing.T) {
	input := "-----BEGIN EC PRIVATE KEY-----\nMIIBogIBAAJBALRi..."
	result := MaskSecrets(input)
	assert.Greater(t, result.SecretCount, 0)
	assert.Contains(t, result.Patterns, "private-key")
}

func TestMaskSecretsPrivateKeyGeneric(t *testing.T) {
	input := "-----BEGIN PRIVATE KEY-----\nMIIBogIBAAJBALRi..."
	result := MaskSecrets(input)
	assert.Greater(t, result.SecretCount, 0)
	assert.Contains(t, result.Patterns, "private-key")
}

func TestMaskSecretsGenericAPIKey(t *testing.T) {
	input := `config: api_key = "abcdefghijklmnopqrstuvwxyz1234567890"`
	result := MaskSecrets(input)
	assert.Greater(t, result.SecretCount, 0)
	assert.Contains(t, result.Patterns, "generic-api-key")
	assert.Contains(t, result.Masked, "[REDACTED:generic-api-key]")
}

func TestMaskSecretsGenericSecret(t *testing.T) {
	input := `password: "super_secret_password_12345678"`
	result := MaskSecrets(input)
	assert.Greater(t, result.SecretCount, 0)
	assert.Contains(t, result.Patterns, "generic-secret")
	assert.Contains(t, result.Masked, "[REDACTED:generic-secret]")
}

func TestMaskSecretsOpenAIKey(t *testing.T) {
	input := "sk-abc12345678901234567T3BlbkFJabc12345678901234567"
	result := MaskSecrets(input)
	assert.Greater(t, result.SecretCount, 0)
	assert.Contains(t, result.Patterns, "openai-key")
	assert.Contains(t, result.Masked, "[REDACTED:openai-key]")
}

func TestMaskSecretsAnthropicKey(t *testing.T) {
	input := "sk-ant-abcdefghijklmnopqrstuvwxyz01234567890123456789"
	result := MaskSecrets(input)
	assert.Greater(t, result.SecretCount, 0)
	assert.Contains(t, result.Patterns, "anthropic-key")
	assert.Contains(t, result.Masked, "[REDACTED:anthropic-key]")
}

func TestMaskSecretsDBConnectionPostgres(t *testing.T) {
	input := "dsn: postgres://admin:secretpass@db.example.com:5432/mydb"
	result := MaskSecrets(input)
	assert.Greater(t, result.SecretCount, 0)
	assert.Contains(t, result.Patterns, "db-connection")
	assert.Contains(t, result.Masked, "[REDACTED:db-connection]")
}

func TestMaskSecretsDBConnectionMySQL(t *testing.T) {
	input := "dsn: mysql://root:pass@localhost/db"
	result := MaskSecrets(input)
	assert.Greater(t, result.SecretCount, 0)
	assert.Contains(t, result.Patterns, "db-connection")
}

func TestMaskSecretsDBConnectionMongoDB(t *testing.T) {
	input := "dsn: mongodb://user:pass@cluster.example.com/mydb"
	result := MaskSecrets(input)
	assert.Greater(t, result.SecretCount, 0)
	assert.Contains(t, result.Patterns, "db-connection")
}

func TestMaskSecretsMultipleSecrets(t *testing.T) {
	input := "AWS: AKIAIOSFODNN7EXAMPLE and GitHub: ghp_abcdefghijklmnopqrstuvwxyz0123456789"
	result := MaskSecrets(input)
	assert.Equal(t, 2, result.SecretCount)
	assert.Contains(t, result.Patterns, "aws-access-key")
	assert.Contains(t, result.Patterns, "github-pat")
	assert.NotContains(t, result.Masked, "AKIAIOSFODNN7EXAMPLE")
	assert.NotContains(t, result.Masked, "ghp_")
}

func TestMaskSecretsPatternsAreUnique(t *testing.T) {
	// Two occurrences of the same pattern should count as 2 but pattern name listed once
	input := "ghp_abcdefghijklmnopqrstuvwxyz0123456789 and ghp_zyxwvutsrqponmlkjihgfedcba0123456789"
	result := MaskSecrets(input)
	assert.Equal(t, 2, result.SecretCount)
	// Pattern name should appear only once in the list
	count := 0
	for _, p := range result.Patterns {
		if p == "github-pat" {
			count++
		}
	}
	assert.Equal(t, 1, count, "pattern name should be listed once even with multiple matches")
}

func TestMaskSecretsPreservesContext(t *testing.T) {
	input := "Before AKIAIOSFODNN7EXAMPLE after"
	result := MaskSecrets(input)
	assert.True(t, strings.HasPrefix(result.Masked, "Before "))
	assert.True(t, strings.HasSuffix(result.Masked, " after"))
}
