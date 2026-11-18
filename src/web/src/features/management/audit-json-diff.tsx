import { useEffect, useMemo, useState } from 'react';

import { Button } from '@/components/ui/button';
import {
  type AuditDiffHunk,
  type AuditDiffInput,
  type AuditDiffResult,
  buildAuditDiff,
} from '@/features/management/audit-diff';
import { formatAuditJSON } from '@/features/management/audit-json';
import { getConfig } from '@/lib/config';
import { cn } from '@/lib/utils';

function Hunk({ hunk }: { hunk: AuditDiffHunk }) {
  let oldLine = hunk.oldStart;
  let newLine = hunk.newStart;
  return (
    <div>
      <div className="border-y border-border bg-muted/60 px-3 py-2 font-mono text-xs text-muted-foreground">
        @@ -{hunk.oldStart},{hunk.oldLines} +{hunk.newStart},{hunk.newLines} @@
      </div>
      {hunk.lines.map((line, index) => {
        const prefix = line.slice(0, 1);
        if (prefix === '\\') return null;
        const previous = prefix === '+' ? '' : String(oldLine++);
        const next = prefix === '-' ? '' : String(newLine++);
        return (
          <div
            key={index}
            className={cn(
              'audit-diff-line',
              prefix === '+' && 'bg-success text-success-foreground',
              prefix === '-' && 'bg-danger text-danger-foreground',
            )}
          >
            <span aria-hidden="true" className="px-2 text-right text-xs text-muted-foreground">
              {previous}
            </span>
            <span aria-hidden="true" className="px-2 text-right text-xs text-muted-foreground">
              {next}
            </span>
            <span aria-label={prefix === '+' ? '新增' : prefix === '-' ? '删除' : '上下文'}>
              {prefix}
            </span>
            <code>{line.slice(1)}</code>
          </div>
        );
      })}
    </div>
  );
}

export function AuditJSONDiff({ before, after }: { before: unknown; after: unknown }) {
  const [expanded, setExpanded] = useState(false);
  const input = useMemo<AuditDiffInput>(
    () => ({
      before: formatAuditJSON(before),
      after: formatAuditJSON(after),
      expanded,
      timeoutMs: getConfig().auditDiffTimeoutMs,
    }),
    [before, after, expanded],
  );
  const [completed, setCompleted] = useState<{
    input: AuditDiffInput;
    result: AuditDiffResult;
  } | null>(null);

  useEffect(() => {
    let active = true;
    const finish = (result: AuditDiffResult) => {
      if (active) setCompleted({ input, result });
    };
    const timeoutResult: AuditDiffResult = {
      status: 'timeout',
      hunks: [],
      additions: 0,
      removals: 0,
    };
    // 浏览器在独立 Worker 中计算，关闭详情或切换输入立即释放。
    if (typeof Worker !== 'undefined') {
      const worker = new Worker(new URL('./audit-diff.worker.ts', import.meta.url), {
        type: 'module',
      });
      const timer = window.setTimeout(() => {
        worker.terminate();
        finish(timeoutResult);
      }, input.timeoutMs + 1000);
      worker.onmessage = (event: MessageEvent<AuditDiffResult>) => {
        window.clearTimeout(timer);
        finish(event.data);
      };
      worker.onerror = () => {
        window.clearTimeout(timer);
        worker.terminate();
        finish(timeoutResult);
      };
      worker.postMessage(input);
      return () => {
        active = false;
        window.clearTimeout(timer);
        worker.terminate();
      };
    }
    // 无 Worker 的环境仍使用同一有界算法，完整前后 JSON 始终可查看。
    const timer = window.setTimeout(() => {
      try {
        finish(buildAuditDiff(input));
      } catch {
        finish(timeoutResult);
      }
    }, 0);
    return () => {
      active = false;
      window.clearTimeout(timer);
    };
  }, [input]);

  const result = completed?.input === input ? completed.result : null;
  if (!result)
    return (
      <p role="status" className="text-xs text-muted-foreground">
        正在计算差异…
      </p>
    );
  if (result.status === 'timeout') {
    return (
      <p className="text-sm text-muted-foreground">
        差异计算未完成，可切换查看完整的变更前、变更后 JSON。
      </p>
    );
  }
  if (result.hunks.length === 0) {
    return <p className="text-sm text-muted-foreground">内容无变化</p>;
  }
  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-xs tabular-nums">
          <span className="text-success-foreground">+{result.additions}</span>{' '}
          <span className="text-danger-foreground">−{result.removals}</span>
        </p>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => {
            setExpanded(!expanded);
          }}
        >
          {expanded ? '仅显示变更上下文' : '展开未变化内容'}
        </Button>
      </div>
      <div aria-label="JSON 差异" className="overflow-x-auto rounded-lg border border-border">
        {result.hunks.map((hunk, index) => (
          <Hunk key={index} hunk={hunk} />
        ))}
      </div>
    </div>
  );
}
