import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

import {
  type AdmissionRuleNode,
  admissionRuleNodeSchema,
  cleanAdmissionRule,
} from '@/features/party-branch/admission-rule';
import { apiDelete, apiGet, apiPost, apiPut } from '@/lib/api';
import { spreadsheetFileSchema } from '@/lib/spreadsheet';

export const priorityLimits = { min: -2147483648, max: 2147483647 } as const;

const text = (limit: number, required = true) =>
  z
    .string()
    .trim()
    .min(required ? 1 : 0)
    .max(limit)
    .refine((value) => !/\p{Cc}/u.test(value));
const inputSchema = z.object({
  full_name: text(200),
  short_name: text(100),
  secretary_id: z.number().int().positive().nullable(),
  priority: z.number().int().min(priorityLimits.min).max(priorityLimits.max),
});
const branchSchema = inputSchema.extend({
  id: z.number().int().positive(),
  secretary_name: z.string(),
  secretary_username: z.string(),
  revision: z.number().int().positive(),
  created_at: z.iso.datetime({ offset: true }),
  updated_at: z.iso.datetime({ offset: true }),
});
const listSchema = z.object({
  items: z.array(branchSchema),
  total: z.number().int().nonnegative(),
  page: z.number().int().positive(),
  page_size: z.number().int().positive(),
});

export type PartyBranch = z.infer<typeof branchSchema>;
export type PartyBranchInput = z.infer<typeof inputSchema>;
export const partyBranchKey = (workspaceID: string) =>
  ['workspace', workspaceID, 'cpc-party-branches'] as const;
const base = (workspaceID: string) => `/v1/workspaces/${workspaceID}/cpc-party-branches`;

export const partyBranchListOptions = (
  workspaceID: string,
  keyword: string,
  page: number,
  pageSize: number,
) =>
  queryOptions({
    queryKey: [...partyBranchKey(workspaceID), 'list', keyword, page, pageSize],
    queryFn: ({ signal }) => {
      const params = new URLSearchParams({
        keyword,
        page: String(page),
        page_size: String(pageSize),
      });
      return apiGet(`${base(workspaceID)}?${params}`, listSchema, signal);
    },
    retry: false,
  });

export const partyBranchSecretaryOptions = (workspaceID: string) =>
  queryOptions({
    queryKey: [...partyBranchKey(workspaceID), 'secretaries'],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID)}/secretaries`,
        z.array(z.object({ value: z.number().int().positive(), label: z.string() })),
        signal,
      ),
    retry: false,
  });

export function savePartyBranch(
  workspaceID: string,
  original: PartyBranch | null,
  values: PartyBranchInput,
  admissionRule?: { rule: AdmissionRuleNode | null },
) {
  const parsed = inputSchema.safeParse(values);
  if (!parsed.success) throw new Error('请检查党支部全称、简称及管理员的格式和长度');
  const body = {
    ...parsed.data,
    revision: original?.revision ?? 0,
    ...(admissionRule ? { admission_rule: { rule: cleanAdmissionRule(admissionRule.rule) } } : {}),
  };
  return original
    ? apiPut(`${base(workspaceID)}/${String(original.id)}`, branchSchema, body)
    : apiPost(base(workspaceID), branchSchema, body);
}

export function deletePartyBranch(workspaceID: string, branch: PartyBranch) {
  return apiDelete(
    `${base(workspaceID)}/${String(branch.id)}?revision=${String(branch.revision)}`,
    z.null(),
  );
}

export function partyBranchTemplate(workspaceID: string) {
  return apiGet(
    `${base(workspaceID)}/template`,
    spreadsheetFileSchema.extend({
      filename: z.literal('party-branches.xlsx'),
    }),
  );
}

export function importPartyBranches(workspaceID: string, content: string) {
  return apiPost(
    `${base(workspaceID)}/import`,
    z.object({ created: z.number().int().positive() }),
    { content },
  );
}

export function exportPartyBranches(workspaceID: string, keyword: string) {
  const params = new URLSearchParams({ keyword });
  return apiGet(
    `${base(workspaceID)}/export?${params}`,
    spreadsheetFileSchema.extend({ filename: z.literal('party-branches.xlsx') }),
  );
}

export function reorderPartyBranchPeople(workspaceID: string) {
  return apiPost(`${base(workspaceID)}/reorder-people`, z.object({ queued: z.boolean() }));
}

const ruleOption = z.object({ value: z.string(), label: z.string() });
const admissionRuleSchema = z.object({
  id: z.number().int().positive(),
  rule: admissionRuleNodeSchema.nullable(),
  revision: z.number().int().positive(),
  updated_at: z.iso.datetime({ offset: true }),
  fields: z.array(
    ruleOption.extend({
      value: z.enum(['advisor', 'major', 'education_level', 'student_no']),
      max_length: z.number().int().positive(),
    }),
  ),
  logics: z.array(ruleOption.extend({ value: z.enum(['and', 'or']) })),
  operators: z.array(ruleOption.extend({ value: z.literal('in') })),
});
export type AdmissionRuleInfo = z.infer<typeof admissionRuleSchema>;

export const partyBranchAdmissionRuleOptions = (workspaceID: string, id: number) =>
  queryOptions({
    queryKey: [...partyBranchKey(workspaceID), 'admission-rule', id],
    queryFn: ({ signal }) =>
      apiGet(`${base(workspaceID)}/${String(id)}/admission-rule`, admissionRuleSchema, signal),
    retry: false,
  });
