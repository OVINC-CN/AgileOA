import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowUpRight, Bell, Check, CheckCheck } from 'lucide-react';
import { type MouseEvent, useEffect, useMemo, useState } from 'react';
import { Link, useNavigate } from 'react-router';

import { ManagementPagination } from '@/components/management-pagination';
import { RefreshButton } from '@/components/refresh-button';
import { Button } from '@/components/ui/button';
import { Drawer } from '@/components/ui/drawer';
import { LoadingIcon } from '@/components/ui/loading-icon';
import {
  NavigationButtonGroup,
  NavigationGroupButton,
} from '@/components/ui/navigation-button-group';
import {
  markNotificationsRead,
  type NotificationInfo,
  notificationKey,
  notificationListOptions,
  type NotificationRecipient,
  type NotificationStatus,
  notificationStatuses,
  notificationUnreadKey,
  notificationUnreadOptions,
} from '@/features/notification/api';
import { refreshNotificationTarget } from '@/features/notification/refresh';
import { useApiAction } from '@/lib/api-action';
import { getConfig } from '@/lib/config';
import { formatDate, formatNumber, formatValue } from '@/lib/format';
import { useTablePagination } from '@/lib/pagination';
import { cn } from '@/lib/utils';

const maxBadgeCount = 99;

export function NotificationCenter({
  userID,
  studentWorkspaceID,
}: {
  userID: number;
  studentWorkspaceID?: string | undefined;
}) {
  const recipient = useMemo<NotificationRecipient>(
    () => (studentWorkspaceID ? { workspaceID: studentWorkspaceID, studentID: userID } : userID),
    [studentWorkspaceID, userID],
  );
  const [open, setOpen] = useState(false);
  const client = useQueryClient();
  const action = useApiAction();
  const config = getConfig();
  const unread = useQuery({
    ...notificationUnreadOptions(recipient),
    refetchInterval: open ? false : config.notificationPollIntervalMs,
    refetchOnWindowFocus: true,
  });
  const count = unread.data?.unread_count ?? 0;
  return (
    <>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="relative"
        aria-label={count > 0 ? `通知（${String(count)} 条未读）` : '通知'}
        aria-haspopup="dialog"
        aria-expanded={open}
        loading={action.pending}
        onClick={() => {
          void action.run('open', async () => {
            const data = await client.query({
              ...notificationListOptions(recipient, 1, config.tablePageSize),
              staleTime: 0,
            });
            client.setQueryData(notificationUnreadKey(recipient), {
              unread_count: data.unread_count,
            });
            setOpen(true);
          });
        }}
      >
        <Bell aria-hidden="true" />
        {count > 0 ? (
          <span
            aria-hidden="true"
            className="pointer-events-none absolute -top-1 -right-1 flex h-4 min-w-4 items-center justify-center rounded-full bg-primary px-1 text-xs leading-none font-semibold text-primary-foreground ring-2 ring-background"
          >
            {count > maxBadgeCount ? `${String(maxBadgeCount)}+` : String(count)}
          </span>
        ) : null}
      </Button>
      <span role="status" className="sr-only">
        {count > 0 ? `有 ${String(count)} 条未读通知` : '没有未读通知'}
      </span>
      {open ? (
        <NotificationDrawer
          recipient={recipient}
          onClose={() => {
            setOpen(false);
          }}
        />
      ) : null}
    </>
  );
}

