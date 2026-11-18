import { formatDate } from '@/lib/format';

export function TableTimestamp({ value }: { value: string | null | undefined }) {
  return <span className="text-xs text-muted-foreground tabular-nums">{formatDate(value)}</span>;
}
