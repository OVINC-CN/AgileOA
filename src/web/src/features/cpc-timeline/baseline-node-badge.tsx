import { Flag } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Tooltip } from '@/components/ui/tooltip';

export function BaselineNodeIcon() {
  return (
    <Tooltip content="基准节点">
      <span
        role="img"
        aria-label="基准节点"
        className="inline-flex shrink-0 text-warning-foreground"
      >
        <Flag aria-hidden="true" className="size-3.5" />
      </span>
    </Tooltip>
  );
}

export function BaselineNodeBadge() {
  return (
    <Badge className="border-chart-2/70 bg-chart-2/20 text-warning-foreground">
      <Flag aria-hidden="true" />
      基准节点
    </Badge>
  );
}
