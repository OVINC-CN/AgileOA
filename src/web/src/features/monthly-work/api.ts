import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

import type { WorkCategory } from '@/features/monthly-work/category';
import { apiDelete, apiGet, apiPost, apiPut } from '@/lib/api';
import { getConfig } from '@/lib/config';
import { uploadFile } from '@/lib/cos-upload';
import { filterParams } from '@/lib/filter-params';
import { spreadsheetFileSchema } from '@/lib/spreadsheet';

export function isExternalURL(value: string) {
  try {
    const url = new URL(value);
    return (
      (url.protocol === 'http:' || url.protocol === 'https:') &&
      url.hostname !== '' &&
      url.username === '' &&
      url.password === '' &&
      !/[\s\\]/u.test(value)
    );
  } catch {
    return false;
  }
}

const linkSchema = z.object({
  name: z.string().trim().max(100),
  url: z.string().trim().min(1).max(2048).refine(isExternalURL),
});
const inputSchema = z.object({
  date: z.iso.date().refine((value) => value >= '1900-01-01' && value <= '9999-12-31'),
  work_item: z.string().trim().min(1).max(100),
  owner_id: z.number().int().positive(),
  participant_ids: z.array(z.number().int().positive()),
  description: z.string().max(10000),
  links: z.array(linkSchema),
});
const recordSchema = inputSchema.extend({
  participants: z.array(
    z.object({
      id: z.number().int().positive(),
      name: z.string(),
      username: z.string(),
    }),
  ),
  attachment_ids: z.array(z.number().int().positive()),
  attachments: z.array(
    z.object({
      id: z.number().int().positive(),
      filename: z.string(),
      size: z.number().int().nonnegative(),
    }),
  ),
  owner_name: z.string(),
  owner_username: z.string(),
  id: z.number().int().positive(),
  revision: z.number().int().positive(),
  created_at: z.iso.datetime({ offset: true }),
  updated_at: z.iso.datetime({ offset: true }),
});
const fileSchema = z.object({
  id: z.number().int().positive(),
  monthly_work_id: z.number().int().positive(),
  category: z.enum(['monthly_work', 'cpc_monthly_work']),
  filename: z.string(),
  size: z.number().int().nonnegative(),
  state: z.enum(['pending', 'ready', 'deleting']),
  revision: z.number().int().positive(),
  created_at: z.iso.datetime({ offset: true }),
  updated_at: z.iso.datetime({ offset: true }),
});
const workFileSchema = (category: WorkCategory) =>
  fileSchema.extend({
    category: z.literal(category === 'cpc' ? 'cpc_monthly_work' : 'monthly_work'),
  });
const list = <T extends z.ZodType>(schema: T) =>
  z.object({
    items: z.array(schema),
    total: z.number().int().nonnegative(),
    page: z.number().int().positive(),
    page_size: z.number().int().positive(),
  });

export type WorkRecord = z.infer<typeof recordSchema>;
export type WorkInput = z.infer<typeof inputSchema>;
export type WorkLink = z.infer<typeof linkSchema>;
export type WorkAttachment = z.infer<typeof fileSchema>;
export interface WorkFilters {
  scope: string[];
  year: string[];
  month: string[];
  keyword: string;
}
export const workKey = (workspaceID: string, category: WorkCategory = 'student') =>
  ['workspace', workspaceID, 'monthly-work', category] as const;
const base = (workspaceID: string) => `/v1/workspaces/${workspaceID}`;
const resource = (category: WorkCategory) =>
  category === 'cpc' ? 'cpc-monthly-work' : 'monthly-work';

export const workListOptions = (
  workspaceID: string,
  filters: WorkFilters,
  category: WorkCategory = 'student',
) => {
  return queryOptions({
    queryKey: [...workKey(workspaceID, category), 'list', filters],
    queryFn: ({ signal }) => {
      const parameters = filterParams({ ...filters });
      return apiGet(
        `${base(workspaceID)}/${resource(category)}?${parameters}`,
        z.object({
          items: z.array(recordSchema),
          total: z.number().int().nonnegative(),
        }),
        signal,
      );
    },
    retry: false,
  });
};

