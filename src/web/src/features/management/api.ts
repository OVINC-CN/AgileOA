import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

import { parseAuditJSON } from '@/features/management/audit-json';
import { passwordDigest, userSchema } from '@/features/user/api';
import { workspaceIDSchema } from '@/features/workspace/api';
import { apiDelete, apiGet, apiPut } from '@/lib/api';
import { filterParams } from '@/lib/filter-params';

const userRowSchema = userSchema.safeExtend({
  is_enabled: z.boolean(),
  created_at: z.iso.datetime({ offset: true }),
  updated_at: z.iso.datetime({ offset: true }),
  last_login_at: z.iso.datetime({ offset: true }).nullable(),
  session_count: z.number().int().nonnegative(),
  has_api_token: z.boolean(),
});
export type UserRow = z.infer<typeof userRowSchema>;
const userListSchema = z.object({
  items: z.array(userRowSchema),
  total: z.number().int().nonnegative(),
  page: z.number().int().positive(),
  page_size: z.number().int().positive(),
});
const userStatsSchema = z.object({
  total: z.number().int().nonnegative(),
  enabled: z.number().int().nonnegative(),
  disabled: z.number().int().nonnegative(),
  admins: z.number().int().nonnegative(),
  logins: z.number().int().nonnegative(),
  days: z.array(
    z.object({
      date: z.iso.date(),
      created: z.number().int().nonnegative(),
      logins: z.number().int().nonnegative(),
    }),
  ),
});
export type UserStatistics = z.infer<typeof userStatsSchema>;
export interface UserFilters {
  keyword: string;
  status: string[];
}
export interface AuditFilters {
  user_identify_tenant_id: string[];
  user_id: string;
  workspace_id: string[];
  action_id: string[];
  result_code: string[];
  access_type: string[];
  access_source_ip: string;
  instance_id: string;
  request_id: string;
  start_time: string;
  end_time: string;
}

export const excludeAuditAction = 'exclude_audit';
export const defaultAuditFilters: AuditFilters = {
  user_identify_tenant_id: [],
  user_id: '',
  workspace_id: [],
  action_id: [excludeAuditAction],
  result_code: [],
  access_type: [],
  access_source_ip: '',
  instance_id: '',
  request_id: '',
  start_time: '',
  end_time: '',
};

const optionSchema = z.object({ value: z.string(), label: z.string() });
const unsignedIDSchema = z.string().regex(/^(?:0|[1-9]\d*)$/u);
const auditWorkspaceSchema = z.union([workspaceIDSchema, z.literal('system')]);
const auditRowSchema = z.object({
  id: unsignedIDSchema,
  event_id: z.string().regex(/^[0-9a-f]{32}$/u),
  event_date: z.iso.date(),
  workspace_id: auditWorkspaceSchema,
  event_content: z.string().nullable(),
  action_id: z.string(),
  request_id: z.string().regex(/^(?!0{32}$)[0-9a-f]{32}$/u),
  user_id: unsignedIDSchema,
  user_identify_type: z.union([z.literal(1), z.literal(2)]),
  user_identify_tenant_id: auditWorkspaceSchema,
  start_time: z.number().int(),
  end_time: z.number().int(),
  access_type: z.union([z.literal(1), z.literal(2)]),
  access_source_ip: z.string(),
  access_user_agent: z.string(),
  resource_type_id: z.string(),
  instance_id: z.string(),
  instance_name: z.string().nullable(),
  result_code: z.number().int().min(-2147483648).max(2147483647),
  result_content: z.string(),
  snapshot_user_info: z.record(z.string(), z.unknown()),
});
export type AuditRow = z.infer<typeof auditRowSchema>;
const auditListSchema = z.object({
  items: z.array(auditRowSchema),
  total: z.number().int().nonnegative(),
  page: z.number().int().positive(),
  page_size: z.number().int().positive(),
  actions: z.array(optionSchema),
  workspaces: z.array(optionSchema),
  user_tenants: z.array(optionSchema),
  result_codes: z.array(optionSchema),
  start_time: z.number().int(),
  end_time: z.number().int(),
});
const auditDetailSchema = auditRowSchema.extend({
  instance_data: z.unknown().refine((value) => value !== undefined),
  instance_origin_data: z.unknown().refine((value) => value !== undefined),
  extend_data: z.object({ method: z.string(), path: z.string() }),
  has_sensitive_snapshot: z.boolean(),
});
export type AuditDetail = z.infer<typeof auditDetailSchema>;
export type AuditDimension = 'result' | 'workspace' | 'action';
const auditTrendSchema = z
  .object({
    series: z.array(optionSchema).max(11),
    days: z.array(
      z.object({
        date: z.iso.date(),
        counts: z.array(z.number().int().nonnegative()),
      }),
    ),
  })
  .refine(({ series, days }) => days.every((day) => day.counts.length === series.length));
