import { Check, Clock3, Pencil, RotateCcw, Save, Send, Undo2 } from 'lucide-react';

import { TableTimestamp } from '@/components/table-timestamp';
import { Badge } from '@/components/ui/badge';
import type { ApprovalStep } from '@/features/project/api';
import {
  type Application,
  applicationOperations,
  approvalSignModes,
} from '@/features/project/application-api';
import { formatIdentity, formatValue } from '@/lib/format';
import { cn } from '@/lib/utils';

interface FlowNode {
  position: number;
  kind: string;
  reviewer_label: string;
  sign_mode: string;
  allow_edit: boolean;
  ready: boolean;
  reviewers: Application['steps'][number]['reviewers'];
}

export function ApplicationFlow({
  record,
  preview,
}: {
  record: Application | null;
  preview: ApprovalStep[];
}) {
  const steps: FlowNode[] =
    record?.steps ??
    preview.map((step, index) => ({ ...step, position: index + 1, ready: true, reviewers: [] }));
  const nodes: FlowNode[] = [
    {
      position: 0,
      kind: 'applicant',
      reviewer_label: record ? formatIdentity(record.student_name, record.student_no) : '申报人',
      sign_mode: 'any',
      allow_edit: true,
      ready: true,
      reviewers: [],
    },
    ...steps,
  ];
  if (!record && steps.length === 0)
    return <p className="text-sm text-muted-foreground">选择申报项目后查看审批流程</p>;
  return (
    <ol aria-label="审批节点" className="text-sm">
      {nodes.map((node) => {
        const completed =
          record?.status === 'approved' || (record !== null && node.position < record.current_step);
        const current =
          record !== null && record.status !== 'approved' && node.position === record.current_step;
        const returned = current && record.status === 'rejected';
        const draft = current && record.status === 'draft';
        const label = completed
          ? '已完成'
          : returned
            ? '待重新提交'
            : draft
              ? '待提交'
              : current
                ? '待审批'
                : '待处理';
        const signed = node.reviewers.filter((reviewer) => reviewer.approved).length;
        return (
          <li
            key={node.position}
            className="group relative grid grid-cols-[2rem_minmax(0,1fr)] gap-4 pb-6 last:pb-0"
          >
            <span
              aria-hidden="true"
              className="absolute top-8 bottom-0 left-4 w-px bg-border/80 group-last:hidden"
            />
            <span
              aria-hidden="true"
              className={cn(
                'relative z-1 flex size-8 items-center justify-center rounded-full border text-xs font-medium tabular-nums',
                completed
                  ? 'border-success-border bg-success text-success-foreground'
                  : returned
                    ? 'border-warning-border bg-warning text-warning-foreground'
                    : current
                      ? 'border-primary bg-primary text-primary-foreground shadow-panel'
                      : 'border-border bg-background text-muted-foreground',
              )}
            >
              {completed ? (
                <Check className="size-4" />
              ) : returned ? (
                <Undo2 className="size-3.5" />
              ) : node.position === 0 ? (
                <Send className="size-3.5" />
              ) : (
                String(node.position)
              )}
            </span>
            <div
              className={cn(
                'min-w-0 space-y-3 rounded-xl border p-4',
                returned
                  ? 'border-warning-border/80 bg-warning/35'
                  : current
                    ? 'border-primary/25 bg-primary/5'
                    : 'border-border bg-background/65',
              )}
            >
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span className="text-xs text-muted-foreground">
                  {node.position === 0 ? '填报' : `第 ${String(node.position)} 级`}
                </span>
                {record ? (
                  <Badge
                    variant="secondary"
                    className={
                      completed
                        ? 'bg-success text-success-foreground'
                        : returned
                          ? 'bg-warning/70 text-warning-foreground'
                          : current
                            ? 'bg-primary/10 text-accent-foreground'
                            : ''
                    }
                  >
                    {label}
                  </Badge>
                ) : null}
              </div>
              <h3 className="text-sm font-medium wrap-anywhere">
                {formatValue(node.reviewer_label)}
              </h3>
              {node.position > 0 ? (
                <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                  <span>
                    {approvalSignModes.find((mode) => mode.value === node.sign_mode)?.label}
                  </span>
                  <span>{node.allow_edit ? '允许编辑内容' : '内容只读'}</span>
                  {!completed && node.reviewers.length > 0 ? (
                    <span className="tabular-nums">
                      已通过 {String(signed)} / {String(node.reviewers.length)}
                    </span>
                  ) : null}
                </div>
              ) : null}
              {!completed && node.reviewers.length > 0 ? (
                <ul className="flex flex-wrap gap-2" aria-label="节点处理人">
                  {node.reviewers.map((reviewer) => (
                    <li
                      key={`${reviewer.identity_type}:${String(reviewer.id)}`}
                      className={cn(
                        'inline-flex max-w-full items-center gap-1 rounded-md border px-2 py-1 text-xs wrap-anywhere',
                        reviewer.approved
                          ? 'border-success-border bg-success text-success-foreground'
                          : 'border-border bg-background/70 text-muted-foreground',
                      )}
                    >
                      {reviewer.approved ? (
                        <Check className="size-3 shrink-0" aria-label="已通过" />
                      ) : (
                        <Clock3 className="size-3 shrink-0" aria-hidden="true" />
                      )}
                      {reviewer.label}
                    </li>
                  ))}
                </ul>
              ) : null}
              {current && !node.ready ? (
                <p className="text-xs text-warning-foreground">所选班委任职尚未配置完整</p>
              ) : null}
            </div>
          </li>
        );
      })}
    </ol>
  );
}

