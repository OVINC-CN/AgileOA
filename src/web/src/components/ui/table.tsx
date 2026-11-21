'use client';

import type * as React from 'react';

import { useLoadingDelay } from '@/lib/loading';
import { cn } from '@/lib/utils';

/* eslint-disable jsx-a11y/no-noninteractive-tabindex -- 滚动容器提供键盘焦点以支持横向滚动。 */
function Table({
  className,
  pinFirstColumn = false,
  ...props
}: React.ComponentProps<'table'> & { pinFirstColumn?: boolean }) {
  const busy = useLoadingDelay(props['aria-busy'] === true || props['aria-busy'] === 'true');
  return (
    <div
      data-slot="table-container"
      role="region"
      aria-label="表格内容，可横向滚动"
      tabIndex={0}
      className="relative w-full max-w-full min-w-0 overflow-x-auto overscroll-x-contain rounded-lg outline-none focus-visible:ring-2 focus-visible:ring-ring/30 focus-visible:ring-inset"
    >
      <table
        data-slot="table"
        data-pin-first-column={pinFirstColumn || undefined}
        className={cn(
          'w-max min-w-full caption-bottom text-sm [&_td_*]:whitespace-nowrap [&_th_*]:whitespace-nowrap',
          className,
        )}
        {...props}
        aria-busy={busy || props['aria-busy']}
      />
    </div>
  );
}

/* eslint-enable jsx-a11y/no-noninteractive-tabindex */

function TableHeader({ className, ...props }: React.ComponentProps<'thead'>) {
  return (
    <thead
      data-slot="table-header"
      className={cn('bg-table-header [&_tr]:border-b', className)}
      {...props}
    />
  );
}

function TableBody({ className, ...props }: React.ComponentProps<'tbody'>) {
  return (
    <tbody
      data-slot="table-body"
      className={cn('[&_tr:last-child]:border-0', className)}
      {...props}
    />
  );
}

function TableFooter({ className, ...props }: React.ComponentProps<'tfoot'>) {
  return (
    <tfoot
      data-slot="table-footer"
      className={cn('border-t bg-table-header font-medium [&>tr]:last:border-b-0', className)}
      {...props}
    />
  );
}

function TableRow({ className, ...props }: React.ComponentProps<'tr'>) {
  return (
    <tr
      data-slot="table-row"
      className={cn(
        'border-b border-border-subtle transition-colors focus-within:bg-table-hover hover:bg-table-hover has-aria-expanded:bg-table-selected data-[state=selected]:bg-table-selected motion-reduce:transition-none',
        className,
      )}
      {...props}
    />
  );
}

function TableHead({ className, ...props }: React.ComponentProps<'th'>) {
  return (
    <th
      data-slot="table-head"
      className={cn(
        'h-8 px-3 py-1 text-left align-middle text-xs font-medium whitespace-nowrap text-foreground-secondary [&:has([role=checkbox])]:pr-0',
        className,
      )}
      {...props}
    />
  );
}

function TableCell({ className, ...props }: React.ComponentProps<'td'>) {
  return (
    <td
      data-slot="table-cell"
      className={cn(
        'h-8 px-3 py-1 align-middle leading-5 whitespace-nowrap [&:has([role=checkbox])]:pr-0',
        className,
      )}
      {...props}
    />
  );
}

function TableCaption({ className, ...props }: React.ComponentProps<'caption'>) {
  return (
    <caption
      data-slot="table-caption"
      className={cn('mt-4 text-sm text-muted-foreground', className)}
      {...props}
    />
  );
}

export { Table, TableBody, TableCaption, TableCell, TableFooter, TableHead, TableHeader, TableRow };
