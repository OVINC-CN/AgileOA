import { ChevronLeft, ChevronRight } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { SearchSelect } from '@/components/ui/search-select';
import { Tooltip } from '@/components/ui/tooltip';
import { useApiAction } from '@/lib/api-action';
import { getConfig } from '@/lib/config';
import { formatNumber } from '@/lib/format';
import { cn } from '@/lib/utils';

export function ManagementPagination({
  total,
  page,
  pageSize,
  loading,
  onPage,
  onPageSize,
  className,
}: {
  total: number;
  page: number;
  pageSize: number;
  loading: boolean;
  onPage: (page: number) => Promise<unknown>;
  onPageSize: (size: number) => Promise<unknown>;
  className?: string;
}) {
  const pages = Math.max(1, Math.ceil(total / pageSize));
  const pagination = useApiAction();
  return (
    <div
      className={cn(
        'flex flex-wrap items-center justify-between gap-3 border-t border-border-subtle px-5 py-3',
        className,
      )}
    >
      <p aria-live="polite" className="text-xs text-muted-foreground tabular-nums">
        共 {formatNumber(total)} 条记录
      </p>
      <div className="ml-auto flex items-center gap-2">
        <SearchSelect
          label="每页条数"
          className="w-28 text-xs text-muted-foreground"
          value={String(pageSize)}
          options={getConfig().tablePageSizeOptions.map((size) => ({
            value: String(size),
            label: `${String(size)} / 页`,
          }))}
          disabled={loading || pagination.pending}
          loading={pagination.isPending('size')}
          onValueChange={(value) => {
            void pagination.run('size', () => onPageSize(Number(value)));
          }}
        />
        <div className="flex items-center gap-1">
          <Tooltip content="上一页" disabled={page <= 1 || loading || pagination.pending}>
            <Button
              type="button"
              size="icon-sm"
              variant="ghost"
              disabled={page <= 1 || loading || pagination.pending}
              loading={pagination.isPending('previous')}
              onClick={() => {
                void pagination.run('previous', () => onPage(page - 1));
              }}
              aria-label="上一页"
            >
              <ChevronLeft aria-hidden="true" />
            </Button>
          </Tooltip>
          <span
            aria-live="polite"
            className="px-1 text-xs whitespace-nowrap text-muted-foreground tabular-nums"
          >
            第 {String(page)} / {String(pages)} 页
          </span>
          <Tooltip content="下一页" disabled={page >= pages || loading || pagination.pending}>
            <Button
              type="button"
              size="icon-sm"
              variant="ghost"
              disabled={page >= pages || loading || pagination.pending}
              loading={pagination.isPending('next')}
              onClick={() => {
                void pagination.run('next', () => onPage(page + 1));
              }}
              aria-label="下一页"
            >
              <ChevronRight aria-hidden="true" />
            </Button>
          </Tooltip>
        </div>
      </div>
    </div>
  );
}
