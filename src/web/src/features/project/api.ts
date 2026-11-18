import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

import {
  cleanScoreCalculation,
  type ScoreCalculation,
  scoreCalculationSchema,
  type ScoreDimensions,
} from '@/features/project/score-calculation';
import { apiDelete, apiGet, apiPost, apiPut } from '@/lib/api';
import { getConfig } from '@/lib/config';
import { uploadFile } from '@/lib/cos-upload';
import { filterParams } from '@/lib/filter-params';
import { spreadsheetFileSchema } from '@/lib/spreadsheet';

const score = z.number().nullable();
export const maxParticipationRank = 2_147_483_647;
const rank = z
  .number('排名必须为正整数或留空')
  .int('排名必须为正整数或留空')
  .positive('排名必须为正整数或留空')
  .max(maxParticipationRank, '排名超出支持范围')
  .nullable();
export const scoreItemSchema = z.object({
  name: z.string().min(1).max(100),
  description: z.string().max(10000),
  score,
});
export const approvalStepSchema = z.object({
  kind: z.enum(['class_position', 'applicant', 'student', 'user']),
  sign_mode: z.enum(['any', 'all']).default('any'),
  allow_edit: z.boolean().default(true),
  class_positions: z.array(z.string().max(64)).default([]),
  student_ids: z.array(z.number().int().positive()).default([]),
  user_ids: z.array(z.number().int().positive()).default([]),
  reviewer_label: z.string(),
  reviewer_options: z.array(z.object({ value: z.string(), label: z.string() })).default([]),
});
export type ApprovalStep = z.infer<typeof approvalStepSchema>;
const projectInput = z.object({
  score_calculation: scoreCalculationSchema.nullish(),
  name: z.string().trim().min(1).max(100),
  year: z.number().int().min(1900).max(9999),
  description: z.string().max(10000),
  allow_self_apply: z.boolean().default(false),
  self_apply_deadline: z.iso.datetime({ offset: true }).nullable(),
  approval_steps: z.array(approvalStepSchema).default([]),
  detail_names: z
    .array(z.string().trim().min(1).max(100))
    .refine((names) => new Set(names).size === names.length, '子项名称不能重复'),
});
const validProjectInput = projectInput.refine(
  (input) => !input.allow_self_apply || input.self_apply_deadline !== null,
  {
    message: '允许学生自行申报时必须填写截止时间',
    path: ['self_apply_deadline'],
  },
);
const version = {
  id: z.number().int().positive(),
  revision: z.number().int().positive(),
  created_at: z.iso.datetime({ offset: true }),
  updated_at: z.iso.datetime({ offset: true }),
};
export const projectSchema = projectInput.extend({
  ...version,
  attachment_ids: z.array(z.number().int().positive()),
});
const participationInput = z.object({
  project_id: z.number().int().positive(),
  student_id: z.number().int().positive(),
  rank,
  score,
  details: z.array(scoreItemSchema),
});
const participationSummarySchema = participationInput.omit({ details: true }).extend({
  ...version,
  project_name: z.string(),
  year: z.number().int(),
  student_no: z.string(),
  student_name: z.string(),
  education_level: z.string(),
  major: z.string(),
  grade: z.string(),
  detail_count: z.number().int().nonnegative(),
});
const participationSchema = participationSummarySchema.extend({
  details: z.array(scoreItemSchema),
});

const fileSchema = z.object({
  id: z.number().int().positive(),
  project_id: z.number().int().positive(),
  category: z.literal('project'),
  filename: z.string(),
  size: z.number().int().nonnegative(),
  state: z.enum(['pending', 'ready', 'deleting']),
  revision: z.number().int().positive(),
  created_at: z.iso.datetime({ offset: true }),
  updated_at: z.iso.datetime({ offset: true }),
});
const list = <T extends z.ZodType>(item: T) =>
  z.object({
    items: z.array(item),
    total: z.number().int().nonnegative(),
    page: z.number().int().positive(),
    page_size: z.number().int().positive(),
  });
