import { Columns3, RotateCcw } from 'lucide-react';
import { Popover } from 'radix-ui';
import { useId, useRef, useState } from 'react';

import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Tooltip } from '@/components/ui/tooltip';
import type { useTableColumns } from '@/lib/table-columns';

export function TableColumnSettings({
  settings,
  label = '列显示',
}: {
  settings: ReturnType<typeof useTableColumns>;
  label?: string;
}) {
  const [open, setOpen] = useState(false);
  const [keyword, setKeyword] = useState('');
  const input = useRef<HTMLInputElement>(null);
  const id = useId();
  const columns = settings.columns.filter((column) =>
    column.label.toLocaleLowerCase().includes(keyword.trim().toLocaleLowerCase()),
  );
  return (
    <Popover.Root
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (next) setKeyword('');
      }}
    >
      <Popover.Trigger asChild>
        <Button type="button" variant="ghost" size="sm" disabled={settings.columns.length === 0}>
          <Columns3 aria-hidden="true" />
          {label}
        </Button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          aria-label={label}
          align="end"
          sideOffset={6}
          collisionPadding={16}
          className="glass-panel menu-motion z-50 flex max-h-[min(24rem,var(--radix-popover-content-available-height))] w-80 max-w-[min(var(--radix-popover-content-available-width),calc(100vw-2rem))] flex-col overflow-hidden rounded-xl border bg-popover shadow-popover"
          onOpenAutoFocus={(event) => {
            event.preventDefault();
            input.current?.focus();
          }}
        >
          <div className="flex shrink-0 items-center gap-1 border-b border-border p-2">
            <Input
              ref={input}
              type="search"
              aria-label="搜索列名称"
              placeholder="搜索列名称"
              className="h-8 flex-1"
              value={keyword}
              onChange={(event) => {
                setKeyword(event.target.value);
              }}
            />
            <Tooltip content="恢复全部列">
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label="恢复全部"
                disabled={settings.visibleColumns.length === settings.columns.length}
                onClick={settings.showAll}
              >
                <RotateCcw aria-hidden="true" />
              </Button>
            </Tooltip>
          </div>
          <div className="min-h-0 space-y-0.5 overflow-y-auto overscroll-contain p-1">
            {columns.map((column) => (
              <Label
                key={column.id}
                htmlFor={`${id}-${column.id}`}
                className="min-h-8 cursor-pointer rounded-lg px-3 py-1.5 leading-5 font-normal hover:bg-muted has-disabled:cursor-default has-disabled:opacity-50"
              >
                <Checkbox
                  id={`${id}-${column.id}`}
                  checked={settings.isVisible(column.id)}
                  disabled={settings.isVisible(column.id) && settings.visibleColumns.length === 1}
                  onChange={(event) => {
                    settings.setVisible(column.id, event.target.checked);
                  }}
                />
                <span className="min-w-0 flex-1 wrap-anywhere">{column.label}</span>
              </Label>
            ))}
            {columns.length === 0 ? (
              <p className="py-6 text-center text-sm text-muted-foreground">未找到匹配字段</p>
            ) : null}
          </div>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}
