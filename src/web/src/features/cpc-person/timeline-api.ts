import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

import { personKey } from '@/features/cpc-person/api';
import { attachmentSchema } from '@/features/cpc-timeline/api';
import { nodeSchema } from '@/features/cpc-timeline/model';
import { apiDelete, apiGet, apiPost, apiPut } from '@/lib/api';
import { getConfig } from '@/lib/config';
import { uploadFile } from '@/lib/cos-upload';

const personAttachmentSchema = attachmentSchema.omit({ timeline_template_id: true }).extend({
  person_id: z.number().int().positive(),
  category: z.literal('cpc_person_timeline_node'),
});
export const valuesSchema = z.record(
  z.string(),
  z.union([z.string(), z.number(), z.boolean(), z.null()]),
);
const issueSchema = z.object({
  rule_id: z.string(),
  reference_node_name: z.string(),
  actual_date: z.string(),
  min_date: z.string(),
  max_date: z.string(),
  message: z.string(),
});
const personNodeSchema = nodeSchema.extend({
  values: valuesSchema,
  status: z.enum(['pending', 'completed', 'skipped']),
  status_label: z.enum(['待处理', '已完成', '已跳过']),
  revision: z.number().int().positive(),
  issues: z.array(issueSchema),
  checks: z.array(
    issueSchema.extend({
      reference_date: z.string(),
      result: z.enum(['pending', 'normal', 'abnormal']),
      result_label: z.string(),
    }),
  ),
  template_attachments: z.array(attachmentSchema),
  attachments: z.array(personAttachmentSchema),
});
export const timelineSchema = z.object({
  person_id: z.number().int().positive(),
  person_revision: z.number().int().positive(),
  template_id: z.number().int().positive().nullable(),
  template_name: z.string(),
  template_revision: z.number().int().nonnegative(),
  can_update: z.boolean(),
  can_cancel_transfer: z.boolean().optional(),
  can_review_transfer: z.boolean().optional(),
  transfer_target_id: z.number().int().positive().nullable().optional(),
  transfer_target_name: z.string().optional(),
  has_transfer_targets: z.boolean().optional(),
  status: z.enum(['', 'normal', 'abnormal', 'transferring']),
  nodes: z.array(personNodeSchema),
});
export type PersonTimeline = z.infer<typeof timelineSchema>;
export type PersonTimelineNode = z.infer<typeof personNodeSchema>;
export type PersonTimelineAttachment = z.infer<typeof personAttachmentSchema>;
export type TimelineValues = z.infer<typeof valuesSchema>;
const base = (workspaceID: string) => `/v1/workspaces/${workspaceID}/cpc-people`;
const timelineBase = (workspaceID: string, personID: number) =>
  `${base(workspaceID)}/${String(personID)}/timeline`;
export const personTimelineKey = (workspaceID: string, personID: number) =>
  [...personKey(workspaceID), 'timeline', personID] as const;
export const personTimelineOptions = (
  workspaceID: string,
  personID: number,
  editing: boolean,
  reviewing = false,
) =>
  queryOptions({
    queryKey: [
      ...personTimelineKey(workspaceID, personID),
      reviewing ? 'review' : editing ? 'edit' : 'detail',
    ],
    queryFn: ({ signal }) =>
      apiGet(
        `${timelineBase(workspaceID, personID)}${reviewing ? '/review' : editing ? '/edit' : ''}`,
        timelineSchema.refine((value) => value.person_id === personID),
        signal,
      ),
    retry: false,
  });
export const personTimelineTemplateOptions = (
  workspaceID: string,
  purpose: 'create' | 'update' | 'import' | 'list',
  keyword: string,
) =>
  queryOptions({
    queryKey: [...personKey(workspaceID), 'timeline-templates', purpose, keyword],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID)}/timeline-templates/${purpose}?${new URLSearchParams({ keyword })}`,
        z.array(z.object({ value: z.string().regex(/^[1-9]\d*$/u), label: z.string() })),
        signal,
      ),
    retry: false,
  });
export const bindPersonTimeline = (
  workspaceID: string,
  personID: number,
  templateID: number,
  revision: number,
) =>
  apiPost(`${timelineBase(workspaceID, personID)}/bind`, timelineSchema, {
    template_id: templateID,
    revision,
  });
export const savePersonTimelineNode = (
  workspaceID: string,
  personID: number,
  nodeID: string,
  values: TimelineValues,
  revision: number,
  templateRevision: number,
) =>
  apiPut(
    `${timelineBase(workspaceID, personID)}/nodes/${encodeURIComponent(nodeID)}`,
    timelineSchema,
    {
      values,
      revision,
      template_revision: templateRevision,
    },
  );
export const resetPersonTimelineNode = (
  workspaceID: string,
  personID: number,
  nodeID: string,
  revision: number,
  templateRevision: number,
) =>
  apiPost(
    `${timelineBase(workspaceID, personID)}/nodes/${encodeURIComponent(nodeID)}/reset`,
    timelineSchema,
    {
      revision,
      template_revision: templateRevision,
    },
  );
export const skipPersonTimelineNode = (
  workspaceID: string,
  personID: number,
  nodeID: string,
  revision: number,
  templateRevision: number,
) =>
  apiPost(
    `${timelineBase(workspaceID, personID)}/nodes/${encodeURIComponent(nodeID)}/skip`,
    timelineSchema,
    {
      revision,
      template_revision: templateRevision,
    },
  );
export function uploadPersonTimelineAttachment(
  workspaceID: string,
  personID: number,
  nodeID: string,
  file: File,
) {
  return uploadFile(
    `${timelineBase(workspaceID, personID)}/nodes/${encodeURIComponent(nodeID)}/attachments`,
    personAttachmentSchema.refine(
      (value) => value.person_id === personID && value.node_id === nodeID,
    ),
    file,
  );
}
export const deletePersonTimelineAttachment = (
  workspaceID: string,
  file: PersonTimelineAttachment,
) =>
  apiDelete(
    `/v1/workspaces/${workspaceID}/attachments/cpc-person-timeline-nodes/${String(file.id)}?revision=${String(file.revision)}`,
    z.null(),
  );
export function personTimelineDownload(
  workspaceID: string,
  personID: number,
  nodeID: string,
  fileID: number,
  template: boolean,
  editing: boolean,
) {
  const path = `${timelineBase(workspaceID, personID)}/nodes/${encodeURIComponent(nodeID)}/${template ? 'template-attachments' : 'attachments'}/${String(fileID)}/${editing ? 'edit/download' : 'download'}`;
  return `${getConfig().apiBaseURL.replace(/\/$/u, '')}${path}`;
}
