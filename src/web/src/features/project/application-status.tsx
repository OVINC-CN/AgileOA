import { Badge } from '@/components/ui/badge';
import { applicationStatuses, type ApplicationSummary } from '@/features/project/application-api';

const statusStyles: Record<ApplicationSummary['status'], string> = {
  draft: 'border-border bg-secondary/70 text-foreground-secondary',
  pending: 'border-warning-border/80 bg-warning text-warning-foreground',
  rejected: 'border-danger-border/80 bg-danger text-danger-foreground',
  approved: 'border-success-border/80 bg-success text-success-foreground',
};

export function ApplicationStatusBadge({ status }: { status: ApplicationSummary['status'] }) {
  return (
    <Badge variant="outline" className={statusStyles[status]}>
      {applicationStatuses.find((item) => item.value === status)?.label}
    </Badge>
  );
}
