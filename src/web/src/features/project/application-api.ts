import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

import {
  approvalStepSchema,
  maxParticipationRank,
  projectFilterOptionsSchema,
  projectKey,
  projectSchema,
  scoreItemSchema,
} from '@/features/project/api';
import { apiDelete, apiGet, apiPost, apiPut } from '@/lib/api';
import { getConfig } from '@/lib/config';
import { uploadFile } from '@/lib/cos-upload';
import type { UploadProgress } from '@/lib/file-upload';
import { filterParams } from '@/lib/filter-params';
import { spreadsheetFileSchema } from '@/lib/spreadsheet';

export const approverKinds = [
  { value: 'class_position', label: '班委' },
  { value: 'applicant', label: '申报人' },
  { value: 'student', label: '学生用户' },
  { value: 'user', label: '组织用户' },
];
export const approvalSignModes = [
  { value: 'any', label: '或签' },
  { value: 'all', label: '并签' },
];
export const applicationStatuses = [
  { value: 'draft', label: '草稿' },
  { value: 'pending', label: '审批中' },
  { value: 'rejected', label: '已驳回' },
  { value: 'approved', label: '已通过' },
];
export const applicationOperations = [
  { value: 'draft', label: '保存草稿' },
  { value: 'submit', label: '提交' },
  { value: 'update', label: '修改内容' },
  { value: 'approve', label: '通过' },
  { value: 'reject', label: '驳回' },
  { value: 'reset', label: '重置审批流' },
];
export type ApplicationScope = 'manage' | 'mine' | 'student-reviews';
export type ApplicationOperation = 'update' | 'submit' | 'approve' | 'reject';
export interface ApplicationFilters {
  student: string;
  project_name: string[];
  year: string[];
  status: string[];
}
const applicationContent = z.object({
  statement: z.string().trim().max(10000),
  rank: z.number().int().positive().max(maxParticipationRank).nullable(),
  score: z.number().nullable(),
  details: z.array(
    scoreItemSchema.extend({ key: z.string().regex(/^[0-9a-f]{32}$/, '子项标识无效') }),
  ),
});
const summarySchema = z.object({
  id: z.number().int().positive(),
  project_id: z.number().int().positive(),
  project_name: z.string(),
  allow_self_apply: z.boolean(),
  year: z.number().int(),
  student_id: z.number().int().positive(),
  student_no: z.string(),
  student_name: z.string(),
  class_name: z.string(),
  status: z.enum(['draft', 'pending', 'rejected', 'approved']),
  current_step: z.number().int().nonnegative(),
  current_reviewer: z.string(),
  can_edit: z.boolean(),
  can_approve: z.boolean(),
  can_reject: z.boolean(),
  revision: z.number().int().positive(),
  created_at: z.iso.datetime({ offset: true }),
  updated_at: z.iso.datetime({ offset: true }),
});
const historySchema = z
  .object({
    id: z.number().int().positive(),
    step: z.number().int().nonnegative(),
    operation: z.enum(['draft', 'submit', 'update', 'approve', 'reject', 'reset']),
    actor_type: z.enum(['student', 'user', 'system']),
    actor_id: z.number().int().nonnegative(),
    actor_name: z.string(),
    comment: z.string(),
    created_at: z.iso.datetime({ offset: true }),
  })
  .refine((item) => (item.actor_type === 'system' ? item.actor_id === 0 : item.actor_id > 0), {
    message: '流转记录处理人编号无效',
    path: ['actor_id'],
  });
