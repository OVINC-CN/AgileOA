import type { ComponentProps } from 'react';

import { cn } from '@/lib/utils';

export function ButtonGroup({ className, ...props }: ComponentProps<'div'>) {
  return (
    <div
      role="group"
      data-slot="button-group"
      className={cn(
        'inline-flex shrink-0 items-stretch gap-1 rounded-xl border border-border bg-muted/70 p-1 [&>[data-slot=button]]:relative [&>[data-slot=button]]:rounded-lg [&>[data-slot=button]:focus-visible]:z-10',
        className,
      )}
      {...props}
    />
  );
}
