import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

import { isNotificationURL } from '@/features/notification/navigation';
import { apiGet, apiPost } from '@/lib/api';

const notificationSchema = z.object({
  id: z.number().int().positive(),
  title: z.string().min(1),
  content: z.string().min(1),
  url: z.string().max(2048).refine(isNotificationURL).default(''),
  read_at: z.iso.datetime({ offset: true }).nullable(),
  created_at: z.iso.datetime({ offset: true }),
});
const unreadSchema = z.object({ unread_count: z.number().int().nonnegative() });
const listSchema = unreadSchema.extend({
  items: z.array(notificationSchema),
  total: z.number().int().nonnegative(),
  page: z.number().int().positive(),
  page_size: z.number().int().positive(),
});
const readSchema = unreadSchema.extend({ changed: z.number().int().nonnegative() });

export type NotificationInfo = z.infer<typeof notificationSchema>;
export type NotificationRecipient = number | { workspaceID: string; studentID: number };
export type NotificationStatus = 'unread' | 'all';
export const notificationStatuses: { value: NotificationStatus; label: string }[] = [
  { value: 'unread', label: '未读通知' },
  { value: 'all', label: '全部通知' },
];

// 通过请求元数据区分同一查询的定时请求和常规请求，重试沿用本次标记。
export const notificationPollFetchMeta = Object.freeze({});

// 两类收件箱按身份隔离，学生缓存同时包含绑定空间和学生编号。
export const notificationKey = (recipient: NotificationRecipient) =>
  typeof recipient === 'number'
    ? (['notifications', recipient] as const)
    : (['student-notifications', recipient.workspaceID, recipient.studentID] as const);
export const notificationUnreadKey = (recipient: NotificationRecipient) =>
  [...notificationKey(recipient), 'unread'] as const;
const notificationBase = (recipient: NotificationRecipient) =>
  typeof recipient === 'number'
    ? '/v1/notifications'
    : `/v1/workspaces/${recipient.workspaceID}/student-service/notifications`;

export function notificationUnreadOptions(recipient: NotificationRecipient) {
  return queryOptions({
    queryKey: notificationUnreadKey(recipient),
    queryFn: ({ client, queryKey, signal }) => {
      const scheduled = client.getQueryState(queryKey)?.fetchMeta === notificationPollFetchMeta;
      const suffix = scheduled ? '?poll=notification-unread' : '';
      return apiGet(`${notificationBase(recipient)}/unread${suffix}`, unreadSchema, signal);
    },
  });
}

export function notificationListOptions(
  recipient: NotificationRecipient,
  page: number,
  pageSize: number,
  status: NotificationStatus = 'unread',
) {
  return queryOptions({
    queryKey: [...notificationKey(recipient), 'list', status, page, pageSize] as const,
    queryFn: ({ client, queryKey, signal }) => {
      const params = new URLSearchParams({
        page: String(page),
        page_size: String(pageSize),
        status,
      });
      if (client.getQueryState(queryKey)?.fetchMeta === notificationPollFetchMeta) {
        params.set('poll', 'notification-list');
      }
      return apiGet(`${notificationBase(recipient)}?${params.toString()}`, listSchema, signal);
    },
  });
}

export function markNotificationsRead(ids: number[], recipient: NotificationRecipient) {
  return apiPost(`${notificationBase(recipient)}/read`, readSchema, { ids });
}