const auditStatsSchema = z.object({
  total: z.number().int().nonnegative(),
  success: z.number().int().nonnegative(),
  failure: z.number().int().nonnegative(),
  denied: z.number().int().nonnegative(),
  days: z.array(
    z.object({
      date: z.iso.date(),
      success: z.number().int().nonnegative(),
      failure: z.number().int().nonnegative(),
    }),
  ),
  trend: auditTrendSchema.optional(),
});
export type AuditStatistics = z.infer<typeof auditStatsSchema>;

export const userListOptions = (filters: UserFilters, page: number, pageSize: number) =>
  queryOptions({
    queryKey: ['management', 'users', 'list', filters, page, pageSize],
    queryFn: ({ signal }) =>
      apiGet(
        `/v1/system/users?${filterParams({ ...filters, page, page_size: pageSize })}`,
        userListSchema,
        signal,
      ),
  });
export const userStatisticsOptions = queryOptions({
  queryKey: ['management', 'users', 'statistics'],
  queryFn: ({ signal }) => apiGet('/v1/system/users/statistics', userStatsSchema, signal),
});
function auditSearch(filters: AuditFilters, pagination: Record<string, number> = {}) {
  const parameters = filterParams({
    ...filters,
    ...pagination,
    user_id: filters.user_id.trim(),
    request_id: filters.request_id.trim(),
    start_time: filters.start_time ? String(Date.parse(filters.start_time)) : '',
    end_time: filters.end_time ? String(Date.parse(filters.end_time)) : '',
  });
  // 全部操作显式传空值，区别于未传参数时后端默认的“不含审计”。
  if (filters.action_id.length === 0) parameters.set('action_id', '');
  return parameters.toString();
}

export const auditListOptions = (filters: AuditFilters, page: number, pageSize: number) =>
  queryOptions({
    queryKey: ['management', 'audit', 'list', filters, page, pageSize],
    queryFn: ({ signal }) =>
      apiGet(
        `/v1/system/audit/logs?${auditSearch(filters, { page, page_size: pageSize })}`,
        auditListSchema,
        signal,
        { parseJSON: parseAuditJSON },
      ),
  });
export const auditStatisticsOptions = (filters: AuditFilters, dimension: AuditDimension) =>
  queryOptions({
    queryKey: ['management', 'audit', 'statistics', filters, dimension],
    queryFn: ({ signal }) => {
      const parameters = new URLSearchParams(auditSearch(filters));
      if (dimension !== 'result') parameters.set('dimension', dimension);
      return apiGet(
        `/v1/system/audit/statistics?${parameters.toString()}`,
        auditStatsSchema.refine((data) => dimension === 'result' || data.trend !== undefined),
        signal,
      );
    },
  });
export const auditDetailOptions = (row: AuditRow) =>
  queryOptions({
    queryKey: ['management', 'audit', 'detail', row.workspace_id, row.event_date, row.event_id],
    queryFn: ({ signal }) =>
      apiGet(
        `/v1/system/audit/logs/${row.event_id}?event_date=${row.event_date}`,
        auditDetailSchema,
        signal,
        { parseJSON: parseAuditJSON },
      ),
  });

export async function updateUser(
  user: UserRow,
  values: {
    display_name: string;
    email: string;
    is_enabled: boolean;
    password_login_enabled: boolean;
    password: string;
  },
) {
  return apiPut(`/v1/system/users/${String(user.id)}`, userSchema, {
    display_name: values.display_name.trim(),
    email: values.email.trim(),
    is_enabled: values.is_enabled,
    password_login_enabled: values.password_login_enabled,
    ...(values.password_login_enabled && values.password
      ? { password_sha256: await passwordDigest(user.username, values.password) }
      : {}),
  });
}
export function deleteUser(id: number) {
  return apiDelete(`/v1/system/users/${String(id)}`, z.null());
}
export function revokeSessions(id: number) {
  return apiDelete(`/v1/system/users/${String(id)}/sessions`, z.null());
}

const apiTokenSchema = z.object({
  token: z.string(),
  created_at: z.iso.datetime({ offset: true }).nullable(),
  updated_at: z.iso.datetime({ offset: true }).nullable(),
});
export type UserAPIToken = z.infer<typeof apiTokenSchema>;

export function getUserAPIToken(id: number, signal?: AbortSignal) {
  return apiGet(`/v1/system/users/${String(id)}/api-token`, apiTokenSchema, signal);
}
export function resetUserAPIToken(id: number) {
  return apiPut(`/v1/system/users/${String(id)}/api-token`, apiTokenSchema, undefined);
}
export function deleteUserAPIToken(id: number) {
  return apiDelete(`/v1/system/users/${String(id)}/api-token`, z.null());
}