const statisticSchema = z.object({
  name: z.string(),
  year: z.number().int(),
  count: z.number().int().nonnegative(),
});
const optionSchema = z.object({ value: z.number().int().positive(), label: z.string() });
const templateSchema = spreadsheetFileSchema.extend({
  filename: z.enum(['projects.xlsx', 'project-participations.xlsx']),
});
export type Project = z.infer<typeof projectSchema>;
export type ProjectInput = z.infer<typeof projectInput>;
export type ParticipationSummary = z.infer<typeof participationSummarySchema>;
export type Participation = z.infer<typeof participationSchema>;
export type ParticipationInput = z.infer<typeof participationInput>;
export type ScoreItem = z.infer<typeof scoreItemSchema>;
export type Attachment = z.infer<typeof fileSchema>;
export type Statistic = z.infer<typeof statisticSchema>;
export type ProjectExportType = 'projects' | 'students' | 'details';
export const projectExports: { value: ProjectExportType; label: string; action: string }[] = [
  { value: 'projects', label: '导出奖项', action: 'project.export' },
  { value: 'students', label: '导出获奖学生', action: 'project.participation.export' },
  { value: 'details', label: '导出获奖明细', action: 'project.participation.export' },
];
export interface ProjectFilters {
  keyword: string;
  project_name: string[];
  detail_keyword: string;
  student: string;
  education_level: string[];
  major: string[];
  grade: string[];
  year: string[];
  project_id: string;
}
export const emptyFilters: ProjectFilters = {
  keyword: '',
  project_name: [],
  detail_keyword: '',
  student: '',
  education_level: [],
  major: [],
  grade: [],
  year: [],
  project_id: '',
};
export const projectKey = (workspaceID: string) => ['workspace', workspaceID, 'projects'] as const;
const base = (workspaceID: string) => `/v1/workspaces/${workspaceID}`;
export const projectListFilters = (filters: ProjectFilters): ProjectFilters => ({
  ...emptyFilters,
  keyword: filters.keyword,
  project_name: filters.project_name,
  year: filters.year,
});
export const projectListOptions = (
  workspaceID: string,
  filters: ProjectFilters,
  page: number,
  size: number,
) =>
  queryOptions({
    queryKey: [...projectKey(workspaceID), 'list', filters, page, size],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID)}/projects?${parameters(projectListFilters(filters), page, size)}`,
        list(projectSchema),
        signal,
      ),
    retry: false,
  });
export const participationListOptions = (
  workspaceID: string,
  filters: ProjectFilters,
  page: number,
  size: number,
) =>
  queryOptions({
    queryKey: [...projectKey(workspaceID), 'participations', filters, page, size],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID)}/project-participations?${parameters(filters, page, size)}`,
        list(participationSummarySchema),
        signal,
      ),
    retry: false,
  });
export const statisticsOptions = (workspaceID: string, filters: ProjectFilters) =>
  queryOptions({
    queryKey: [...projectKey(workspaceID), 'statistics', filters],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID)}/projects/statistics?${filterParams({ ...filters })}`,
        z.array(statisticSchema),
        signal,
      ),
    retry: false,
  });
export const projectOptions = (workspaceID: string, students = false, keyword = '') =>
  queryOptions({
    queryKey: [...projectKey(workspaceID), 'options', students ? 'students' : 'projects', keyword],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID)}/${students ? 'project-participations/students' : 'projects/options'}?keyword=${encodeURIComponent(keyword)}`,
        z.array(optionSchema),
        signal,
      ),
    retry: false,
  });
