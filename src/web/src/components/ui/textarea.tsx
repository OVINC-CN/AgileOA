import type { ComponentProps } from 'react';

import { cn } from '@/lib/utils';

export function Textarea({ className, ...props }: ComponentProps<'textarea'>) {
  return (
    <textarea
      data-slot="textarea"
      className={cn(
        'form-control min-h-24 w-full min-w-0 resize-none appearance-none rounded-lg border px-3 py-2 text-sm leading-5 outline-none selection:bg-accent selection:text-accent-foreground placeholder:text-muted-foreground disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50',
        'focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/30',
        'aria-invalid:border-destructive aria-invalid:ring-destructive/20',
        className,
      )}
      {...props}
    />
  );
}
