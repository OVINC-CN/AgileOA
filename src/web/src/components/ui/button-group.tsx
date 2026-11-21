import type { ComponentProps } from 'react';

import { cn } from '@/lib/utils';

export function ButtonGroup({ className, ...props }: ComponentProps<'div'>) {
  return (
    <div
      role="group"
      data-slot="button-group"
      className={cn(
        'inline-flex shrink-0 items-stretch gap-1 rounded-lg border border-border bg-muted/70 p-px [&>[data-slot=button]]:relative [&>[data-slot=button]]:rounded-md [&>[data-slot=button]:focus-visible]:z-10 [&>[data-slot=button][aria-pressed=true]]:bg-accent [&>[data-slot=button][aria-pressed=true]]:text-accent-foreground',
        className,
      )}
      {...props}
    />
  );
}
