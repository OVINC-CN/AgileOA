import { queryOptions } from '@tanstack/react-query';
import { z } from 'zod';

import {
  definitionSchema,
  parseTimelineInput,
  type TimelineInput,
  transferSchema,
} from '@/features/cpc-timeline/model';
import { apiDelete, apiGet, apiPost, apiPut } from '@/lib/api';
import { getConfig } from '@/lib/config';
import { uploadFile } from '@/lib/cos-upload';

const summarySchema = z.object({
  id: z.number().int().positive(),
  name: z.string(),
  description: z.string(),
  node_count: z.number().int().positive(),
  revision: z.number().int().positive(),
  created_at: z.iso.datetime({ offset: true }),
  updated_at: z.iso.datetime({ offset: true }),
});
const infoSchema = summarySchema.extend({
  definition: definitionSchema,
  transfers: z.array(transferSchema).optional(),
});
const transferTargetSchema = z.object({
  id: z.number().int().positive(),
  name: z.string(),
  definition: definitionSchema,
});
export type TransferTarget = z.infer<typeof transferTargetSchema>;
const listSchema = z.object({
  items: z.array(summarySchema),
  total: z.number().int().nonnegative(),
  page: z.number().int().positive(),
  page_size: z.number().int().positive(),
});
export type TimelineSummary = z.infer<typeof summarySchema>;
export type TimelineInfo = z.infer<typeof infoSchema>;
export const timelineKey = (workspaceID: string) =>
  ['workspace', workspaceID, 'cpc-timeline-templates'] as const;
const base = (workspaceID: string) => `/v1/workspaces/${workspaceID}/cpc-timeline-templates`;

export const timelineListOptions = (
  workspaceID: string,
  keyword: string,
  page: number,
  size: number,
) =>
  queryOptions({
    queryKey: [...timelineKey(workspaceID), 'list', keyword, page, size],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID)}?${new URLSearchParams({
          keyword,
          page: String(page),
          page_size: String(size),
        })}`,
        listSchema,
        signal,
      ),
    retry: false,
  });

export const timelineDetailOptions = (workspaceID: string, id: number) =>
  queryOptions({
    queryKey: [...timelineKey(workspaceID), 'detail', id],
    queryFn: ({ signal }) => apiGet(`${base(workspaceID)}/${String(id)}`, infoSchema, signal),
    retry: false,
  });

export const timelineTransferTargetsOptions = (
  workspaceID: string,
  purpose: 'create' | 'update',
  keyword = '',
  id?: number,
) =>
  queryOptions({
    queryKey: [...timelineKey(workspaceID), 'transfer-targets', purpose, keyword, id],
    queryFn: ({ signal }) =>
      apiGet(
        `${base(workspaceID)}/transfer-targets/${purpose}?${new URLSearchParams({
          keyword,
          ...(id ? { id: String(id) } : {}),
        })}`,
        z.array(transferTargetSchema),
        signal,
      ),
    retry: false,
  });

export function saveTimeline(
  workspaceID: string,
  original: TimelineInfo | null,
  values: TimelineInput,
  confirmNodeDeletion = false,
) {
  const input = parseTimelineInput(values);
  const body = {
    ...input,
    revision: original?.revision ?? 0,
    confirm_node_deletion: confirmNodeDeletion,
  };
  return original
    ? apiPut(`${base(workspaceID)}/${String(original.id)}`, infoSchema, body)
    : apiPost(base(workspaceID), infoSchema, body);
}

export function deleteTimeline(workspaceID: string, record: TimelineSummary) {
  return apiDelete(
    `${base(workspaceID)}/${String(record.id)}?revision=${String(record.revision)}`,
    z.null(),
  );
}

export function cloneTimeline(workspaceID: string, record: TimelineSummary) {
  return apiPost(`${base(workspaceID)}/${String(record.id)}/clone`, infoSchema, {
    revision: record.revision,
  });
}

export function importTimeline(workspaceID: string, value: unknown) {
  return apiPost(`${base(workspaceID)}/import`, infoSchema, parseTimelineInput(value));
}

export function timelineImportTemplate(workspaceID: string) {
  return apiGet(
    `${base(workspaceID)}/template`,
    z.object({
      filename: z.literal('timeline-template.json'),
      content_type: z.literal('application/json;charset=utf-8'),
      content: z.string(),
    }),
  );
}

export const attachmentSchema = z.object({
  id: z.number().int().positive(),
  timeline_template_id: z.number().int().positive(),
  node_id: z.string().regex(/^[a-z][a-z0-9_]{0,63}$/u),
  category: z.literal('cpc_timeline_node'),
  filename: z.string(),
  size: z.number().int().nonnegative(),
  state: z.enum(['pending', 'ready', 'deleting']),
  revision: z.number().int().positive(),
  created_at: z.iso.datetime({ offset: true }),
  updated_at: z.iso.datetime({ offset: true }),
});
export type TimelineAttachment = z.infer<typeof attachmentSchema>;
export const timelineAttachmentKey = (workspaceID: string, templateID: number, nodeID: string) =>
  [...timelineKey(workspaceID), 'attachments', templateID, nodeID] as const;
const attachmentBase = (workspaceID: string) =>
  `/v1/workspaces/${workspaceID}/attachments/cpc-timeline-nodes`;

const nodeAttachments = (workspaceID: string, templateID: number, nodeID: string) =>
  `${base(workspaceID)}/${String(templateID)}/nodes/${encodeURIComponent(nodeID)}/attachments`;

export const timelineAttachmentOptions = (
  workspaceID: string,
  templateID: number,
  nodeID: string,
  page: number,
  size: number,
) =>
  queryOptions({
    queryKey: [...timelineAttachmentKey(workspaceID, templateID, nodeID), page, size],
    queryFn: ({ signal }) =>
      apiGet(
        `${nodeAttachments(workspaceID, templateID, nodeID)}?${new URLSearchParams({
          page: String(page),
          page_size: String(size),
        })}`,
        z.object({
          items: z
            .array(attachmentSchema)
            .refine((items) =>
              items.every(
                (item) => item.timeline_template_id === templateID && item.node_id === nodeID,
              ),
            ),
          total: z.number().int().nonnegative(),
          page: z.number().int().positive(),
          page_size: z.number().int().positive(),
        }),
        signal,
      ),
    retry: false,
  });

export function uploadTimelineAttachment(
  workspaceID: string,
  templateID: number,
  nodeID: string,
  file: File,
) {
  return uploadFile(
    nodeAttachments(workspaceID, templateID, nodeID),
    attachmentSchema.refine(
      (item) => item.timeline_template_id === templateID && item.node_id === nodeID,
    ),
    file,
  );
}

export function deleteTimelineAttachment(workspaceID: string, file: TimelineAttachment) {
  return apiDelete(
    `${attachmentBase(workspaceID)}/${String(file.id)}?revision=${String(file.revision)}`,
    z.null(),
  );
}

export function timelineAttachmentDownload(workspaceID: string, id: number) {
  return `${getConfig().apiBaseURL.replace(/\/$/u, '')}${attachmentBase(workspaceID)}/${String(id)}/download`;
}
