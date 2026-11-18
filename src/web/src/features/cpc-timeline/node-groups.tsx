import { Fragment, type ReactNode } from 'react';

import { Separator } from '@/components/ui/separator';
import type { TimelineNode } from '@/features/cpc-timeline/model';

type GroupableNode = Pick<TimelineNode, 'id' | 'group_name'>;

export function TimelineNodeGroups<T extends GroupableNode>({
  nodes,
  label,
  isVisible,
  renderNode,
}: {
  nodes: T[];
  label: string;
  isVisible?: (node: T) => boolean;
  renderNode: (node: T, index: number) => ReactNode;
}) {
  // 先按原始顺序划分连续分组，再筛选内容，避免筛选改变分组边界和节点编号。
  const groups: { id: string; name: string; entries: { node: T; index: number }[] }[] = [];
  nodes.forEach((node, index) => {
    const name = node.group_name.trim();
    const previous = groups.at(-1);
    if (previous?.name === name) {
      previous.entries.push({ node, index });
    } else {
      groups.push({ id: node.id, name, entries: [{ node, index }] });
    }
  });

  // 仅在可见分组之后显示未分组分割线，避免筛选后出现多余的开头分割线。
  const visibleGroups = groups
    .map((group) => ({
      ...group,
      entries: group.entries.filter(({ node }) => !isVisible || isVisible(node)),
    }))
    .filter((group) => group.entries.length > 0);
  return (
    <ol aria-label={label} className="min-w-0 space-y-0">
      {visibleGroups.map((group, groupIndex) => {
        const children = group.entries.map(({ node, index }) => renderNode(node, index));
        if (!group.name) {
          return (
            <Fragment key={group.id}>
              {visibleGroups[groupIndex - 1]?.name ? (
                <li role="presentation">
                  <Separator decorative={false} className="mb-5 bg-border" />
                </li>
              ) : null}
              {children}
            </Fragment>
          );
        }
        return (
          <li key={group.id}>
            <section aria-label={`分组 ${group.name}`}>
              <div className="mb-5 flex min-w-0 items-center gap-3">
                <Separator className="min-w-0 flex-1 bg-border" />
                <h2 className="max-w-[70%] min-w-0 text-center text-sm font-medium wrap-anywhere text-muted-foreground">
                  {group.name}
                </h2>
                <Separator className="min-w-0 flex-1 bg-border" />
              </div>
              <ol aria-label={`${group.name}的节点`} className="space-y-0">
                {children}
              </ol>
            </section>
          </li>
        );
      })}
    </ol>
  );
}
