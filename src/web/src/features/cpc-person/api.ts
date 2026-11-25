import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

import {
  serializeTimelineFilters,
  type TimelineFilterDraft,
  timelineFilterFieldSchema,
} from '@/features/cpc-person/timeline-filter-model';
import { apiDelete, apiGet, apiPost, apiPut } from '@/lib/api';
import type { UploadProgress } from '@/lib/file-upload';
import { filterParams } from '@/lib/filter-params';
import { saveSpreadsheetFile, spreadsheetFileSchema } from '@/lib/spreadsheet';

export const stages = [
  { value: 'applicant', label: '入党申请人' },
  { value: 'activist', label: '入党积极分子' },
  { value: 'development_candidate', label: '发展对象' },
  { value: 'probationary_member', label: '中共预备党员' },
  { value: 'full_member', label: '中共党员' },
] as const;

const stageSchema = z.enum([
  'applicant',
  'activist',
  'development_candidate',
  'probationary_member',
  'full_member',
]);
export const defaultStatus = { value: '在编', label: '在编' };
const inputSchema = z.object({
  student_id: z.number().int().positive(),
  timeline_template_id: z.number().int().positive().nullable(),
  stage: stageSchema,
  status: z
    .string()
    .trim()
    .min(1)
    .max(8)
    .refine((value) => !/\p{Cc}/u.test(value)),
  party_position: z
    .string()
    .trim()
    .max(64)
    .refine((value) => !/\p{Cc}/u.test(value)),
});
const studentSchema = z.object({
  student_no: z.string().max(32),
  name: z.string().max(50),
  gender: z.string().max(16),
  ethnicity: z.string().max(32),
  grade: z.string().max(16),
  student_status: z.string().max(8),
  education_level: z.string().max(32),
  class_name: z.string().max(64),
  major: z.string().max(100),
  advisor: z.string().max(50),
  phone: z.string().max(32),
  arrival_month: z.string().max(7),
  previous_school_or_employer: z.string().max(255),
  previous_graduation_month: z.string().max(7),
});
export const personSchema = inputSchema.extend(studentSchema.shape).extend({
  party_branch_id: z.number().int().positive(),
  id: z.number().int().positive(),
  party_branch_name: z.string(),
  party_branch_short_name: z.string(),
  party_branch_secretaries: z.array(z.string()).transform((names) => names.toSorted()),
  revision: z.number().int().positive(),
  can_update: z.boolean(),
  can_delete: z.boolean(),
  timeline_template_name: z.string(),
  timeline_pending: z.number().int().nonnegative(),
  timeline_completed: z.number().int().nonnegative(),
  timeline_skipped: z.number().int().nonnegative(),
  timeline_status: z.enum(['normal', 'abnormal', 'transferring']).nullable(),
  can_review_timeline_transfer: z.boolean().optional(),
  created_at: z.iso.datetime({ offset: true }),
  updated_at: z.iso.datetime({ offset: true }),
});
const listSchema = z.object({
  items: z.array(personSchema),
  total: z.number().int().nonnegative(),
  page: z.number().int().positive(),
  page_size: z.number().int().positive(),
  can_create: z.boolean(),
  can_import: z.boolean(),
  can_export: z.boolean(),
});
const optionSchema = z.object({ value: z.string(), label: z.string() });
const candidateSchema = studentSchema.extend({
  value: z.number().int().positive(),
  label: z.string(),
  party_branch_id: z.number().int().positive(),
  party_branch_name: z.string(),
});
const fileSchema = spreadsheetFileSchema.extend({
  filename: z.enum(['cpc-people.xlsx', 'cpc-people-full.xlsx', 'cpc-people-template.xlsx']),
});
export type Person = z.infer<typeof personSchema>;
export type PersonInput = z.infer<typeof inputSchema>;
export type Candidate = z.infer<typeof candidateSchema>;
export type StudentInfo = z.infer<typeof studentSchema>;
export type PersonFile = z.infer<typeof fileSchema>;
export function savePersonFile(file: PersonFile) {
  saveSpreadsheetFile(file);
}
export const timelineStatuses = [
  { value: 'normal', label: '正常' },
  { value: 'abnormal', label: '异常' },
  { value: 'transferring', label: '转移中' },
] as const;
export const timelineStatusLabel = (value: string | null) =>
  timelineStatuses.find((status) => status.value === value)?.label ?? '-';
