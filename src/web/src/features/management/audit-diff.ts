import { structuredPatch } from 'diff';

import { formatAuditJSON, parseAuditJSON } from '@/features/management/audit-json';

export interface AuditDiffInput {
  before: string;
  after: string;
  expanded: boolean;
  timeoutMs: number;
}
export interface AuditDiffHunk {
  oldStart: number;
  oldLines: number;
  newStart: number;
  newLines: number;
  lines: string[];
}
export interface AuditDiffResult {
  status: 'ready' | 'timeout';
  hunks: AuditDiffHunk[];
  additions: number;
  removals: number;
}

export function buildAuditDiff(input: AuditDiffInput): AuditDiffResult {
  const before: unknown = parseAuditJSON(input.before);
  const after: unknown = parseAuditJSON(input.after);
  const left = before === null ? '' : formatAuditJSON(before) + '\n';
  const right = after === null ? '' : formatAuditJSON(after) + '\n';
  const patch = structuredPatch('变更前', '变更后', left, right, '', '', {
    context: input.expanded ? Number.MAX_SAFE_INTEGER : 3,
    timeout: input.timeoutMs,
  });
  if (!patch) return { status: 'timeout', hunks: [], additions: 0, removals: 0 };
  let additions = 0;
  let removals = 0;
  for (const hunk of patch.hunks) {
    for (const line of hunk.lines) {
      if (line.startsWith('+')) additions += 1;
      if (line.startsWith('-')) removals += 1;
    }
  }
  return { status: 'ready', hunks: patch.hunks, additions, removals };
}
