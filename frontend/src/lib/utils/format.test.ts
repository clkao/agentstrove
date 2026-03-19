// ABOUTME: Tests for token count and model name formatting helpers.
// ABOUTME: Covers edge cases for abbreviation thresholds and Claude model name stripping.

import { describe, it, expect } from 'vitest';
import { formatTokenCount, formatModelName } from './format';

describe('formatTokenCount', () => {
  it('returns "0" for zero', () => {
    expect(formatTokenCount(0)).toBe('0');
  });

  it('returns raw number below 1000', () => {
    expect(formatTokenCount(999)).toBe('999');
  });

  it('formats thousands with one decimal', () => {
    expect(formatTokenCount(1234)).toBe('1.2k');
  });

  it('drops decimal when it would be zero', () => {
    expect(formatTokenCount(5000)).toBe('5k');
  });

  it('formats large thousands without decimal', () => {
    expect(formatTokenCount(145000)).toBe('145k');
  });

  it('formats millions with one decimal', () => {
    expect(formatTokenCount(1500000)).toBe('1.5M');
  });

  it('drops decimal for even millions', () => {
    expect(formatTokenCount(2000000)).toBe('2M');
  });

  it('handles exact 1000', () => {
    expect(formatTokenCount(1000)).toBe('1k');
  });

  it('handles exact 1000000', () => {
    expect(formatTokenCount(1000000)).toBe('1M');
  });
});

describe('formatModelName', () => {
  it('returns empty string for empty input', () => {
    expect(formatModelName('')).toBe('');
  });

  it('strips "claude-" prefix and date suffix', () => {
    expect(formatModelName('claude-opus-4-20250514')).toBe('opus-4');
  });

  it('strips "claude-" prefix when no date suffix', () => {
    expect(formatModelName('claude-sonnet-4-6')).toBe('sonnet-4-6');
  });

  it('keeps non-Claude models as-is', () => {
    expect(formatModelName('gemini-3-pro-preview')).toBe('gemini-3-pro-preview');
  });

  it('handles Claude model with longer date suffix', () => {
    expect(formatModelName('claude-sonnet-4-20250514')).toBe('sonnet-4');
  });

  it('handles Claude model with version and date', () => {
    expect(formatModelName('claude-haiku-3-5-20241022')).toBe('haiku-3-5');
  });
});
