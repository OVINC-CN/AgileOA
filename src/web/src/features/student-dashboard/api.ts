import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

import { apiGet } from '@/lib/api';
import { filterParams } from '@/lib/filter-params';
import { spreadsheetFileSchema } from '@/lib/spreadsheet';

const countsSchema = z.object({
  major_counts: z.record(z.string(), z.number().int().nonnegative()),
  total: z.number().int().nonnegative(),
});
const classSummarySchema = countsSchema.extend({
  class_name: z.string(),
  monitors: z.string(),
});
const dashboardSchema = z.object({
  majors: z.array(z.string()),
  items: z.array(classSummarySchema),
  totals: countsSchema,
});

export type StudentDashboard = z.infer<typeof dashboardSchema>;
export type StudentDashboardRow = z.infer<typeof classSummarySchema>;
export const dashboardKey = (workspaceID: string) =>
  ['workspace', workspaceID, 'student-dashboard'] as const;
const base = (workspaceID: string) => `/v1/workspaces/${workspaceID}/student-dashboard`;

export const dashboardOptions = (workspaceID: string) =>
  queryOptions({
    queryKey: dashboardKey(workspaceID),
    queryFn: ({ signal }) => apiGet(base(workspaceID), dashboardSchema, signal),
    retry: false,
  });

export function exportDashboard(workspaceID: string) {
  return apiGet(`${base(workspaceID)}/export`, spreadsheetFileSchema);
}

const optionSchema = z.object({ value: z.string(), label: z.string() });
const accommodationTypeSchema = optionSchema.extend({
  value: z.enum(['on_campus', 'off_campus', 'joint_abroad', 'unknown']),
});
const accommodationRowSchema = z.object({
  accommodation_type: accommodationTypeSchema,
  campus: z.string(),
  building: z.string(),
  room: z.string(),
  count: z.number().int().nonnegative(),
});
const accommodationSchema = z.object({
  items: z.array(accommodationRowSchema),
  total: z.number().int().nonnegative(),
});
const accommodationOptionsSchema = z.object({
  accommodation_types: z.array(accommodationTypeSchema),
  campuses: z.array(optionSchema),
  buildings: z.array(optionSchema),
  rooms: z.array(optionSchema),
});

export type AccommodationData = z.infer<typeof accommodationSchema>;
export type AccommodationRow = z.infer<typeof accommodationRowSchema>;
export type AccommodationFilterOptions = z.infer<typeof accommodationOptionsSchema>;
export interface AccommodationFilters {
  accommodation_type: string[];
  campus: string[];
  building: string[];
  room: string[];
}

export function defaultAccommodationFilters(): AccommodationFilters {
  return { accommodation_type: [], campus: [], building: [], room: [] };
}

export const accommodationSummaryOptions = (workspaceID: string, filters: AccommodationFilters) =>
  queryOptions({
    queryKey: [...dashboardKey(workspaceID), 'accommodation', filters],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID)}/accommodation?${filterParams({ ...filters })}`,
        accommodationSchema,
        signal,
      ),
    retry: false,
  });

export const accommodationFilterOptions = (workspaceID: string) =>
  queryOptions({
    queryKey: [...dashboardKey(workspaceID), 'accommodation', 'filter-options'],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID)}/accommodation/filter-options`,
        accommodationOptionsSchema,
        signal,
      ),
    retry: false,
  });

export function exportAccommodation(workspaceID: string, filters: AccommodationFilters) {
  return apiGet(
    `${base(workspaceID)}/accommodation/export?${filterParams({ ...filters })}`,
    spreadsheetFileSchema.extend({ filename: z.literal('住宿信息表.xlsx') }),
  );
}