export const attachmentListOptions = (
  workspaceID: string,
  page: number,
  size: number,
  projectID = 0,
) =>
  queryOptions({
    queryKey: [...projectKey(workspaceID), 'attachments', projectID, page, size],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID)}/attachments/project?page=${String(page)}&page_size=${String(size)}${projectID ? `&project_id=${String(projectID)}` : ''}`,
        list(fileSchema),
        signal,
      ),
    retry: false,
  });
function parameters(filters: ProjectFilters, page: number, size: number) {
  const result = filterParams({ ...filters });
  result.set('page', String(page));
  result.set('page_size', String(size));
  return result.toString();
}
export function previewScoreCalculation(
  workspaceID: string,
  projectID: number | undefined,
  calculation: ScoreCalculation,
  names: string[],
  dimensions: ScoreDimensions,
  scores: Record<string, number | null>,
) {
  const path = projectID
    ? `/projects/${String(projectID)}/score-preview`
    : '/projects/score-preview';
  return apiPost(
    `${base(workspaceID)}${path}`,
    z.object({ matched_rule: z.number().int().positive(), score: z.number() }),
    {
      score_calculation: cleanScoreCalculation(calculation),
      detail_names: names,
      dimensions,
      details: names.map((name) => ({ name, description: '', score: scores[name] ?? null })),
    },
  );
}

export function saveProject(workspaceID: string, current: Project | null, values: ProjectInput) {
  const body = {
    ...validProjectInput.parse(values),
    score_calculation: cleanScoreCalculation(values.score_calculation),
    revision: current?.revision ?? 0,
  };
  return current
    ? apiPut(`${base(workspaceID)}/projects/${String(current.id)}`, projectSchema, body)
    : apiPost(`${base(workspaceID)}/projects`, projectSchema, body);
}
export function saveParticipation(
  workspaceID: string,
  current: Participation | null,
  values: ParticipationInput,
) {
  const body = { ...participationInput.parse(values), revision: current?.revision ?? 0 };
  return current
    ? apiPut(
        `${base(workspaceID)}/project-participations/${String(current.id)}`,
        participationSchema,
        body,
      )
    : apiPost(`${base(workspaceID)}/project-participations`, participationSchema, body);
}
export function deleteRecord(
  workspaceID: string,
  record: Project | ParticipationSummary,
  participation: boolean,
) {
  return apiDelete(
    `${base(workspaceID)}/${participation ? 'project-participations' : 'projects'}/${String(record.id)}?revision=${String(record.revision)}`,
    z.null(),
  );
}
export function uploadAttachment(workspaceID: string, projectID: number, file: File) {
  return uploadFile(
    `${base(workspaceID)}/projects/${String(projectID)}/attachments`,
    fileSchema,
    file,
  );
}
export function deleteAttachment(workspaceID: string, file: Attachment) {
  return apiDelete(
    `${base(workspaceID)}/attachments/${String(file.id)}?revision=${String(file.revision)}`,
    z.null(),
  );
}
export function attachmentDownload(workspaceID: string, id: number) {
  return `${getConfig().apiBaseURL.replace(/\/$/, '')}${base(workspaceID)}/attachments/${String(id)}/download`;
}
export function importRecords(workspaceID: string, participation: boolean, content: string) {
  return apiPost(
    `${base(workspaceID)}/${participation ? 'project-participations' : 'projects'}/import`,
    z.object({ created: z.number().int().positive() }),
    { content },
  );
}
export function downloadTemplate(workspaceID: string, participation: boolean) {
  return apiGet(
    `${base(workspaceID)}/${participation ? 'project-participations' : 'projects'}/template`,
    templateSchema,
  );
}

export function exportRecords(
  workspaceID: string,
  type: ProjectExportType,
  filters: ProjectFilters,
) {
  // 项目导出使用项目列表的筛选目录，学生和明细共用参与记录筛选。
  const participation = type !== 'projects';
  const parameters = filterParams({ ...(participation ? filters : projectListFilters(filters)) });
  if (participation) parameters.set('format', type === 'details' ? 'full' : 'basic');
  const filenames = { projects: '奖项.xlsx', students: '获奖学生.xlsx', details: '获奖明细.xlsx' };
  return apiGet(
    `${base(workspaceID)}/${participation ? 'project-participations' : 'projects'}/export?${parameters}`,
    spreadsheetFileSchema.extend({
      filename: z.literal(filenames[type]),
    }),
  );
}

export const participationDetailOptions = (workspaceID: string, id: number) =>
  queryOptions({
    queryKey: [...projectKey(workspaceID), 'participation-detail', id],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID)}/project-participations/${String(id)}`,
        participationSchema,
        signal,
      ),
    retry: false,
  });
export const detailNameOptions = (workspaceID: string, projectID: number, keyword: string) =>
  queryOptions({
    queryKey: [...projectKey(workspaceID), 'detail-names', projectID, keyword],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID)}/project-participations/detail-names?project_id=${String(projectID)}&keyword=${encodeURIComponent(keyword)}`,
        z.array(z.object({ value: z.string(), label: z.string() })),
        signal,
      ),
    retry: false,
  });

const filterOptionSchema = z.object({ value: z.string(), label: z.string() });
export const projectFilterOptionsSchema = z.object({
  names: z.array(filterOptionSchema),
  years: z.array(filterOptionSchema),
  education_levels: z.array(filterOptionSchema),
  majors: z.array(filterOptionSchema),
  grades: z.array(filterOptionSchema),
});
export const projectFilterOptions = (workspaceID: string, participation: boolean) =>
  queryOptions({
    queryKey: [
      ...projectKey(workspaceID),
      'filter-options',
      participation ? 'participations' : 'projects',
    ],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID)}/${participation ? 'project-participations' : 'projects'}/filter-options`,
        projectFilterOptionsSchema,
        signal,
      ),
    retry: false,
  });
