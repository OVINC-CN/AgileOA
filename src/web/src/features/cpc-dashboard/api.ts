import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

import { apiGet } from '@/lib/api';
import { spreadsheetFileSchema } from '@/lib/spreadsheet';

const countsSchema = z.object({
  full_members: z.number().int().nonnegative(),
  probationary_members: z.number().int().nonnegative(),
  total: z.number().int().nonnegative(),
});
const branchSummarySchema = countsSchema.extend({
  party_branch_id: z.number().int().positive(),
  party_branch_name: z.string(),
  secretaries: z.string(),
  organization_members: z.string(),
  publicity_members: z.string(),
  discipline_members: z.string(),
  advisors: z.string(),
});
const dashboardSchema = z.object({
  items: z.array(branchSummarySchema),
  totals: countsSchema,
});

export type CPCDashboard = z.infer<typeof dashboardSchema>;
export const dashboardKey = (workspaceID: string) =>
  ['workspace', workspaceID, 'cpc-dashboard'] as const;
const base = (workspaceID: string) => `/v1/workspaces/${workspaceID}/cpc-dashboard`;

export const dashboardOptions = (workspaceID: string) =>
  queryOptions({
    queryKey: dashboardKey(workspaceID),
    queryFn: ({ signal }) => apiGet(base(workspaceID), dashboardSchema, signal),
    retry: false,
  });

export function exportDashboard(workspaceID: string) {
  return apiGet(`${base(workspaceID)}/export`, spreadsheetFileSchema);
}
