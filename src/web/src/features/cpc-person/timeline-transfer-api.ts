import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

import { personTimelineKey, timelineSchema } from '@/features/cpc-person/timeline-api';
import { apiGet, apiPost } from '@/lib/api';

const optionsSchema = z.object({
  source_template_id: z.number().int().positive(),
  source_template_name: z.string(),
  targets: z.array(z.object({ value: z.string().regex(/^[1-9]\d*$/u), label: z.string() })),
});
const previewSchema = z.object({
  source_template_id: z.number().int().positive(),
  source_template_name: z.string(),
  preview_version: z.string().regex(/^[0-9a-f]{64}$/u),
  timeline: timelineSchema,
  discarded_fields: z.array(
    z.object({
      node_name: z.string(),
      field_name: z.string(),
      value: z.union([z.string(), z.number(), z.boolean()]),
    }),
  ),
});
export type TimelineTransferPreview = z.infer<typeof previewSchema>;
export type TimelineTransferOptions = z.infer<typeof optionsSchema>;
const base = (workspaceID: string, personID: number) =>
  `/v1/workspaces/${workspaceID}/cpc-people/${String(personID)}/timeline`;

export const timelineTransferOptions = (workspaceID: string, personID: number) =>
  queryOptions({
    queryKey: [...personTimelineKey(workspaceID, personID), 'transfer-options'],
    queryFn: ({ signal }) =>
      apiGet(`${base(workspaceID, personID)}/transfer-options`, optionsSchema, signal),
    retry: false,
    staleTime: 0,
  });

export const previewTimelineTransfer = (workspaceID: string, personID: number, targetID: number) =>
  apiPost(
    `${base(workspaceID, personID)}/transfer-preview`,
    previewSchema.refine(
      (value) => value.timeline.person_id === personID && value.timeline.template_id === targetID,
    ),
    { target_template_id: targetID },
  );

export const requestTimelineTransfer = (
  workspaceID: string,
  personID: number,
  preview: TimelineTransferPreview,
) =>
  apiPost(
    `${base(workspaceID, personID)}/transfer`,
    timelineSchema.refine(
      (value) =>
        value.person_id === personID &&
        value.status === 'transferring' &&
        value.transfer_target_id === preview.timeline.template_id,
    ),
    { target_template_id: preview.timeline.template_id, preview_version: preview.preview_version },
  );

export const timelineTransferReviewOptions = (workspaceID: string, personID: number) =>
  queryOptions({
    queryKey: [...personTimelineKey(workspaceID, personID), 'transfer-review'],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID, personID)}/transfer-review`,
        previewSchema.refine((value) => value.timeline.person_id === personID),
        signal,
      ),
    retry: false,
    staleTime: 0,
  });

export const reviewTimelineTransfer = (
  workspaceID: string,
  personID: number,
  preview: TimelineTransferPreview,
) =>
  apiPost(
    `${base(workspaceID, personID)}/transfer-review`,
    timelineSchema.refine(
      (value) =>
        value.person_id === personID &&
        value.template_id === preview.timeline.template_id &&
        value.status !== 'transferring',
    ),
    { target_template_id: preview.timeline.template_id, preview_version: preview.preview_version },
  );

export const cancelTimelineTransfer = (workspaceID: string, personID: number, revision: number) =>
  apiPost(
    `${base(workspaceID, personID)}/transfer-cancel`,
    timelineSchema.refine(
      (value) => value.person_id === personID && value.status !== 'transferring',
    ),
    { revision },
  );

export const rejectTimelineTransfer = (
  workspaceID: string,
  personID: number,
  revision: number,
  reason: string,
) =>
  apiPost(
    `${base(workspaceID, personID)}/transfer-reject`,
    timelineSchema.refine(
      (value) => value.person_id === personID && value.status !== 'transferring',
    ),
    { revision, reason },
  );