const historyStyles = {
  reset: {
    icon: RotateCcw,
    dot: 'border-change-border bg-change text-change-foreground',
    panel: 'border-change-border/65 bg-change/25',
    text: 'text-change-foreground',
  },
  draft: {
    icon: Save,
    dot: 'border-border bg-muted text-foreground-secondary',
    panel: 'border-border bg-muted/25',
    text: 'text-foreground-secondary',
  },
  submit: {
    icon: Send,
    dot: 'border-info-border bg-info text-info-foreground',
    panel: 'border-info-border/65 bg-info/25',
    text: 'text-info-foreground',
  },
  update: {
    icon: Pencil,
    dot: 'border-change-border bg-change text-change-foreground',
    panel: 'border-change-border/65 bg-change/25',
    text: 'text-change-foreground',
  },
  approve: {
    icon: Check,
    dot: 'border-success-border bg-success text-success-foreground',
    panel: 'border-success-border/65 bg-success/25',
    text: 'text-success-foreground',
  },
  reject: {
    icon: Undo2,
    dot: 'border-danger-border bg-danger text-danger-foreground',
    panel: 'border-danger-border/65 bg-danger/25',
    text: 'text-danger-foreground',
  },
};

export function ApplicationHistory({ record }: { record: Application | null }) {
  if (!record || record.history.length === 0)
    return <p className="text-sm text-muted-foreground">暂无流转记录</p>;
  return (
    <ol aria-label="流转节点" className="text-sm">
      {record.history.map((item) => {
        const style = historyStyles[item.operation];
        const Icon = style.icon;
        return (
          <li
            key={item.id}
            className="group relative grid grid-cols-[2rem_minmax(0,1fr)] gap-4 pb-6 last:pb-0"
          >
            <span
              aria-hidden="true"
              className="absolute top-8 bottom-0 left-4 w-px bg-border/80 group-last:hidden"
            />
            <span
              aria-hidden="true"
              className={cn(
                'relative z-1 flex size-8 items-center justify-center rounded-full border',
                style.dot,
              )}
            >
              <Icon className="size-3.5" />
            </span>
            <div className={cn('min-w-0 space-y-3 rounded-xl border p-4', style.panel)}>
              <div className="flex flex-wrap items-center justify-between gap-2">
                <h3 className={cn('text-sm font-medium', style.text)}>
                  {
                    applicationOperations.find((operation) => operation.value === item.operation)
                      ?.label
                  }
                </h3>
                <span className="text-xs text-muted-foreground">
                  {item.step === 0 ? '申报人' : `第 ${String(item.step)} 级`}
                </span>
              </div>
              <div className="flex flex-wrap items-center justify-between gap-2">
                <p className="text-sm wrap-anywhere">{item.actor_name}</p>
                <span className="inline-flex items-center gap-1 text-muted-foreground">
                  <Clock3 className="size-3" aria-hidden="true" />
                  <TableTimestamp value={item.created_at} />
                </span>
              </div>
              {item.comment.trim() ? (
                <p className="rounded-lg bg-background/60 px-3 py-2 text-sm leading-6 wrap-anywhere whitespace-pre-wrap">
                  {item.comment}
                </p>
              ) : null}
            </div>
          </li>
        );
      })}
    </ol>
  );
}
