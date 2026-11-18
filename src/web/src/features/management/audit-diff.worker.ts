import { type AuditDiffInput, buildAuditDiff } from '@/features/management/audit-diff';

self.onmessage = (event: MessageEvent<AuditDiffInput>) => {
  try {
    self.postMessage(buildAuditDiff(event.data));
  } catch {
    self.postMessage({ status: 'timeout', hunks: [], additions: 0, removals: 0 });
  }
};
