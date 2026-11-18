import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

import {
  personFilterParams,
  type PersonFilters,
  personKey,
  personSchema,
} from '@/features/cpc-person/api';
import { valuesSchema } from '@/features/cpc-person/timeline-api';
import { nodeSchema } from '@/features/cpc-timeline/model';
import { apiGet } from '@/lib/api';

const viewSchema = z.object({
  template_id: z.number().int().positive(),
  template_name: z.string(),
  nodes: z.array(nodeSchema.extend({ numeric_id: z.number().int().positive() })),
});
const rowSchema = z.object({
  person: personSchema,
  values: valuesSchema,
  status: z.enum(['pending', 'completed', 'skipped']),
  status_label: z.enum(['待处理', '已完成', '已跳过']),
});
const listSchema = z.object({
  items: z.array(rowSchema),
  total: z.number().int().nonnegative(),
  page: z.number().int().positive(),
  page_size: z.number().int().positive(),
  can_create: z.boolean(),
  can_import: z.boolean(),
  can_export: z.boolean(),
});
const statisticsSchema = z
  .object({
    total: z.number().int().nonnegative(),
    completed: z.number().int().nonnegative(),
    pending: z.number().int().nonnegative(),
    skipped: z.number().int().nonnegative(),
  })
  .refine((data) => data.total === data.completed + data.pending + data.skipped);

export type TimelineView = z.infer<typeof viewSchema>;
export type TimelineViewPerson = z.infer<typeof rowSchema>;
export type TimelineViewStatistics = z.infer<typeof statisticsSchema>;

const base = (workspaceID: string) => `/v1/workspaces/${workspaceID}/cpc-people/timeline-view`;
const nodeBase = (workspaceID: string, templateID: number, nodeID: number) =>
  `${base(workspaceID)}/${String(templateID)}/nodes/${String(nodeID)}`;

export const timelineViewOptions = (workspaceID: string, templateID: number) =>
  queryOptions({
    queryKey: [...personKey(workspaceID), 'timeline-view', templateID],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID)}/${String(templateID)}`,
        viewSchema.refine((data) => data.template_id === templateID),
        signal,
      ),
    retry: false,
  });

export const timelineViewListOptions = (
  workspaceID: string,
  templateID: number,
  nodeID: number,
  filters: PersonFilters,
  page: number,
  pageSize: number,
) =>
  queryOptions({
    queryKey: [
      ...personKey(workspaceID),
      'timeline-view',
      templateID,
      nodeID,
      'list',
      filters,
      page,
      pageSize,
    ],
    queryFn: ({ signal }) => {
      const params = personFilterParams(filters, true);
      params.set('page', String(page));
      params.set('page_size', String(pageSize));
      return apiGet(
        `${nodeBase(workspaceID, templateID, nodeID)}?${params}`,
        listSchema.refine((data) =>
          data.items.every((row) => row.person.timeline_template_id === templateID),
        ),
        signal,
      );
    },
    retry: false,
  });

export const timelineViewStatisticsOptions = (
  workspaceID: string,
  templateID: number,
  nodeID: number,
  filters: PersonFilters,
) =>
  queryOptions({
    queryKey: [
      ...personKey(workspaceID),
      'timeline-view',
      templateID,
      nodeID,
      'statistics',
      filters,
    ],
    queryFn: ({ signal }) =>
      apiGet(
        `${nodeBase(workspaceID, templateID, nodeID)}/statistics?${personFilterParams(filters, true)}`,
        statisticsSchema,
        signal,
      ),
    retry: false,
  });
