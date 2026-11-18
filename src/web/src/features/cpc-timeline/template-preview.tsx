import { CalendarDays } from 'lucide-react';

import type { TimelineInfo } from '@/features/cpc-timeline/api';
import { TimelineNodeAttachments } from '@/features/cpc-timeline/attachments';
import { BaselineNodeBadge } from '@/features/cpc-timeline/baseline-node-badge';
import { fieldTypes, studentFieldOptions, type TimelineNode } from '@/features/cpc-timeline/model';
import { TimelineNodeGroups } from '@/features/cpc-timeline/node-groups';
import { RulePreview } from '@/features/cpc-timeline/rule-preview';

export function TemplatePreview({
  nodes,
  workspaceID,
  template,
  actions,
}: {
  nodes: TimelineNode[];
  workspaceID: string;
  template: TimelineInfo | null;
  actions: string[];
}) {
  return (
    <TimelineNodeGroups
      nodes={nodes}
      label="时间线预览"
      renderNode={(node, index) => (
        <li
          key={node.id}
          className="relative ml-3 border-l border-border pb-6 pl-7 last:border-transparent"
        >
          <span className="absolute -left-3 flex size-6 items-center justify-center rounded-full bg-border text-xs text-foreground-secondary">
            {String(index + 1)}
          </span>
          <div className="space-y-3 rounded-xl border border-border bg-background/70 p-4">
            <div className="flex flex-wrap items-center gap-2">
              <h3 className="min-w-0 text-sm font-semibold wrap-anywhere">
                {node.name || '未命名节点'}
              </h3>
              {node.is_baseline ? <BaselineNodeBadge /> : null}
            </div>
            {node.description ? (
              <p className="text-xs leading-5 wrap-anywhere whitespace-pre-wrap text-muted-foreground">
                {node.description}
              </p>
            ) : null}
            <ul className="space-y-2 text-xs">
              {node.fields.map((field) => (
                <li key={field.id} className="flex flex-wrap items-center gap-2">
                  {field.id === node.time_field_id ? (
                    <CalendarDays className="size-3.5" aria-hidden="true" />
                  ) : null}
                  <span className="min-w-0 wrap-anywhere">{field.name || '未命名字段'}</span>
                  <span className="text-muted-foreground">
                    {fieldTypes.find((type) => type.value === field.type)?.label}
                    {field.student_field
                      ? ` · 引用${studentFieldOptions.find((item) => item.value === field.student_field)?.label ?? ''} · 只读`
                      : field.required
                        ? ' · 必填'
                        : ' · 选填'}
                    {field.id === node.time_field_id ? ' · 发生时间' : ''}
                  </span>
                </li>
              ))}
            </ul>
            <RulePreview node={node} nodes={nodes} />
            <TimelineNodeAttachments
              workspaceID={workspaceID}
              template={template}
              node={node}
              actions={actions}
              saved={template?.definition.nodes.some((item) => item.id === node.id) ?? false}
              readOnly
            />
          </div>
        </li>
      )}
    />
  );
}
