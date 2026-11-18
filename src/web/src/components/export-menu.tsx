import { ChevronDown, Download, FileDown } from 'lucide-react';
import { DropdownMenu } from 'radix-ui';

import { Button } from '@/components/ui/button';

export function ExportMenu<T extends string>({
  items,
  loading,
  disabled,
  onSelect,
}: {
  items: { value: T; label: string; disabled?: boolean }[];
  loading: boolean;
  disabled?: boolean;
  onSelect: (value: T) => void;
}) {
  if (items.length === 0) return null;

  return (
    <span className="inline-flex items-center gap-2">
      <DropdownMenu.Root modal={false}>
        <DropdownMenu.Trigger asChild>
          <Button variant="outline" loading={loading} disabled={disabled} className="group">
            <Download aria-hidden="true" />
            导出
            <ChevronDown
              aria-hidden="true"
              className="size-3 transition-transform duration-200 group-data-[state=open]:rotate-180 motion-reduce:transition-none"
            />
          </Button>
        </DropdownMenu.Trigger>
        <DropdownMenu.Portal>
          <DropdownMenu.Content
            align="end"
            sideOffset={8}
            className="glass-panel menu-motion z-50 min-w-40 rounded-lg border bg-popover p-1"
          >
            {items.map((item) => (
              <DropdownMenu.Item
                key={item.value}
                disabled={item.disabled}
                className="flex cursor-pointer items-center gap-2 rounded-md px-3 py-2 text-sm outline-none data-disabled:pointer-events-none data-disabled:opacity-50 data-highlighted:bg-accent"
                onSelect={() => {
                  onSelect(item.value);
                }}
              >
                <FileDown className="size-4" aria-hidden="true" />
                {item.label}
              </DropdownMenu.Item>
            ))}
          </DropdownMenu.Content>
        </DropdownMenu.Portal>
      </DropdownMenu.Root>
    </span>
  );
}
