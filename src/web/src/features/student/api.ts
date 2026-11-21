import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

import { studentFields } from '@/features/student/fields';
import { apiDelete, apiGet, apiPost, apiPut } from '@/lib/api';
import { uploadFile } from '@/lib/cos-upload';
import { filterParams } from '@/lib/filter-params';
import { saveSpreadsheetFile, spreadsheetFileSchema } from '@/lib/spreadsheet';

const optionalMonth = z.union([
  z.literal(''),
  z.string().regex(/^(?:19\d{2}|[2-9]\d{3})-(?:0[1-9]|1[0-2])$/u, {
    error: '请输入有效年月，格式为 YYYY-MM，年份为 1900 至 9999',
  }),
]);
const inputSchema = z.object({
  student_no: z.string().min(1, '学号不能为空').max(32),
  name: z.string().min(1, '姓名不能为空').max(50),
  grade: z.string().max(16),
  status: z.string().max(8),
  education_level: z.string().max(32),
  major: z.string().max(100),
  class_name: z.string().max(64),
  class_position: z.string().max(64),
  admission_type: z.string().max(32),
  arrival_month: optionalMonth.default(''),
  previous_school_or_employer: z.string().max(255).default(''),
  previous_graduation_month: optionalMonth.default(''),
  advisor: z.string().max(50),
  advisor_no: z.string().max(32).default(''),
  advisor_phone: z.string().max(32).default(''),
  gender: z.string().max(16),
  ethnicity: z.string().max(32),
  political_status: z.string().max(32),
  phone: z.string().max(32),
  qq: z.string().max(20),
  email: z.string().max(254),
  accommodation_type: z.string().max(16),
  campus: z.string().max(64),
  building: z.string().max(32),
  room: z.string().max(32),
  bed: z.string().max(16),
  origin: z.string().max(128),
  native_place: z.string().max(128),
  guardian1_name: z.string().max(50),
  guardian1_relationship: z.string().max(32),
  guardian1_phone: z.string().max(32),
  guardian2_name: z.string().max(50),
  guardian2_relationship: z.string().max(32),
  guardian2_phone: z.string().max(32),
});
// 历史记录仍可读取；新增和编辑提交时按固定目录校验必填字段。
const writeInputSchema = inputSchema.superRefine((values, context) => {
  for (const field of studentFields) {
    if (field.group !== 'basic' || !field.required) continue;
    const value = values[field.key as keyof typeof values].trim();
    if (!value || (field.options.length > 0 && value === 'unknown')) {
      context.addIssue({
        code: 'custom',
        path: [field.key],
        message: `${field.label}不能为空`,
      });
    }
  }
});

export class StudentValidationError extends Error {
  readonly field: string;

  constructor(message: string, field: string) {
    super(message);
    this.name = 'StudentValidationError';
    this.field = field;
  }
}

function parseStudentInput(values: StudentInput) {
  const parsed = writeInputSchema.safeParse(values);
  if (!parsed.success) {
    const issue = parsed.error.issues[0];
    throw new StudentValidationError(
      issue?.message ?? '学生信息无效',
      String(issue?.path[0] ?? ''),
    );
  }
  return parsed.data;
}

