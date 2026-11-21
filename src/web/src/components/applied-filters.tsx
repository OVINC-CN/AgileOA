import { X } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Tooltip } from '@/components/ui/tooltip';
import { useApiAction } from '@/lib/api-action';
import { cn } from '@/lib/utils';

export interface AppliedFilter {
  id: string;
  label: string;
  value: string;
  onRemove: () => Promise<unknown>;
}

function FilterChip({ filter }: { filter: AppliedFilter }) {
  const remove = useApiAction();
  return (
    <Tooltip content={`${filter.label}：${filter.value}`} onlyOverflow>
      <Button
        type="button"
        variant="secondary"
        size="sm"
        className="max-w-full min-w-0 gap-2 px-2 font-normal"
        aria-label={`移除筛选 ${filter.label}：${filter.value}`}
        loading={remove.pending}
        onClick={() => {
          void remove.run('remove', filter.onRemove);
        }}
      >
        <span className="min-w-0 truncate">
          {filter.label}：{filter.value}
        </span>
        <X className="size-3" aria-hidden="true" />
      </Button>
    </Tooltip>
  );
}

export function AppliedFilters({
  filters,
  className,
}: {
  filters: readonly AppliedFilter[];
  className?: string;
}) {
  if (filters.length === 0) return null;
  return (
    <div
      aria-label="已应用筛选"
      className={cn('flex min-w-0 flex-wrap items-center gap-2', className)}
    >
      <span className="text-xs text-muted-foreground">已应用</span>
      {filters.map((filter) => (
        <FilterChip key={filter.id} filter={filter} />
      ))}
    </div>
  );
}