export const applicationAttachmentSchema = z.object({
  id: z.number().int().positive(),
  application_id: z.number().int().positive(),
  detail_id: z.number().int().positive(),
  category: z.literal('project_application_detail'),
  filename: z.string(),
  size: z.number().int().nonnegative(),
  state: z.enum(['pending', 'ready', 'deleting']),
  revision: z.number().int().positive(),
  created_at: z.iso.datetime({ offset: true }),
  updated_at: z.iso.datetime({ offset: true }),
});
export type ApplicationAttachment = z.infer<typeof applicationAttachmentSchema>;
export const applicationSchema = summarySchema.extend({
  project_description: z.string(),
  score_calculation_enabled: z.boolean().optional(),
  self_apply_deadline: z.iso.datetime({ offset: true }).nullable(),
  ...applicationContent.shape,
  details: z.array(
    applicationContent.shape.details.element.extend({
      id: z.number().int().positive(),
      attachments: z.array(applicationAttachmentSchema),
    }),
  ),
  steps: z.array(
    approvalStepSchema.extend({
      position: z.number().int().positive(),
      ready: z.boolean(),
      reviewers: z.array(
        z.object({
          identity_type: z.enum(['student', 'user']),
          id: z.number().int().positive(),
          label: z.string(),
          approved: z.boolean(),
        }),
      ),
    }),
  ),
  detail_names: z.array(z.string()),
  history: z.array(historySchema),
});
export type ApplicationSummary = z.infer<typeof summarySchema>;
export type Application = z.infer<typeof applicationSchema>;
export type ApplicationContent = z.infer<typeof applicationContent>;
export const applicationVersionSchema = z.object({
  revision: z.number().int().positive(),
  actor_name: z.string(),
  updated_at: z.iso.datetime({ offset: true }),
  statement: z.string(),
  rank: z.number().int().positive().nullable(),
  score: z.number().nullable(),
  details: z.array(
    scoreItemSchema.extend({
      id: z.number().int().positive(),
      detail_code: z.string(),
      attachments: z.array(z.object({ filename: z.string() })),
    }),
  ),
});
export type ApplicationVersion = z.infer<typeof applicationVersionSchema>;
const applicationVersionsSchema = z.object({
  items: z.array(applicationVersionSchema).max(3),
});
const listSchema = <T extends z.ZodType>(schema: T) =>
  z.object({
    items: z.array(schema),
    total: z.number().int().nonnegative(),
    page: z.number().int().positive(),
    page_size: z.number().int().positive(),
  });
const base = (workspaceID: string) => `/v1/workspaces/${workspaceID}`;
const scopePath = (scope: ApplicationScope) =>
  ({
    manage: 'project-applications',
    mine: 'student-service/project-applications',
    'student-reviews': 'student-service/project-reviews',
  })[scope];
export const applicationListOptions = (
  workspaceID: string,
  scope: ApplicationScope,
  filters: ApplicationFilters,
  page: number,
  pageSize: number,
) =>
  queryOptions({
    queryKey: [...projectKey(workspaceID), 'applications', scope, filters, page, pageSize],
    queryFn: ({ signal }) => {
      const params = filterParams({
        ...filters,
        page: String(page),
        page_size: String(pageSize),
      });
      return apiGet(
        `${base(workspaceID)}/${scopePath(scope)}?${params.toString()}`,
        listSchema(summarySchema),
        signal,
      );
    },
    retry: false,
  });

export function exportApplicationRecords(workspaceID: string, filters: ApplicationFilters) {
  return apiGet(
    `${base(workspaceID)}/project-applications/export?${filterParams({ ...filters }).toString()}`,
    spreadsheetFileSchema.extend({ filename: z.literal('填报明细.xlsx') }),
  );
}

export function deleteApplication(workspaceID: string, record: ApplicationSummary) {
  return apiDelete(
    `${base(workspaceID)}/project-applications/${String(record.id)}?revision=${String(record.revision)}`,
    z.null(),
  );
}

export const applicationProjectFilterOptions = (workspaceID: string, scope: ApplicationScope) =>
  queryOptions({
    queryKey: [...projectKey(workspaceID), 'application-filter-options', scope],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID)}/${scopePath(scope)}/filter-options`,
        projectFilterOptionsSchema,
        signal,
      ),
    retry: false,
  });
export const applicationDetailOptions = (
  workspaceID: string,
  scope: ApplicationScope,
  id: number,
) =>
  queryOptions({
    queryKey: [...projectKey(workspaceID), 'application', scope, id],
    queryFn: ({ signal }) =>
      apiGet(`${base(workspaceID)}/${scopePath(scope)}/${String(id)}`, applicationSchema, signal),
    retry: false,
  });
export const applicationVersionsOptions = (
  workspaceID: string,
  scope: ApplicationScope,
  record: Application,
) =>
  queryOptions({
    queryKey: [
      ...projectKey(workspaceID),
      'application-versions',
      scope,
      record.id,
      record.revision,
      record.self_apply_deadline,
    ],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID)}/${scopePath(scope)}/${String(record.id)}/versions`,
        applicationVersionsSchema,
        signal,
      ),
    retry: false,
  });
