import { type LucideIcon, MoreHorizontal } from 'lucide-react';
import { DropdownMenu } from 'radix-ui';

import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

export function TableActionsMenu({
  label,
  actions,
  disabled = false,
  loading = false,
}: {
  label: string;
  disabled?: boolean;
  loading?: boolean;
  actions: readonly {
    label: string;
    icon: LucideIcon;
    onSelect: () => void;
    destructive?: boolean;
    disabled?: boolean;
  }[];
}) {
  if (actions.length === 0) return null;
  return (
    <DropdownMenu.Root modal={false}>
      <DropdownMenu.Trigger asChild disabled={disabled || loading}>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          aria-label={`更多操作 ${label}`}
          loading={loading}
        >
          <MoreHorizontal aria-hidden="true" />
        </Button>
      </DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content
          align="end"
          sideOffset={4}
          className="glass-panel menu-motion z-50 min-w-36 rounded-lg border bg-popover p-1"
        >
          {actions.map(({ label: actionLabel, icon: Icon, onSelect, destructive, disabled }) => (
            <DropdownMenu.Item
              key={actionLabel}
              onSelect={onSelect}
              disabled={disabled ?? false}
              className={cn(
                'flex min-h-8 cursor-pointer items-center gap-2 rounded-md px-3 py-1 text-sm',
                'outline-none data-disabled:pointer-events-none data-disabled:opacity-50',
                'data-highlighted:bg-secondary',
                destructive ? 'text-danger-foreground' : 'text-foreground',
              )}
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
