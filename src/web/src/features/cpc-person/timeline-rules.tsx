import { TriangleAlert } from 'lucide-react';

import type { PersonTimelineNode } from '@/features/cpc-person/timeline-api';
import type { TimelineNode } from '@/features/cpc-timeline/model';
import {
  describeTimelineRule,
  timelineRuleRangeLabel,
} from '@/features/cpc-timeline/rule-description';
import { cn } from '@/lib/utils';

export function PersonTimelineRules({
  node,
  nodes,
}: {
  node: PersonTimelineNode;
  nodes: TimelineNode[];
}) {
  if (!node.rules.length || node.status === 'skipped') return null;
  return (
    <section className="min-w-0 space-y-2" aria-label={`${node.name}校验规则`}>
      <h4 className="text-xs font-medium text-muted-foreground">时间校验规则</h4>
      {node.rules.map((rule) => {
        const check = node.checks.find((item) => item.rule_id === rule.id);
        return (
          <div
            key={rule.id}
            className={`min-w-0 space-y-1 rounded-lg border p-3 text-sm ${check?.result === 'abnormal' ? 'border-danger-border bg-background/80 text-danger-foreground' : 'border-border bg-muted/60 text-muted-foreground'}`}
          >
            <div className="flex flex-wrap items-center justify-between gap-2 text-xs font-medium">
              <span>{timelineRuleRangeLabel(rule)}</span>
              <span
                className={cn(
                  'inline-flex items-center gap-1',
                  check?.result === 'normal' && 'text-success-foreground',
                )}
              >
                {check?.result === 'abnormal' ? (
                  <TriangleAlert aria-hidden="true" className="size-4" />
                ) : null}
                {check?.result === 'abnormal' ? '时间范围异常' : (check?.result_label ?? '待校验')}
              </span>
            </div>
            <p className="overflow-x-auto overscroll-x-contain leading-6 whitespace-nowrap">
              {describeTimelineRule(node, rule, nodes, check?.reference_date ? check : undefined)}
            </p>
          </div>
        );
      })}
    </section>
  );
}
