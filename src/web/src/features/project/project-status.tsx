import { Badge } from '@/components/ui/badge';

export function ProjectSelfApplyBadge({ allowed }: { allowed: boolean }) {
  return (
    <Badge
      variant="outline"
      className={
        allowed
          ? 'border-success-border/80 bg-success text-success-foreground'
          : 'border-border bg-secondary/70 text-foreground-secondary'
      }
    >
      {allowed ? '允许' : '关闭'}
    </Badge>
  );
}
