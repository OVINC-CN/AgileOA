import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

import { apiGet, apiPut } from '@/lib/api';

const actionSchema = z.object({
  value: z.string().min(1),
  label: z.string().min(1),
  description: z.string(),
  group: z.object({ value: z.string(), label: z.string() }),
  scope: z.enum(['public', 'session', 'system', 'workspace']),
  endpoints: z.array(z.object({ method: z.string(), path: z.string() })),
});
export type Action = z.infer<typeof actionSchema>;
const accessUserSchema = z.object({
  id: z.number().int().positive(),
  username: z.string().min(1),
  display_name: z.string().min(1),
  is_enabled: z.boolean(),
  permission_count: z.number().int().nonnegative(),
  created_at: z.iso.datetime({ offset: true }),
  updated_at: z.iso.datetime({ offset: true }),
});
export type AccessUser = z.infer<typeof accessUserSchema>;
const actionsSchema = z.object({
  actions: z.array(z.string()),
  revision: z.number().int().positive(),
});
export type UserActions = z.infer<typeof actionsSchema>;

export const actionCatalogOptions = queryOptions({
  queryKey: ['management', 'access', 'catalog'],
  queryFn: ({ signal }) =>
    apiGet('/v1/system/access/actions', z.object({ items: z.array(actionSchema) }), signal),
});

export const accessUsersOptions = (
  workspaceID: string,
  keyword: string,
  page: number,
  pageSize: number,
  permissionStatus: 'granted' | 'ungranted' = 'granted',
) =>
  queryOptions({
    queryKey: [
      'management',
      'access',
      'users',
      workspaceID,
      permissionStatus,
      keyword,
      page,
      pageSize,
    ],
    queryFn: ({ signal }) => {
      const params = new URLSearchParams({
        keyword,
        page: String(page),
        page_size: String(pageSize),
        permission_status: permissionStatus,
      });
      return apiGet(
        `/v1/system/access/workspaces/${workspaceID}/users?${params.toString()}`,
        z.object({
          items: z.array(accessUserSchema),
          total: z.number().int().nonnegative(),
          page: z.number().int().positive(),
          page_size: z.number().int().positive(),
        }),
        signal,
      );
    },
    enabled: workspaceID !== '',
  });

export const userActionsOptions = (workspaceID: string, userID: number) =>
  queryOptions({
    queryKey: ['management', 'access', 'actions', workspaceID, userID],
    queryFn: ({ signal }) =>
      apiGet(
        `/v1/system/access/workspaces/${workspaceID}/users/${String(userID)}/actions`,
        actionsSchema,
        signal,
      ),
    staleTime: 0,
  });

export function setUserActions(workspaceID: string, userID: number, values: UserActions) {
  return apiPut(
    `/v1/system/access/workspaces/${workspaceID}/users/${String(userID)}/actions`,
    actionsSchema,
    values,
  );
}