export function saveWork(
  workspaceID: string,
  current: WorkRecord | null,
  values: WorkInput,
  category: WorkCategory = 'student',
) {
  const parsed = inputSchema.safeParse(values);
  if (!parsed.success)
    throw new Error('请检查日期、工作项目、负责人、参与人及外部链接的格式和长度');
  if (parsed.data.links.length > getConfig().monthlyWorkMaxLinks)
    throw new Error('外部链接数量超过限制');
  const body = { ...parsed.data, revision: current?.revision ?? 0 };
  return current
    ? apiPut(`${base(workspaceID)}/${resource(category)}/${String(current.id)}`, recordSchema, body)
    : apiPost(`${base(workspaceID)}/${resource(category)}`, recordSchema, body);
}

export function deleteWork(
  workspaceID: string,
  record: WorkRecord,
  category: WorkCategory = 'student',
) {
  return apiDelete(
    `${base(workspaceID)}/${resource(category)}/${String(record.id)}?revision=${String(record.revision)}`,
    z.null(),
  );
}

export const workAttachmentOptions = (
  workspaceID: string,
  workID: number,
  page: number,
  size: number,
  category: WorkCategory = 'student',
) =>
  queryOptions({
    queryKey: [...workKey(workspaceID, category), 'attachments', workID, page, size],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID)}/attachments/${resource(category)}?monthly_work_id=${String(workID)}&page=${String(page)}&page_size=${String(size)}`,
        list(workFileSchema(category)),
        signal,
      ),
    retry: false,
  });

export function uploadWorkAttachment(
  workspaceID: string,
  workID: number,
  file: File,
  category: WorkCategory = 'student',
) {
  return uploadFile(
    `${base(workspaceID)}/${resource(category)}/${String(workID)}/attachments`,
    workFileSchema(category),
    file,
  );
}

export function deleteWorkAttachment(
  workspaceID: string,
  file: WorkAttachment,
  category: WorkCategory = 'student',
) {
  return apiDelete(
    `${base(workspaceID)}/attachments/${resource(category)}/${String(file.id)}?revision=${String(file.revision)}`,
    z.null(),
  );
}

export function workAttachmentDownload(
  workspaceID: string,
  id: number,
  category: WorkCategory = 'student',
) {
  return `${getConfig().apiBaseURL.replace(/\/$/, '')}${base(workspaceID)}/attachments/${resource(category)}/${String(id)}/download`;
}

export function workTemplate(workspaceID: string, category: WorkCategory = 'student') {
  return apiGet(
    `${base(workspaceID)}/${resource(category)}/template`,
    spreadsheetFileSchema.extend({
      filename: z.literal('monthly-work.xlsx'),
    }),
  );
}

export function importWork(
  workspaceID: string,
  content: string,
  category: WorkCategory = 'student',
) {
  return apiPost(
    `${base(workspaceID)}/${resource(category)}/import`,
    z.object({ created: z.number().int().positive() }),
    { content },
  );
}

export const workOwnerOptions = (workspaceID: string, category: WorkCategory = 'student') =>
  queryOptions({
    queryKey: [...workKey(workspaceID, category), 'owners'],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID)}/${resource(category)}/owners`,
        z.array(z.object({ value: z.number().int().positive(), label: z.string() })),
        signal,
      ),
    retry: false,
  });

export const workFilterOptions = (workspaceID: string, category: WorkCategory = 'student') =>
  queryOptions({
    queryKey: [...workKey(workspaceID, category), 'filter-options'],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID)}/${resource(category)}/filter-options`,
        z.object({
          years: z.array(z.object({ value: z.string().regex(/^[0-9]{4}$/u), label: z.string() })),
        }),
        signal,
      ),
    retry: false,
  });