function NotificationDrawer({
  recipient,
  onClose,
}: {
  recipient: NotificationRecipient;
  onClose: () => void;
}) {
  const navigate = useNavigate();
  const client = useQueryClient();
  const pagination = useTablePagination();
  const [status, setStatus] = useState<NotificationStatus>('unread');
  const mark = useApiAction();
  const filter = useApiAction();
  const query = useQuery({
    ...notificationListOptions(recipient, pagination.page, pagination.pageSize, status),
    placeholderData: keepPreviousData,
    refetchInterval: getConfig().notificationPollIntervalMs,
    refetchOnWindowFocus: true,
  });
  const data = query.data;
  const unreadIDs =
    data?.items.filter((item) => item.read_at === null).map((item) => item.id) ?? [];

  // 打开、翻页和后台刷新后同步角标，已读提交会先取消旧请求，避免旧数量覆盖结果。
  useEffect(() => {
    if (data && !query.isPlaceholderData) {
      client.setQueryData(notificationUnreadKey(recipient), { unread_count: data.unread_count });
    }
  }, [client, data, query.isPlaceholderData, recipient]);

  const markRead = async (ids: number[]) => {
    await client.cancelQueries({ queryKey: notificationKey(recipient) });
    const result = await markNotificationsRead(ids, recipient);
    client.setQueryData(notificationUnreadKey(recipient), { unread_count: result.unread_count });
    // 未读列表缩短时回到仍存在的页码，避免最后一页读完后停在空页。
    const lastPage = Math.max(1, Math.ceil(result.unread_count / pagination.pageSize));
    if (status === 'unread' && pagination.page > lastPage) {
      await client.query({
        ...notificationListOptions(recipient, lastPage, pagination.pageSize, status),
        staleTime: 0,
      });
      pagination.setPage(lastPage);
    }
    await client.invalidateQueries({ queryKey: notificationKey(recipient) });
  };
  const fetchPage = async (page: number, size: number) => {
    await client.query({ ...notificationListOptions(recipient, page, size, status), staleTime: 0 });
    if (size !== pagination.pageSize) pagination.setPageSize(size);
    else pagination.setPage(page);
  };
  const viewDetails = async (
    event: MouseEvent<HTMLAnchorElement>,
    notification: NotificationInfo,
  ) => {
    if (event.type === 'auxclick' && event.button !== 1) return;
    if (mark.pending || filter.pending || query.isPlaceholderData) {
      event.preventDefault();
      return;
    }

    // 普通点击等待已读及缓存更新，修饰键和中键保留浏览器的新窗口交互。
    const openHere =
      event.button === 0 && !event.ctrlKey && !event.metaKey && !event.shiftKey && !event.altKey;
    if (openHere) event.preventDefault();
    if (openHere || notification.read_at === null) {
      const result = await mark.run(`detail:${String(notification.id)}`, async () => {
        if (notification.read_at === null) await markRead([notification.id]);
        if (openHere) await refreshNotificationTarget(client, notification.url);
      });
      if (!result.ok) return;
    }

    // 同步关闭通知并跳转，避免目标抽屉被页面过渡快照覆盖。
    if (openHere) {
      onClose();
      void navigate(notification.url, { flushSync: true });
    }
  };
  return (
    <Drawer
      title="通知"
      description={`共 ${formatNumber(data?.unread_count ?? 0)} 条未读通知`}
      busy={mark.pending}
      onClose={onClose}
      footerNavigation={
        <ManagementPagination
          className="border-0 p-0"
          total={data?.total ?? 0}
          page={pagination.page}
          pageSize={pagination.pageSize}
          loading={query.isPending || mark.pending || filter.pending}
          onPage={(page) => fetchPage(page, pagination.pageSize)}
          onPageSize={(size) => fetchPage(1, size)}
        />
      }
    >
      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-border-subtle px-6 py-3">
        <NavigationButtonGroup value={status} role="group" aria-label="通知范围">
          {notificationStatuses.map(({ value, label }) => (
            <NavigationGroupButton
              key={value}
              aria-pressed={value === status}
              disabled={mark.pending || filter.pending}
              loading={filter.isPending(value)}
              onClick={() => {
                void filter.run(value, async () => {
                  await client.query({
                    ...notificationListOptions(recipient, 1, pagination.pageSize, value),
                    staleTime: 0,
                  });
                  setStatus(value);
                  pagination.setPage(1);
                });
              }}
            >
              {label}
            </NavigationGroupButton>
          ))}
        </NavigationButtonGroup>
        <div className="ml-auto flex items-center gap-2">
          <Button
            variant="outline"
            disabled={
              mark.pending || filter.pending || unreadIDs.length === 0 || query.isPlaceholderData
            }
            loading={mark.isPending('page')}
            onClick={() => {
              void mark.run('page', () => markRead(unreadIDs));
            }}
          >
            <CheckCheck aria-hidden="true" />
            本页标为已读
          </Button>
          <RefreshButton label="刷新通知" onRefresh={() => query.refetch({ throwOnError: true })} />
        </div>
      </div>
      {data?.items.length ? (
        <ul aria-label="通知列表" className="space-y-3 p-6">
          {data.items.map((notification) => (
            <li
              key={notification.id}
              className={cn(
                'glass-panel rounded-xl border border-border p-4',
                notification.read_at === null && 'border-primary/30 bg-primary/5',
              )}
            >
              <div className="flex items-start gap-2">
                {notification.read_at === null ? (
                  <span className="mt-1.5 size-2 shrink-0 rounded-full bg-primary">
                    <span className="sr-only">未读</span>
                  </span>
                ) : null}
                <h3 className="min-w-0 text-sm font-semibold wrap-anywhere">
                  {formatValue(notification.title)}
                </h3>
              </div>
              <p className="mt-2 text-sm leading-6 wrap-anywhere whitespace-pre-wrap text-foreground-secondary">
                {formatValue(notification.content)}
              </p>
              <div className="mt-3 flex flex-wrap items-center justify-between gap-2">
                <time dateTime={notification.created_at} className="text-xs text-muted-foreground">
                  {formatDate(notification.created_at)}
                </time>
                <div className="ml-auto flex items-center gap-2">
                  {notification.url ? (
                    <Button
                      asChild
                      variant="link"
                      size="sm"
                      loading={mark.isPending(`detail:${String(notification.id)}`)}
                    >
                      <Link
                        to={notification.url}
                        aria-label={`查看详情：${notification.title}`}
                        aria-disabled={mark.pending || filter.pending || query.isPlaceholderData}
                        onClick={(event) => {
                          void viewDetails(event, notification);
                        }}
                        onAuxClick={(event) => {
                          void viewDetails(event, notification);
                        }}
                      >
                        <LoadingIcon
                          loading={mark.isPending(`detail:${String(notification.id)}`)}
                          fallback={<ArrowUpRight aria-hidden="true" />}
                        />
                        查看详情
                      </Link>
                    </Button>
                  ) : null}
                  {notification.read_at === null ? (
                    <Button
                      variant="ghost"
                      size="sm"
                      aria-label={`标为已读：${notification.title}`}
                      disabled={mark.pending || filter.pending || query.isPlaceholderData}
                      loading={mark.isPending(String(notification.id))}
                      onClick={() => {
                        void mark.run(String(notification.id), () => markRead([notification.id]));
                      }}
                    >
                      <Check aria-hidden="true" />
                      标为已读
                    </Button>
                  ) : null}
                </div>
              </div>
            </li>
          ))}
        </ul>
      ) : (
        <p role="status" className="px-6 py-16 text-center text-sm text-muted-foreground">
          {query.isPending
            ? '正在读取通知'
            : query.isError
              ? '通知暂时无法读取'
              : status === 'unread'
                ? '暂无未读通知'
                : '暂无通知'}
        </p>
      )}
    </Drawer>
  );
}