function validateSensitiveInput(values: SensitiveData['values'] | undefined) {
  if (!values?.find((item) => item.field === 'identity_number')?.value.trim()) {
    throw new StudentValidationError('身份证号码不能为空', 'identity_number');
  }
  return values;
}
const studentSchema = inputSchema.extend({
  id: z.number().int().positive(),
  birth_date: z.string().max(10),
  revision: z.number().int().positive(),
  created_at: z.iso.datetime({ offset: true }),
  updated_at: z.iso.datetime({ offset: true }),
});
const optionalDate = z.union([z.literal(''), z.iso.date()]);
const offCampusValuesSchema = z.object({
  reason: z.string().max(1000),
  start_date: optionalDate,
  end_date: optionalDate,
  category: z.string().max(64),
  rental_method: z.string().max(64),
  landlord_name: z.string().max(50),
  landlord_phone: z.string().max(32),
  roommate_name: z.string().max(50),
  roommate_unit: z.string().max(100),
  roommate_phone: z.string().max(32),
  address: z.string().max(255),
});
export type OffCampusValues = z.infer<typeof offCampusValuesSchema>;
export const emptyOffCampus: OffCampusValues = {
  reason: '',
  start_date: '',
  end_date: '',
  category: '',
  rental_method: '',
  landlord_name: '',
  landlord_phone: '',
  roommate_name: '',
  roommate_unit: '',
  roommate_phone: '',
  address: '',
};
const avatarUploadSchema = z.object({
  id: z.number().int().positive(),
  filename: z.string(),
  state: z.enum(['pending', 'deleting']),
  revision: z.number().int().positive(),
});
export type AvatarUpload = z.infer<typeof avatarUploadSchema>;
export const studentDetailSchema = studentSchema.extend({
  avatar_uploads: z.array(avatarUploadSchema).default([]),
  avatar_url: z.union([z.literal(''), z.url({ protocol: /^https$/ })]),
  off_campus_details: offCampusValuesSchema.optional(),
});
export type Student = z.infer<typeof studentSchema>;
export type StudentDetails = z.infer<typeof studentDetailSchema>;
export type StudentInput = z.infer<typeof inputSchema>;
export interface StudentFilters {
  keyword: string;
  [key: string]: string | string[];
}
export const emptyStudent: StudentInput = {
  student_no: '',
  name: '',
  grade: '',
  status: '',
  education_level: '',
  major: '',
  class_name: '',
  class_position: '',
  admission_type: '',
  arrival_month: '',
  previous_school_or_employer: '',
  previous_graduation_month: '',
  advisor: '',
  advisor_no: '',
  advisor_phone: '',
  gender: '',
  ethnicity: '',
  political_status: '',
  phone: '',
  qq: '',
  email: '',
  accommodation_type: '',
  campus: '',
  building: '',
  room: '',
  bed: '',
  origin: '',
  native_place: '',
  guardian1_name: '',
  guardian1_relationship: '',
  guardian1_phone: '',
  guardian2_name: '',
  guardian2_relationship: '',
  guardian2_phone: '',
};
const listSchema = z.object({
  items: z.array(studentSchema),
  total: z.number().int().nonnegative(),
  page: z.number().int().positive(),
  page_size: z.number().int().positive(),
});
const mutationSchema = z.object({
  id: z.number().int().positive(),
  revision: z.number().int().positive(),
});
const optionSchema = z.object({ value: z.string(), label: z.string() });
const sensitiveFields = studentFields.filter((field) => field.group === 'sensitive');
const sensitiveSchema = z.object({
  student_id: z.number().int().positive(),
  revision: z.number().int().positive(),
  values: z
    .array(z.object({ field: z.string(), value: z.string() }))
    .length(sensitiveFields.length)
    .refine(
      (values) =>
        new Set(values.map((item) => item.field)).size === sensitiveFields.length &&
        values.every((item) => sensitiveFields.some((field) => field.key === item.field)),
    ),
});
export type SensitiveData = z.infer<typeof sensitiveSchema>;
const statsSchema = z.object({
  total: z.number().int().nonnegative(),
  class_count: z.number().int().nonnegative(),
  religious_count: z.number().int().nonnegative(),
  off_campus_count: z.number().int().nonnegative(),
});
export type StudentStatistics = z.infer<typeof statsSchema>;
const fileSchema = spreadsheetFileSchema.extend({
  filename: z.union([
    z.string().regex(/^[a-z-]+\.xlsx$/),
    z.literal('XJ 明细表.xlsx'),
    z.literal('校外住宿明细表.xlsx'),
  ]),
});
const base = (workspaceID: string) => `/v1/workspaces/${workspaceID}/students`;
const params = (filters: StudentFilters) => filterParams(filters);
export const studentKey = (workspaceID: string) => ['workspace', workspaceID, 'students'] as const;
export const studentListOptions = (
  workspaceID: string,
  filters: StudentFilters,
  page: number,
  pageSize: number,
) =>
  queryOptions({
    queryKey: [...studentKey(workspaceID), 'list', filters, page, pageSize],
    queryFn: ({ signal }) => {
      const query = params(filters);
      query.set('page', String(page));
      query.set('page_size', String(pageSize));
      return apiGet(`${base(workspaceID)}?${query.toString()}`, listSchema, signal);
    },
    retry: false,
  });
export const studentDetailOptions = (workspaceID: string, id: number) =>
  queryOptions({
    queryKey: [...studentKey(workspaceID), 'detail', id],
    queryFn: ({ signal }) =>
      apiGet(`${base(workspaceID)}/${String(id)}`, studentDetailSchema, signal),
    retry: false,
  });
