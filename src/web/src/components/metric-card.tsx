import type { LucideIcon } from 'lucide-react';

import { Card } from '@/components/ui/card';
import { emptyValueLabel, formatNumber } from '@/lib/format';
import { cn } from '@/lib/utils';

export function MetricCard({
  title,
  value,
  note,
  icon: Icon,
  valueSize = 'default',
}: {
  title: string;
  value: number | undefined;
  note: string;
  icon: LucideIcon;
  valueSize?: 'default' | 'large';
}) {
  return (
    <Card className="gap-2 p-4 sm:p-5">
      <div className="flex items-center justify-between gap-3">
        <span className="text-xs leading-4 text-muted-foreground">{title}</span>
        <Icon className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
      </div>
      <strong
        className={cn(
          'font-medium tabular-nums',
          valueSize === 'large' ? 'text-metric-emphasis' : 'text-xl leading-6',
        )}
      >
        {value === undefined ? emptyValueLabel : formatNumber(value)}
      </strong>
      <p className="text-xs text-muted-foreground">{note}</p>
    </Card>
  );
}
