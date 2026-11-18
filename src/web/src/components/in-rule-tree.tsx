import type { InRuleNode } from '@/components/in-rule-editor';
import { Badge } from '@/components/ui/badge';

export function InRuleTree<Field extends string>({
  rule,
  labels,
  label,
}: {
  rule: InRuleNode<Field>;
  labels: Readonly<Partial<Record<Field, string>>>;
  label: string;
}) {
  return (
    <ul aria-label={label} className="space-y-3 text-sm">
      <RuleTreeNode node={rule} labels={labels} />
    </ul>
  );
}

function RuleTreeNode<Field extends string>({
  node,
  labels,
  nested = false,
}: {
  node: InRuleNode<Field>;
  labels: Readonly<Partial<Record<Field, string>>>;
  nested?: boolean;
}) {
  return (
    <li className="relative space-y-2">
      {nested ? (
        <span aria-hidden="true" className="absolute top-3 -left-4 h-px w-3 bg-border/80" />
      ) : null}
      {node.type === 'group' ? (
        <>
          <p>{node.logic === 'and' ? '全部满足（AND）' : '任一满足（OR）'}</p>
          <ul className="space-y-3 border-l border-border pl-4">
            {node.children.map((child, index) => (
              <RuleTreeNode key={index} node={child} labels={labels} nested />
            ))}
          </ul>
        </>
      ) : (
        <div className="flex flex-wrap items-center gap-2">
          <span>{labels[node.field] ?? node.field}</span>
          <span className="text-xs text-muted-foreground">in</span>
          {node.values.map((value) => (
            <Badge
              key={value}
              variant="outline"
              className="max-w-full rounded-md border-border bg-background/70 px-2 py-1 text-xs font-normal break-words whitespace-normal"
            >
              {value}
            </Badge>
          ))}
        </div>
      )}
    </li>
  );
}