export const nodeStatuses = [
  { value: 'pending', label: '待处理' },
  { value: 'skipped', label: '已跳过' },
  { value: 'completed', label: '已完成' },
] as const;
export const filterFields = [
  { key: 'timeline_status', label: '时间线状态' },
  { key: 'stage', label: '发展阶段' },
  { key: 'party_branch_id', label: '所属党支部' },
  { key: 'party_position', label: '党内职务' },
  { key: 'grade', label: '年级' },
  { key: 'status', label: '状态' },
  { key: 'education_level', label: '学历' },
  { key: 'major', label: '专业' },
  { key: 'class_name', label: '班级' },
  { key: 'advisor', label: '导师' },
] as const;
type PersonFilterKey = (typeof filterFields)[number]['key'];
export type PersonFilters = Record<PersonFilterKey, string[]> & {
  keyword: string;
  timeline_template: z.infer<typeof optionSchema> | null;
  timeline_filters: TimelineFilterDraft[];
  node_status: string[];
};
export const emptyFilters: PersonFilters = {
  keyword: '',
  timeline_template: null,
  timeline_filters: [],
  node_status: [],
  ...(Object.fromEntries(
    filterFields.map(({ key }): [PersonFilterKey, string[]] => [key, []]),
  ) as Record<PersonFilterKey, string[]>),
};
export const defaultFilters: PersonFilters = {
  ...emptyFilters,
  status: [defaultStatus.value],
};
export const personKey = (workspaceID: string) => ['workspace', workspaceID, 'cpc-people'] as const;
const base = (workspaceID: string) => `/v1/workspaces/${workspaceID}/cpc-people`;
export function personFilterParams(filters: PersonFilters, nodeView = false) {
  const { timeline_template, timeline_filters, timeline_status, node_status, ...ordinary } =
    filters;
  // 时间线视图使用已选择的模板，人员视图使用筛选区的模板条件。
  const params = filterParams(
    nodeView
      ? { ...ordinary, node_status }
      : { ...ordinary, timeline_template_id: timeline_template?.value ?? '', timeline_status },
  );
  for (const condition of serializeTimelineFilters(timeline_filters)) {
    params.append('timeline_filter', JSON.stringify(condition));
  }
  return params;
}
export const personTimelineFilterFieldOptions = (workspaceID: string, keyword: string) =>
  queryOptions({
    queryKey: [...personKey(workspaceID), 'timeline-fields', keyword],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID)}/timeline-fields?${new URLSearchParams({ keyword })}`,
        z.array(timelineFilterFieldSchema),
        signal,
      ),
    retry: false,
  });
export const personListOptions = (
  workspaceID: string,
  filters: PersonFilters,
  page: number,
  pageSize: number,
) =>
  queryOptions({
    queryKey: [...personKey(workspaceID), 'list', filters, page, pageSize],
    queryFn: ({ signal }) => {
      const params = personFilterParams(filters);
      params.set('page', String(page));
      params.set('page_size', String(pageSize));
      return apiGet(`${base(workspaceID)}?${params}`, listSchema, signal);
    },
    retry: false,
  });
export const personDetailOptions = (workspaceID: string, id: number) =>
  queryOptions({
    queryKey: [...personKey(workspaceID), 'detail', id],
    queryFn: ({ signal }) => apiGet(`${base(workspaceID)}/${String(id)}`, personSchema, signal),
    retry: false,
  });
export const personOptions = (workspaceID: string, field: string) =>
  queryOptions({
    queryKey: [...personKey(workspaceID), 'options', field],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID)}/options?field=${encodeURIComponent(field)}`,
        z.array(optionSchema),
        signal,
      ),
    retry: false,
  });
export const personBranchOptions = (workspaceID: string, operation = 'list') =>
  queryOptions({
    queryKey: [...personKey(workspaceID), 'branches', operation],
    queryFn: ({ signal }) =>
      apiGet(`${base(workspaceID)}/branches?operation=${operation}`, z.array(optionSchema), signal),
    retry: false,
  });
export const personCandidateOptions = (workspaceID: string, keyword: string) =>
  queryOptions({
    queryKey: [...personKey(workspaceID), 'candidates', keyword],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID)}/student-options?keyword=${encodeURIComponent(keyword)}`,
        z.array(candidateSchema),
        signal,
      ),
    retry: false,
  });
export function savePerson(workspaceID: string, original: Person | null, values: PersonInput) {
  const parsed = inputSchema.safeParse(values);
  if (!parsed.success) throw new Error('请选择学生和发展阶段，并检查状态、时间线模板及职务格式');
  if (!original && !parsed.data.timeline_template_id) throw new Error('新增人员必须绑定时间线模板');
  const { timeline_template_id: templateID, ...personValues } = parsed.data;
  const body = original
    ? { ...personValues, revision: original.revision }
    : { ...personValues, timeline_template_id: templateID, revision: 0 };
  return original
    ? apiPut(`${base(workspaceID)}/${String(original.id)}`, personSchema, body)
    : apiPost(base(workspaceID), personSchema, body);
}
export const deletePerson = (workspaceID: string, person: Person) =>
  apiDelete(
    `${base(workspaceID)}/${String(person.id)}?revision=${String(person.revision)}`,
    z.null(),
  );
export const personTemplate = (workspaceID: string) =>
  apiGet(`${base(workspaceID)}/template`, fileSchema);
export const importPeople = (workspaceID: string, content: string, onProgress?: UploadProgress) =>
  apiPost(
    `${base(workspaceID)}/import`,
    z.object({ created: z.number().int().positive() }),
    { content },
    onProgress,
  );
export const exportPeople = (
  workspaceID: string,
  filters: PersonFilters,
  format: 'basic' | 'full' = 'basic',
  templateID?: number,
  nodeID?: number,
) => {
  const params = personFilterParams(filters, Boolean(nodeID));
  params.set('format', format);
  if (templateID) params.set('timeline_template_id', String(templateID));
  if (nodeID) params.set('timeline_node_id', String(nodeID));
  return apiGet(`${base(workspaceID)}/export?${params}`, fileSchema);
};
export const stageLabel = (value: string) =>
  stages.find((stage) => stage.value === value)?.label ?? value;

const statisticsSchema = z
  .object({
    total: z.number().int().nonnegative(),
    stages: z
      .array(optionSchema.extend({ value: stageSchema, count: z.number().int().nonnegative() }))
      .length(stages.length)
      .refine((items) => new Set(items.map(({ value }) => value)).size === stages.length),
  })
  .refine((value) => value.stages.reduce((sum, item) => sum + item.count, 0) === value.total);
export type PersonStatistics = z.infer<typeof statisticsSchema>;
export const personStatisticsOptions = (workspaceID: string, filters: PersonFilters) =>
  queryOptions({
    queryKey: [...personKey(workspaceID), 'statistics', filters],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID)}/statistics?${personFilterParams(filters)}`,
        statisticsSchema,
        signal,
      ),
    retry: false,
  });
