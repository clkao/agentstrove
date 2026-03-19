// ABOUTME: Display formatting helpers for time, text, tokens, and model names.
// ABOUTME: Provides relative time, timestamp, token count, model name, and string utilities.

const MINUTE = 60;
const HOUR = 3600;
const DAY = 86400;
const WEEK = 604800;

/** Formats an ISO timestamp as a human-friendly relative time */
export function formatRelativeTime(
  isoString: string | null | undefined,
): string {
  if (!isoString) return "\u2014";

  const date = new Date(isoString);
  const diffSec = Math.floor((Date.now() - date.getTime()) / 1000);

  if (diffSec < MINUTE) return "just now";
  if (diffSec < HOUR) return `${Math.floor(diffSec / MINUTE)}m ago`;
  if (diffSec < DAY) return `${Math.floor(diffSec / HOUR)}h ago`;
  if (diffSec < WEEK) return `${Math.floor(diffSec / DAY)}d ago`;

  return date.toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
  });
}

/** Formats an ISO timestamp as a readable date/time string */
export function formatTimestamp(
  isoString: string | null | undefined,
): string {
  if (!isoString) return "\u2014";
  const d = new Date(isoString);
  return d.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

/** Truncates a string with ellipsis */
export function truncate(s: string, maxLen: number): string {
  if (s.length <= maxLen) return s;
  return s.slice(0, maxLen - 1) + "\u2026";
}

/** Formats an agent name for display */
export function formatAgentName(
  agent: string | null | undefined,
): string {
  if (!agent) return "Unknown";
  return agent.charAt(0).toUpperCase() + agent.slice(1);
}

/** Formats a number with commas */
export function formatNumber(n: number): string {
  return n.toLocaleString();
}

/** Formats a token count as a compact string (e.g. 1234 → "1.2k", 1500000 → "1.5M") */
export function formatTokenCount(n: number): string {
  if (n < 1000) return String(n);
  if (n < 1_000_000) {
    const k = n / 1000;
    return k % 1 === 0 ? `${k}k` : `${parseFloat(k.toFixed(1))}k`;
  }
  const m = n / 1_000_000;
  return m % 1 === 0 ? `${m}M` : `${parseFloat(m.toFixed(1))}M`;
}

/** Strips "claude-" prefix and trailing date suffix from model identifiers */
export function formatModelName(model: string): string {
  if (!model) return '';
  if (!model.startsWith('claude-')) return model;
  let name = model.slice('claude-'.length);
  // Strip trailing date suffix (8+ digit number like 20250514)
  name = name.replace(/-\d{8,}$/, '');
  return name;
}
