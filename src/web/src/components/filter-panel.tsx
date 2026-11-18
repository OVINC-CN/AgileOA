import { Filter } from 'lucide-react';
import { type ReactNode, type SubmitEventHandler, useId, useState } from 'react';

import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';

interface Props {
  children: ReactNode;
  actions: ReactNode;
  more?: ReactNode;
  onSubmit: SubmitEventHandler<HTMLFormElement>;
}

export function FilterPanel({ children, actions, more, onSubmit }: Props) {
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
        <div className="flex flex-wrap justify-end gap-2 border-t border-border-subtle pt-4">
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
            </Button>
          ) : null}
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
      <span className="text-sm leading-5 text-foreground-secondary">{label}</span>
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
