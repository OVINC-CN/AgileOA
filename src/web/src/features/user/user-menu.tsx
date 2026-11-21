import { useQueryClient } from '@tanstack/react-query';
import { ChevronDown, Eye, EyeOff, LogIn, LogOut, Settings2, UserRound } from 'lucide-react';
import { DropdownMenu } from 'radix-ui';
import { useEffect, useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router';

import { ErrorMessage } from '@/components/error-message';
import { Button } from '@/components/ui/button';
import { LoadingIcon } from '@/components/ui/loading-icon';
import { Tooltip } from '@/components/ui/tooltip';
import { NotificationCenter } from '@/features/notification/notification-center';
import {
  type CurrentUser,
  currentUserKey,
  currentUserOptions,
  logout,
  useCurrentUser,
} from '@/features/user/api';
import { authURL, studentLoginPath } from '@/features/user/navigation';
import { setWatermarkHidden, useWatermarkHidden } from '@/features/user/watermark';
import { useApiAction, useApiMutation } from '@/lib/api-action';
import { cn } from '@/lib/utils';

export function UserMenu({
  disabled = false,
  onBusyChange,
}: {
  disabled?: boolean;
  onBusyChange?: (busy: boolean) => void;
}) {
  const { data: user, isPending } = useCurrentUser();
  const client = useQueryClient();
  const navigate = useNavigate();
  const location = useLocation();
  const [open, setOpen] = useState(false);
  const hidden = useWatermarkHidden(user);
  const watermark = useApiAction();
  const mutation = useApiMutation({
    mutationFn: logout,
    onSuccess: async () => {
      await client.cancelQueries();
      client.clear();
      client.setQueryData(currentUserKey, null);
      await navigate(user?.identity_type === 'student' ? studentLoginPath : '/login', {
        replace: true,
      });
    },
  });
  const pending = mutation.isPending || watermark.pending;
  const busy = pending || disabled;
  useEffect(() => {
    onBusyChange?.(pending);
    return () => {
      onBusyChange?.(false);
    };
  }, [pending, onBusyChange]);

  if (!user) {
    return isPending ? null : (
      <Button asChild variant="outline">
        <Link to={authURL('/login', location.pathname + location.search + location.hash)}>
          <LogIn aria-hidden="true" />
          登录
        </Link>
      </Button>
    );
  }
  return (
    <div className="flex shrink-0 items-center gap-1 lg:gap-2">
      {!user.password_change_required ? (
        <NotificationCenter
          key={`${user.identity_type}:${user.workspace_id}:${String(user.id)}`}
          userID={user.id}
          studentWorkspaceID={user.identity_type === 'student' ? user.workspace_id : undefined}
        />
      ) : null}
      <DropdownMenu.Root
        modal={false}
        open={open}
        onOpenChange={(next) => {
          if (!busy) setOpen(next);
        }}
      >
        <DropdownMenu.Trigger asChild>
          <Button
            type="button"
            variant="ghost"
            className="group size-9 max-w-52 gap-2 px-0 font-normal sm:w-auto sm:px-3"
            disabled={disabled}
            aria-label={`当前用户 ${user.display_name}`}
          >
            <UserRound aria-hidden="true" />
            <Tooltip content={user.display_name} onlyOverflow>
              <span className="hidden min-w-0 truncate sm:inline">{user.display_name}</span>
            </Tooltip>
            <ChevronDown
              aria-hidden="true"
              className={cn(
                'hidden size-3.5 shrink-0 transition-transform duration-200 group-data-[state=open]:rotate-180',
                'motion-reduce:transition-none sm:block',
              )}
            />
          </Button>
        </DropdownMenu.Trigger>
        <DropdownMenu.Portal>
          <DropdownMenu.Content
            align="end"
            sideOffset={8}
            className="glass-panel menu-motion z-50 max-w-64 min-w-40 rounded-lg border bg-popover p-1"
          >
            <DropdownMenu.Item
              disabled
              className="flex cursor-default items-center gap-2 rounded-md px-3 py-2 text-sm outline-none"
            >
              <UserRound className="size-4" aria-hidden="true" />
              <span className="break-all">{user.username}</span>
            </DropdownMenu.Item>
            {user.is_admin ? (
              <>
                <DropdownMenu.Item asChild disabled={busy}>
                  <Link
                    to="/system"
                    onClick={() => {
                      setOpen(false);
                    }}
                    className="flex cursor-pointer items-center gap-2 rounded-md px-3 py-2 text-sm outline-none data-disabled:opacity-50 data-highlighted:bg-accent"
                  >
                    <Settings2 className="size-4" aria-hidden="true" />
                    系统管理
                  </Link>
                </DropdownMenu.Item>
                <DropdownMenu.Item
                  disabled={busy}
                  aria-busy={watermark.pending}
                  onSelect={(event) => {
                    event.preventDefault();
                    void watermark.run('watermark', async () => {
                      // 点击时复核缓存与实时身份，账号切换或管理员撤权后拒绝修改。
                      const cached = client.getQueryData<CurrentUser | null>(currentUserKey);
                      if (!cached?.is_admin || cached.id !== user.id) {
                        throw new Error('仅管理员可以设置水印');
                      }
                      const current = await client.query(currentUserOptions);
                      if (!current?.is_admin || current.id !== user.id) {
                        throw new Error('管理员身份已失效，请重新登录');
                      }
                      setWatermarkHidden(current, !hidden);
                      setOpen(false);
                    });
                  }}
                  className="flex cursor-pointer items-center gap-2 rounded-md px-3 py-2 text-sm outline-none data-disabled:opacity-50 data-highlighted:bg-accent"
                >
                  <LoadingIcon
                    loading={watermark.pending}
                    fallback={
                      hidden ? (
                        <Eye className="size-4" aria-hidden="true" />
                      ) : (
                        <EyeOff className="size-4" aria-hidden="true" />
                      )
                    }
                  />
                  {hidden ? '显示水印' : '隐藏水印'}
                </DropdownMenu.Item>
              </>
            ) : null}
            <DropdownMenu.Item
              disabled={busy}
              aria-busy={mutation.isPending}
              onSelect={(event) => {
                event.preventDefault();
                mutation.mutate();
              }}
              className="flex cursor-pointer items-center gap-2 rounded-md px-3 py-2 text-sm outline-none data-disabled:opacity-50 data-highlighted:bg-accent"
            >
              <LoadingIcon
                loading={mutation.isPending}
                fallback={<LogOut className="size-4" aria-hidden="true" />}
              />
              退出登录
            </DropdownMenu.Item>
            {mutation.error ? <ErrorMessage>{mutation.error.message}</ErrorMessage> : null}
          </DropdownMenu.Content>
        </DropdownMenu.Portal>
      </DropdownMenu.Root>
    </div>
  );
}
