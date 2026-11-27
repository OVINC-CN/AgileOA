import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Building, Check, ChevronDown, RefreshCw } from 'lucide-react';
import { DropdownMenu } from 'radix-ui';
import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router';

import { ErrorMessage } from '@/components/error-message';
import { Button } from '@/components/ui/button';
import { LoadingIcon } from '@/components/ui/loading-icon';
import { Tooltip } from '@/components/ui/tooltip';
import { myWorkspacesOptions, workspaceContextOptions } from '@/features/workspace/api';
import {
  getWorkspaceEntryPath,
  useActiveWorkspaceID,
  useWorkspaceSection,
  workspaceURL,
} from '@/features/workspace/navigation';
import { useApiAction, useInitialLoading } from '@/lib/api-action';
import { cn } from '@/lib/utils';

export function WorkspaceSwitcher({
  workspaceName,
  onBusyChange,
  disabled = false,
}: {
  workspaceName?: string | undefined;
  onBusyChange?: (busy: boolean) => void;
  disabled?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const workspaceID = useActiveWorkspaceID();
  const section = useWorkspaceSection();
  const navigate = useNavigate();
  const client = useQueryClient();
  const action = useApiAction();
  useEffect(() => {
    onBusyChange?.(action.pending);
    return () => {
      onBusyChange?.(false);
    };
  }, [action.pending, onBusyChange]);
  const list = useQuery({ ...myWorkspacesOptions, enabled: open });
  const listLoading = useInitialLoading(open && list.isFetching && !action.pending);
  const label = workspaceName ?? '选择工作空间';
  const items = list.data?.items ?? [];
  return (
    <DropdownMenu.Root
      modal={false}
      open={open}
      onOpenChange={(next) => {
        if (!action.pending) setOpen(next);
      }}
    >
      <DropdownMenu.Trigger asChild>
        <Button
          type="button"
          variant="ghost"
          disabled={disabled || action.pending}
          aria-label={workspaceName ? `切换工作空间，当前为${workspaceName}` : label}
          aria-busy={listLoading || action.pending}
          className={cn(
            'group max-w-40 min-w-0 shrink gap-2 rounded-lg bg-muted/70 px-2 font-normal text-foreground sm:max-w-56 lg:px-3',
            'hover:bg-secondary data-[state=open]:bg-secondary',
          )}
        >
          <LoadingIcon
            loading={listLoading || action.pending}
            fallback={<Building className="size-4" aria-hidden="true" />}
          />
          <Tooltip content={label} onlyOverflow>
            <span className="max-w-[6em] min-w-0 truncate sm:max-w-none">{label}</span>
          </Tooltip>
          <ChevronDown
            aria-hidden="true"
            className={cn(
              'size-3.5 transition-transform duration-200 group-data-[state=open]:rotate-180',
              'motion-reduce:transition-none',
            )}
          />
        </Button>
      </DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content
          aria-label="工作空间"
          align="start"
          sideOffset={8}
          collisionPadding={16}
          className={cn(
            'glass-panel menu-motion z-50 w-56 overflow-y-auto rounded-lg border bg-popover p-1',
            'max-w-[calc(100vw-2rem)]',
            'max-h-[min(24rem,var(--radix-dropdown-menu-content-available-height))]',
          )}
        >
          {workspaceName ? (
            <DropdownMenu.Label className="space-y-1 border-b border-border px-3 py-2 font-normal sm:hidden">
              <span className="block text-xs text-muted-foreground">当前工作空间</span>
              <span className="block text-sm leading-relaxed break-all whitespace-normal">
                {workspaceName}
              </span>
            </DropdownMenu.Label>
          ) : null}
          {list.isPending ? (
            <p role="status" className="px-3 py-4 text-sm text-muted-foreground">
              正在加载工作空间
            </p>
          ) : null}
          {action.isPending('retry') || list.error ? (
            <div className="space-y-2 p-3">
              <ErrorMessage>{list.error?.message}</ErrorMessage>
              <Button
                variant="outline"
                loading={action.isPending('retry')}
                onClick={() => {
                  void action.run('retry', () => list.refetch());
                }}
              >
                <RefreshCw aria-hidden="true" />
                重新加载
              </Button>
            </div>
          ) : null}
          {items.length > 0 ? (
            <DropdownMenu.RadioGroup
              value={workspaceID}
              onValueChange={(id) => {
                if (disabled || action.pending || id === workspaceID) return;
                void action
                  .run(id, async () => {
                    // 读取目标空间权限，保留当前页面直到目标空间就绪。
                    const context = await client.query({
                      ...workspaceContextOptions(id),
                      staleTime: 0,
                    });

                    // 清理旧空间缓存，并按目标权限进入对应工作类别。
                    if (workspaceID) {
                      await client.cancelQueries({ queryKey: ['workspace', workspaceID] });
                      client.removeQueries({ queryKey: ['workspace', workspaceID] });
                    }
                    await navigate(
                      workspaceURL(getWorkspaceEntryPath(context.actions, section.value), id),
                    );
                  })
                  .then((result) => {
                    if (result.ok) setOpen(false);
                  });
              }}
            >
              {items.map((workspace) => (
                <DropdownMenu.RadioItem
                  key={workspace.id}
                  value={workspace.id}
                  disabled={disabled || action.pending}
                  aria-busy={action.isPending(workspace.id)}
                  onSelect={(event) => {
                    event.preventDefault();
                  }}
                  className="flex cursor-pointer items-center gap-2 rounded-md px-3 py-2 text-sm outline-none data-disabled:opacity-50 data-highlighted:bg-accent"
                >
                  <span className="size-4 shrink-0">
                    <LoadingIcon
                      loading={action.isPending(workspace.id)}
                      fallback={
                        <DropdownMenu.ItemIndicator>
                          <Check className="size-4" aria-hidden="true" />
                        </DropdownMenu.ItemIndicator>
                      }
                    />
                  </span>
                  <Tooltip content={workspace.name} onlyOverflow>
                    <span className="min-w-0 truncate">{workspace.name}</span>
                  </Tooltip>
                </DropdownMenu.RadioItem>
              ))}
            </DropdownMenu.RadioGroup>
          ) : !list.isPending && !list.error && !action.isPending('retry') ? (
            <p className="px-3 py-4 text-sm text-muted-foreground">暂无可访问的工作空间</p>
          ) : null}
          {action.error ? <ErrorMessage>{action.error.message}</ErrorMessage> : null}
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  );
}
