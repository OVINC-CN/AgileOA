import { Badge } from '@/components/ui/badge';

export function ProjectEnabledBadge({
  enabled,
  enabledLabel = '开启',
}: {
  enabled: boolean;
  enabledLabel?: string;
}) {
  return (
    <Badge
      variant="outline"
      className={
        enabled
          ? 'border-success-border/80 bg-success text-success-foreground'
          : 'border-border bg-secondary/70 text-foreground-secondary'
      }
    >
      {enabled ? enabledLabel : '关闭'}
    </Badge>
  );
}