export const studentOptions = (workspaceID: string, field: string) =>
  queryOptions({
    queryKey: [...studentKey(workspaceID), 'options', field],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID)}/${studentFields.find((item) => item.key === field)?.group === 'sensitive' ? 'sensitive-options' : 'options'}?field=${encodeURIComponent(field)}`,
        z.array(optionSchema),
        signal,
      ),
    retry: false,
  });
export const studentStatisticsOptions = (workspaceID: string, filters: StudentFilters) =>
  queryOptions({
    queryKey: [...studentKey(workspaceID), 'statistics', filters],
    queryFn: ({ signal }) =>
      apiGet(`${base(workspaceID)}/statistics?${params(filters).toString()}`, statsSchema, signal),
    retry: false,
  });

export const studentSensitiveOptions = (workspaceID: string, id: number, revision?: number) =>
  queryOptions({
    queryKey: [...studentKey(workspaceID), 'sensitive', id, revision],
    queryFn: ({ signal }) =>
      apiGet(`${base(workspaceID)}/${String(id)}/sensitive`, sensitiveSchema, signal),
    staleTime: 0,
    gcTime: 0,
    retry: false,
  });
export function createStudent(
  workspaceID: string,
  values: StudentInput,
  sensitive?: SensitiveData['values'],
) {
  return apiPost(base(workspaceID), mutationSchema, {
    ...parseStudentInput(values),
    sensitive: validateSensitiveInput(sensitive),
  });
}
export function updateStudent(
  workspaceID: string,
  student: Pick<Student, 'id' | 'revision'>,
  values: StudentInput,
  sensitive?: SensitiveData['values'],
  offCampusDetails?: OffCampusValues,
) {
  return apiPut(`${base(workspaceID)}/${String(student.id)}`, mutationSchema, {
    ...parseStudentInput(values),
    revision: student.revision,
    ...(sensitive ? { sensitive: validateSensitiveInput(sensitive) } : {}),
    ...(offCampusDetails
      ? { off_campus_details: offCampusValuesSchema.parse(offCampusDetails) }
      : {}),
  });
}
export function uploadStudentAvatar(
  workspaceID: string,
  student: Pick<Student, 'id' | 'revision'>,
  file: File,
) {
  return uploadFile(
    `${base(workspaceID)}/${String(student.id)}/avatar`,
    mutationSchema,
    file,
    { revision: student.revision },
    true,
  );
}
export function deleteStudent(workspaceID: string, student: Student) {
  return apiDelete(
    `${base(workspaceID)}/${String(student.id)}?revision=${String(student.revision)}`,
    z.null(),
  );
}
export function updateStudentSensitive(
  workspaceID: string,
  id: number,
  revision: number,
  values: SensitiveData['values'],
) {
  return apiPut(`${base(workspaceID)}/${String(id)}/sensitive`, mutationSchema, {
    revision,
    values: validateSensitiveInput(values),
  });
}
export function studentTemplate(workspaceID: string, includeSensitive: boolean) {
  return apiGet(
    `${base(workspaceID)}/template?include_sensitive=${String(includeSensitive)}`,
    fileSchema,
  );
}
export function importStudents(workspaceID: string, content: string) {
  return apiPost(
    `${base(workspaceID)}/import`,
    z.object({ created: z.number().int().positive() }),
    { content },
  );
}
export function exportStudents(
  workspaceID: string,
  filters: StudentFilters,
  includeSensitive: boolean,
) {
  const query = params(filters);
  query.set('include_sensitive', String(includeSensitive));
  return apiGet(`${base(workspaceID)}/export?${query.toString()}`, fileSchema);
}
export function exportStudentReport(
  workspaceID: string,
  filters: StudentFilters,
  format: 'religion' | 'off_campus',
) {
  const query = params(filters);
  query.set('format', format);
  return apiGet(`${base(workspaceID)}/export?${query.toString()}`, fileSchema);
}
export function saveStudentFile(file: z.infer<typeof fileSchema>) {
  saveSpreadsheetFile(file);
}

export function deleteAvatarUpload(workspaceID: string, studentID: number, upload: AvatarUpload) {
  return apiDelete(
    `${base(workspaceID)}/${String(studentID)}/avatar-uploads/${String(upload.id)}?revision=${String(upload.revision)}`,
    z.null(),
  );
}
