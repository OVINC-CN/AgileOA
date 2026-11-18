import type { TimelineNode } from '@/features/cpc-timeline/model';
import {
  describeTimelineRule,
  timelineRuleRangeLabel,
} from '@/features/cpc-timeline/rule-description';

export function RulePreview({ node, nodes }: { node: TimelineNode; nodes: TimelineNode[] }) {
  if (!node.rules.length) return null;
  const nodeName = node.name.trim() || '未命名节点';
  return (
    <section
      aria-label={`${nodeName}的时间校验规则`}
      className="min-w-0 space-y-3 rounded-lg border border-warning-border/60 bg-warning/60 p-3"
    >
      <h4 className="text-xs font-medium text-warning-foreground">时间校验规则</h4>
      <ul className="space-y-2 overflow-x-auto overscroll-x-contain text-sm leading-6 text-foreground-secondary">
        {node.rules.map((rule) => (
          <li key={rule.id} className="whitespace-nowrap">
            <span className="mr-2 rounded bg-background/80 px-1.5 py-0.5 text-xs font-medium text-warning-foreground">
              {timelineRuleRangeLabel(rule)}
            </span>
            {describeTimelineRule(node, rule, nodes)}
          </li>
        ))}
      </ul>
    </section>
  );
}
