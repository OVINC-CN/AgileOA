import { type LucideIcon, MoreHorizontal } from 'lucide-react';
import { DropdownMenu } from 'radix-ui';
import { useState } from 'react';

import { Button } from '@/components/ui/button';
import { LoadingIcon } from '@/components/ui/loading-icon';
import { type DeleteConfirmationAction, useDeleteConfirmation } from '@/lib/delete-confirmation';
import { cn } from '@/lib/utils';

type TableAction = {
  label: string;
  icon: LucideIcon;
  destructive?: boolean;
  disabled?: boolean;
} & ({ onSelect: () => void } | { confirmDelete: DeleteConfirmationAction });

export function TableActionsMenu({
  label,
  actions,
  disabled = false,
  loading = false,
}: {
  label: string;
  disabled?: boolean;
  loading?: boolean;
  actions: readonly TableAction[];
}) {
  const [open, setOpen] = useState(false);
  const deletion = useDeleteConfirmation();
  if (actions.length === 0) return null;
  return (
    <DropdownMenu.Root
      modal={false}
      open={open}
      onOpenChange={(next) => {
        if (deletion.pending) return;
        if (!next) deletion.cancel();
        setOpen(next);
      }}
    >
      <DropdownMenu.Trigger asChild disabled={disabled || loading}>
        <Button
          type="button"
          variant="table-action"
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
          onEscapeKeyDown={(event) => {
            if (deletion.pending) event.preventDefault();
          }}
          onInteractOutside={(event) => {
            if (deletion.pending) event.preventDefault();
          }}
        >
          {actions.map((action) => {
            const { label: actionLabel, icon: Icon, destructive, disabled } = action;
            const confirmDelete = 'confirmDelete' in action ? action.confirmDelete : null;
            const confirming = confirmDelete !== null && deletion.target === confirmDelete.target;
            const busy = confirming && deletion.pending;
            return (
              <DropdownMenu.Item
                key={actionLabel}
                onClick={(event) => {
                  if (confirmDelete && event.detail > 1) event.preventDefault();
                }}
                onKeyDown={(event) => {
                  if (
                    confirmDelete &&
                    event.repeat &&
                    (event.key === 'Enter' || event.key === ' ')
                  ) {
                    event.preventDefault();
                  }
                }}
                onSelect={(event) => {
                  if ('confirmDelete' in action) {
                    event.preventDefault();
                    deletion.select(action.confirmDelete, () => {
                      setOpen(false);
                    });
                  } else {
                    action.onSelect();
                  }
                }}
                disabled={deletion.pending || disabled === true}
                aria-description={confirming ? `确认删除 ${confirmDelete.target} ?` : undefined}
                aria-busy={busy || undefined}
                className={cn(
                  'flex min-h-8 cursor-pointer items-center gap-2 rounded-md px-3 py-1 text-sm',
                  'transition-colors outline-none motion-safe:duration-(--motion-delete-confirmation-duration) motion-reduce:transition-none',
                  'data-disabled:pointer-events-none data-disabled:opacity-50',
                  confirming
                    ? 'bg-primary text-primary-foreground data-highlighted:ring-2 data-highlighted:ring-ring/30'
                    : 'data-highlighted:bg-secondary',
                  !confirming && (destructive ? 'text-danger-foreground' : 'text-foreground'),
                )}
              >
                <LoadingIcon
                  loading={busy}
                  fallback={<Icon className="size-4" aria-hidden="true" />}
                />
                <span
                  key={String(confirming)}
                  className={
                    confirming
                      ? 'motion-safe:animate-in motion-safe:duration-(--motion-delete-confirmation-duration) motion-safe:ease-standard motion-safe:fade-in'
                      : undefined
                  }
                >
                  {confirming ? '确认' : actionLabel}
                </span>
              </DropdownMenu.Item>
            );
          })}
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  );
}
