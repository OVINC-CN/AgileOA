import { type LucideIcon, MoreHorizontal } from 'lucide-react';
import { DropdownMenu } from 'radix-ui';

import { Button } from '@/components/ui/button';

export function TableActionsMenu({
  label,
  actions,
}: {
  label: string;
  actions: readonly {
    label: string;
    icon: LucideIcon;
    onSelect: () => void;
    destructive?: boolean;
  }[];
}) {
  if (actions.length === 0) return null;
  return (
    <DropdownMenu.Root modal={false}>
      <DropdownMenu.Trigger asChild>
        <Button type="button" variant="ghost" size="icon-sm" aria-label={`更多操作 ${label}`}>
          <MoreHorizontal aria-hidden="true" />
        </Button>
      </DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content
          align="end"
          sideOffset={4}
          className="glass-panel menu-motion z-50 min-w-36 rounded-lg border bg-popover p-1"
        >
          {actions.map(({ label: actionLabel, icon: Icon, onSelect, destructive }) => (
            <DropdownMenu.Item
              key={actionLabel}
              onSelect={onSelect}
              className={`flex min-h-8 cursor-pointer items-center gap-2 rounded-md px-3 py-1 text-sm outline-none data-highlighted:bg-secondary ${destructive ? 'text-danger-foreground' : 'text-foreground'}`}
            >
              <Icon className="size-4" aria-hidden="true" />
              {actionLabel}
            </DropdownMenu.Item>
          ))}
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  );
}
