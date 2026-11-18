import type * as React from 'react';

import { cn } from '@/lib/utils';

function preventNumberWheel(event: WheelEvent) {
  const input = event.currentTarget;
  if (
    input instanceof HTMLInputElement &&
    input.type === 'number' &&
    input.ownerDocument.activeElement === input &&
    !input.readOnly &&
    !event.ctrlKey &&
    !event.metaKey
  ) {
    event.preventDefault();
  }
}

function Input({ className, type, onFocus, onBlur, ...props }: React.ComponentProps<'input'>) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        'form-control h-9 w-full min-w-0 rounded-lg border px-3 py-1 text-sm outline-none selection:bg-accent selection:text-accent-foreground placeholder:text-muted-foreground disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50',
        'focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/30',
        'aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40',
        type === 'number'
          ? '[appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none'
          : 'appearance-none',
        type === 'search' &&
          '[&::-webkit-search-cancel-button]:appearance-none [&::-webkit-search-decoration]:appearance-none',
        className,
      )}
      {...props}
      onFocus={(event) => {
        // 原生非被动监听器用于阻止数字步进，保留输入焦点及浏览器缩放。
        if (event.currentTarget.type === 'number') {
          event.currentTarget.addEventListener('wheel', preventNumberWheel, { passive: false });
        }
        onFocus?.(event);
      }}
      onBlur={(event) => {
        event.currentTarget.removeEventListener('wheel', preventNumberWheel);
        onBlur?.(event);
      }}
    />
  );
}

export { Input };
