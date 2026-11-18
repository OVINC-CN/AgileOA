import { Check } from 'lucide-react';
import type { ComponentProps } from 'react';

import { cn } from '@/lib/utils';

export function Checkbox({ className, ...props }: Omit<ComponentProps<'input'>, 'type'>) {
  return (
    <span
      className={cn(
        'relative inline-flex size-4 shrink-0 align-middle has-disabled:opacity-50',
        className,
      )}
    >
      <input
        {...props}
        type="checkbox"
        data-slot="checkbox"
        className="peer m-0 size-full cursor-pointer appearance-none rounded border border-input bg-background/70 shadow-panel outline-none checked:border-primary checked:bg-primary focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/30 disabled:cursor-not-allowed"
      />
      <Check
        aria-hidden="true"
        className="pointer-events-none absolute inset-0 size-4 p-0.5 text-primary-foreground opacity-0 peer-checked:opacity-100"
      />
    </span>
  );
}
