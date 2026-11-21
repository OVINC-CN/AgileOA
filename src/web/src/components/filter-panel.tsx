import { Filter } from 'lucide-react';
import { type ReactNode, type SubmitEventHandler, useId, useState } from 'react';

import { type AppliedFilter, AppliedFilters } from '@/components/applied-filters';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';

export type { AppliedFilter } from '@/components/applied-filters';

interface Props {
  children: ReactNode;
  actions: ReactNode;
  refreshAction?: ReactNode;
  more?: ReactNode;
  hasUnappliedChanges?: boolean | undefined;
  appliedFilters?: readonly AppliedFilter[];
  moreActiveCount?: number;
  onSubmit: SubmitEventHandler<HTMLFormElement>;
}

export function FilterPanel({
  children,
  actions,
  refreshAction,
  more,
  hasUnappliedChanges,
  appliedFilters,
  moreActiveCount = 0,
  onSubmit,
}: Props) {
  const [expanded, setExpanded] = useState(false);
  const moreID = useId();
  return (
    <Card className="gap-4 p-4 sm:p-5">
      <form className="space-y-4" autoComplete="off" onSubmit={onSubmit}>
        <div className="grid grid-cols-1 items-end gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {children}
        </div>
        {more && expanded ? (
          <div
            id={moreID}
            className="grid grid-cols-1 items-end gap-4 sm:grid-cols-2 lg:grid-cols-4"
          >
            {more}
          </div>
        ) : null}
        {appliedFilters ? <AppliedFilters filters={appliedFilters} /> : null}
        <div className="flex flex-wrap items-center justify-end gap-2 border-t border-border-subtle pt-4">
          {hasUnappliedChanges ? (
            <p role="status" aria-label="筛选生效提示" className="mr-auto text-xs text-primary">
              有未应用条件，按回车或点击搜索
            </p>
          ) : null}
          {more ? (
            <Button
              type="button"
              variant="ghost"
              aria-expanded={expanded}
              aria-controls={expanded ? moreID : undefined}
              onClick={() => {
                setExpanded((current) => !current);
              }}
            >
              <Filter aria-hidden="true" />
              更多筛选
              {moreActiveCount > 0 ? `（${String(moreActiveCount)}）` : ''}
            </Button>
          ) : null}
          {refreshAction}
          {actions}
        </div>
      </form>
    </Card>
  );
}

export function FilterField({
  label,
  htmlFor,
  children,
}: {
  label: string;
  htmlFor?: string;
  children: ReactNode;
}) {
  const content = (
    <>
      <span className="text-xs leading-4 text-foreground-secondary">{label}</span>
      {children}
    </>
  );
  return htmlFor ? (
    <label htmlFor={htmlFor} className="grid min-w-0 grid-cols-1 gap-2">
      {content}
    </label>
  ) : (
    <div className="grid min-w-0 grid-cols-1 gap-2">{content}</div>
  );
}