export const studentProjectOptions = (workspaceID: string, keyword: string) =>
  queryOptions({
    queryKey: [...projectKey(workspaceID), 'self-apply', keyword],
    queryFn: ({ signal }) => {
      const params = new URLSearchParams({
        keyword,
        page_size: String(getConfig().tablePageSize),
      });
      return apiGet(
        `${base(workspaceID)}/student-service/projects?${params.toString()}`,
        listSchema(projectSchema),
        signal,
      );
    },
    retry: false,
  });
export const approvalOptions = (workspaceID: string, kind: string, keyword: string) =>
  queryOptions({
    queryKey: [...projectKey(workspaceID), 'approval-options', kind, keyword],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID)}/projects/approval-options?${new URLSearchParams({ kind, keyword }).toString()}`,
        z.object({ items: z.array(z.object({ value: z.string(), label: z.string() })) }),
        signal,
      ),
    retry: false,
  });
export function createApplication(
  workspaceID: string,
  projectID: number,
  content: ApplicationContent,
  saveAsDraft = false,
) {
  return apiPost(`${base(workspaceID)}/student-service/project-applications`, applicationSchema, {
    ...applicationContent.parse(content),
    project_id: projectID,
    save_as_draft: saveAsDraft,
    revision: 0,
    comment: '',
  });
}
export function changeApplication(
  workspaceID: string,
  scope: ApplicationScope,
  record: Application,
  operation: ApplicationOperation,
  content: ApplicationContent,
  comment: string,
) {
  const path = `${base(workspaceID)}/${scopePath(scope)}/${String(record.id)}`;
  const body = { ...applicationContent.parse(content), revision: record.revision, comment };
  return operation === 'update'
    ? apiPut(path, applicationSchema, body)
    : apiPost(`${path}/${operation}`, applicationSchema, body);
}

export const defaultApplicationFilters = (scope: ApplicationScope): ApplicationFilters => ({
  student: '',
  project_name: [],
  year: [],
  status: scope === 'mine' ? [] : ['pending_mine'],
});

export function applicationStatusOptions(scope: ApplicationScope) {
  const states = applicationStatuses.filter(
    (status) => scope === 'mine' || status.value !== 'draft',
  );
  return scope !== 'mine'
    ? [{ value: 'pending_mine', label: '待我审批', pinned: true }, ...states]
    : states;
}

export function uploadApplicationAttachment(
  workspaceID: string,
  scope: ApplicationScope,
  record: Application,
  key: string,
  file: File,
  onProgress?: UploadProgress,
) {
  return uploadFile(
    `${base(workspaceID)}/${scopePath(scope)}/${String(record.id)}/attachments`,
    applicationSchema,
    file,
    { fields: { detail_key: key, application_revision: record.revision }, onProgress },
  );
}

export function deleteApplicationAttachment(
  workspaceID: string,
  scope: ApplicationScope,
  record: Application,
  file: ApplicationAttachment,
) {
  const query = new URLSearchParams({
    revision: String(file.revision),
    application_revision: String(record.revision),
  });
  return apiDelete(
    `${base(workspaceID)}/${scopePath(scope)}/${String(record.id)}/attachments/${String(file.id)}?${query}`,
    applicationSchema,
  );
}

export function applicationAttachmentDownload(
  workspaceID: string,
  scope: ApplicationScope,
  applicationID: number,
  fileID: number,
) {
  return `${getConfig().apiBaseURL.replace(/\/$/, '')}${base(workspaceID)}/${scopePath(scope)}/${String(applicationID)}/attachments/${String(fileID)}/download`;
}
