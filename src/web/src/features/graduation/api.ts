import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

import { apiDelete, apiGet, apiPost, apiPut } from '@/lib/api';
import { filterParams } from '@/lib/filter-params';
import { spreadsheetFileSchema } from '@/lib/spreadsheet';

export const pendingOption = { value: '', label: '待就业' };
export const unsetGraduationYearOption = { value: '', label: '未设置' };
const yearSchema = z
  .string()
  .refine(
    (value) =>
      value === '' || (/^\d{4}$/.test(value) && Number(value) >= 1900 && Number(value) <= 9999),
    '毕业年份须为 1900 至 9999 之间的四位年份',
  );
const option = z.object({ value: z.string(), label: z.string() });
const fields = {
  graduation_year: yearSchema,
  type: z.string().max(71),
  type_label: z.string().max(100),
  destination: z.string().max(255),
};
const inputSchema = z.object(fields).extend({ type_label: z.string().trim().max(100) });
const recordSchema = z.object({
  ...fields,
  student_id: z.number().int().positive(),
  student_no: z.string(),
  student_name: z.string(),
  education_level: z.string(),
  major: z.string(),
  grade: z.string(),
  revision: z.number().int().positive(),
  created_at: z.iso.datetime({ offset: true }),
  updated_at: z.iso.datetime({ offset: true }),
});
const statisticSchema = z.object({
  graduation_year: yearSchema,
  value: z.string(),
  label: z.string(),
  total: z.number().int().nonnegative(),
  employed: z.number().int().nonnegative(),
  pending: z.number().int().nonnegative(),
  employment_rate: z.number().min(0).max(1),
});
const mutationSchema = z.object({
  student_id: z.number().int().positive(),
  revision: z.number().int().positive(),
});
export type Graduation = z.infer<typeof recordSchema>;
export type GraduationInput = z.infer<typeof inputSchema>;
export type GraduationStatistic = z.infer<typeof statisticSchema>;
export type GraduationOption = z.infer<typeof option>;
export type Dimension = 'major' | 'type';
export interface GraduationFilters {
  keyword: string;
  education_level: string[];
  major: string[];
  grade: string[];
  // 空数组表示全部年份，空字符串选项表示未设置。
  graduation_year: string[];
  // 空数组表示全部类型，空字符串选项表示待就业。
  type: string[];
}
export const emptyFilters: GraduationFilters = {
  keyword: '',
  education_level: [],
  major: [],
  grade: [],
  graduation_year: [],
  type: [],
};
export const graduationKey = (workspaceID: string) =>
  ['workspace', workspaceID, 'graduation'] as const;
const base = (workspaceID: string) => `/v1/workspaces/${workspaceID}/graduation-destinations`;
const parameters = (filters: GraduationFilters) => filterParams({ ...filters });

export const graduationListOptions = (
  workspaceID: string,
  filters: GraduationFilters,
  page: number,
  size: number,
) =>
  queryOptions({
    queryKey: [...graduationKey(workspaceID), 'list', filters, page, size],
    queryFn: ({ signal }) => {
      const query = parameters(filters);
      query.set('page', String(page));
      query.set('page_size', String(size));
      return apiGet(
        `${base(workspaceID)}?${query.toString()}`,
        z.object({
          items: z.array(recordSchema),
          total: z.number().int().nonnegative(),
          page: z.number().int().positive(),
          page_size: z.number().int().positive(),
        }),
        signal,
      );
    },
    retry: false,
  });

export const graduationStatisticsOptions = (
  workspaceID: string,
  filters: GraduationFilters,
  dimension: Dimension,
) =>
  queryOptions({
    queryKey: [...graduationKey(workspaceID), 'statistics', dimension, filters],
    queryFn: ({ signal }) => {
      const query = parameters(filters);
      query.set('dimension', dimension);
      return apiGet(
        `${base(workspaceID)}/statistics?${query.toString()}`,
        z.array(statisticSchema),
        signal,
      );
    },
    retry: false,
  });

export const graduationFilterOptions = (workspaceID: string, statistics = false) =>
  queryOptions({
    queryKey: [...graduationKey(workspaceID), 'filter-options', statistics],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID)}/${statistics ? 'statistics/' : ''}filter-options`,
        z.object({
          education_levels: z.array(option),
          majors: z.array(option),
          grades: z.array(option),
          graduation_years: z.array(option),
          types: z.array(option),
        }),
        signal,
      ),
    retry: false,
  });

export function saveGraduation(workspaceID: string, record: Graduation, values: GraduationInput) {
  return apiPut(`${base(workspaceID)}/${String(record.student_id)}`, mutationSchema, {
    ...inputSchema.parse(values),
    revision: record.revision,
  });
}

export function clearGraduation(workspaceID: string, record: Graduation) {
  return apiDelete(
    `${base(workspaceID)}/${String(record.student_id)}?revision=${String(record.revision)}`,
    z.null(),
  );
}

export function graduationTemplate(workspaceID: string) {
  return apiGet(
    `${base(workspaceID)}/template`,
    spreadsheetFileSchema.extend({
      filename: z.literal('graduation-destinations.xlsx'),
    }),
  );
}

export function exportGraduations(workspaceID: string, filters: GraduationFilters) {
  return apiGet(
    `${base(workspaceID)}/export?${parameters(filters)}`,
    spreadsheetFileSchema.extend({ filename: z.literal('毕业去向.xlsx') }),
  );
}

export function importGraduations(workspaceID: string, content: string) {
  return apiPost(
    `${base(workspaceID)}/import`,
    z.object({ created: z.number().int().positive() }),
    { content },
  );
}
